// Package postgres — PeriodRepository implements period.Repository using PostgreSQL.
//
// All write methods accept a period.Tx (which is a thin wrapper over pgx.Tx)
// so the caller can compose period writes with other writes in one tx —
// notably the use case will lock the period, insert the close request, update
// the period status, and (on approve) insert N snapshot rows all in ONE tx.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"

	"github.com/runut/fmcg-wallet/internal/domain/period"
)

// =============================================================================
// DTOs
// =============================================================================

type PeriodDTO struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	PeriodStart time.Time
	PeriodEnd   time.Time
	Status      string
}

type CloseRequestDTO struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	PeriodID        uuid.UUID
	RequesterID     uuid.UUID
	ApproverID      *uuid.UUID
	Status          string
	TrialBalanceOK  bool
	TotalDebit      int64
	TotalCredit     int64
	Imbalance       int64
	RejectionReason *string
	RequestedAt     time.Time
	DecidedAt       *time.Time
	Metadata        []byte
}

type PeriodSnapshotDTO struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	PeriodID     uuid.UUID
	RequestID    uuid.UUID
	AccountID    uuid.UUID
	BalanceMinor int64
	Currency     string
	EntryCount   int
	SnapshotAt   time.Time
	Metadata     []byte
}

// =============================================================================
// Repository
// =============================================================================

type PeriodRepository struct {
	db *DB
}

func NewPeriodRepository(db *DB) *PeriodRepository {
	return &PeriodRepository{db: db}
}

// assertPeriodTx type-asserts a period.Tx into *periodTxAdapter.
func (r *PeriodRepository) assertPeriodTx(tx period.Tx) (*periodTxAdapter, error) {
	a, ok := tx.(*periodTxAdapter)
	if !ok {
		return nil, fmt.Errorf("expected *periodTxAdapter, got %T", tx)
	}
	return a, nil
}

// ----- Period operations -----

func (r *PeriodRepository) LockPeriod(ctx context.Context, tx period.Tx, periodID string) (period.Period, error) {
	a, err := r.assertPeriodTx(tx)
	if err != nil {
		return period.Period{}, err
	}

	const q = `
SELECT id, tenant_id, period_start, period_end, status
FROM accounting_periods
WHERE id = $1
FOR UPDATE
`
	var dto PeriodDTO
	err = a.pgxTx.QueryRow(ctx, q, periodID).Scan(
		&dto.ID, &dto.TenantID, &dto.PeriodStart, &dto.PeriodEnd, &dto.Status,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return period.Period{}, apperrors.ErrNotFound
		}
		return period.Period{}, fmt.Errorf("lock period: %w", err)
	}
	return dtoToPeriod(dto), nil
}

func (r *PeriodRepository) UpdatePeriodStatus(ctx context.Context, tx period.Tx, periodID string, status period.PeriodStatus) error {
	a, err := r.assertPeriodTx(tx)
	if err != nil {
		return err
	}

	const q = `
UPDATE accounting_periods
SET status = $2, updated_at = now()
WHERE id = $1
`
	tag, err := a.pgxTx.Exec(ctx, q, periodID, string(status))
	if err != nil {
		return fmt.Errorf("update period status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ----- Close request operations -----

func (r *PeriodRepository) InsertCloseRequest(ctx context.Context, tx period.Tx, req period.CloseRequest) error {
	a, err := r.assertPeriodTx(tx)
	if err != nil {
		return err
	}

	const q = `
INSERT INTO period_close_requests (
    id, tenant_id, period_id, requester_id, status,
    trial_balance_ok, total_debit, total_credit, imbalance,
    requested_at, metadata
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11
)
`
	_, err = a.pgxTx.Exec(ctx, q,
		req.ID, req.TenantID, req.PeriodID, req.RequesterID, string(req.Status),
		req.TrialBalanceOK, req.TotalDebit.Minor(), req.TotalCredit.Minor(), req.Imbalance.Minor(),
		req.RequestedAt, jsonRaw(req.Metadata),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.ErrAlreadyExists
		}
		return fmt.Errorf("insert close request: %w", err)
	}
	return nil
}

func (r *PeriodRepository) GetCloseRequest(ctx context.Context, id string) (period.CloseRequest, error) {
	const q = `
SELECT id, tenant_id, period_id, requester_id, approver_id, status,
       trial_balance_ok, total_debit, total_credit, imbalance,
       rejection_reason, requested_at, decided_at, metadata
FROM period_close_requests
WHERE id = $1
`
	dto, err := scanCloseRequest(r.db.Pool.QueryRow(ctx, q, id))
	if err != nil {
		return period.CloseRequest{}, err
	}
	return dtoToCloseRequest(dto), nil
}

func (r *PeriodRepository) LockCloseRequest(ctx context.Context, tx period.Tx, id string) (period.CloseRequest, error) {
	a, err := r.assertPeriodTx(tx)
	if err != nil {
		return period.CloseRequest{}, err
	}

	const q = `
SELECT id, tenant_id, period_id, requester_id, approver_id, status,
       trial_balance_ok, total_debit, total_credit, imbalance,
       rejection_reason, requested_at, decided_at, metadata
FROM period_close_requests
WHERE id = $1
FOR UPDATE
`
	dto, err := scanCloseRequest(a.pgxTx.QueryRow(ctx, q, id))
	if err != nil {
		return period.CloseRequest{}, err
	}
	return dtoToCloseRequest(dto), nil
}

func (r *PeriodRepository) DecideCloseRequest(
	ctx context.Context, tx period.Tx, id string,
	status period.CloseRequestStatus, approverID string,
	totalDebit, totalCredit, imbalance int64,
	rejectionReason string, decidedAt time.Time,
) error {
	a, err := r.assertPeriodTx(tx)
	if err != nil {
		return err
	}

	const q = `
UPDATE period_close_requests
SET status = $2,
    approver_id = $3,
    trial_balance_ok = $4,
    total_debit = $5,
    total_credit = $6,
    imbalance = $7,
    rejection_reason = $8,
    decided_at = $9
WHERE id = $1
`
	tag, err := a.pgxTx.Exec(ctx, q,
		id, string(status), approverID,
		status == period.CloseRequestApproved, // trial_balance_ok = true kalau approved
		totalDebit, totalCredit, imbalance,
		nullStr(rejectionReason), decidedAt,
	)
	if err != nil {
		return fmt.Errorf("decide close request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ----- Snapshot operations -----

func (r *PeriodRepository) InsertSnapshot(ctx context.Context, tx period.Tx, snap period.PeriodSnapshot) error {
	a, err := r.assertPeriodTx(tx)
	if err != nil {
		return err
	}

	const q = `
INSERT INTO period_snapshots (
    id, tenant_id, period_id, request_id, account_id,
    balance_minor, currency, entry_count, snapshot_at, metadata
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10
)
`
	_, err = a.pgxTx.Exec(ctx, q,
		snap.ID, snap.TenantID, snap.PeriodID, snap.RequestID, snap.AccountID,
		snap.BalanceMinor, snap.Currency, snap.EntryCount, snap.SnapshotAt, jsonRaw(snap.Metadata),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.ErrAlreadyExists
		}
		return fmt.Errorf("insert snapshot: %w", err)
	}
	return nil
}

func (r *PeriodRepository) ListSnapshotsByPeriod(ctx context.Context, periodID string) ([]period.PeriodSnapshot, error) {
	const q = `
SELECT id, tenant_id, period_id, request_id, account_id,
       balance_minor, currency, entry_count, snapshot_at, metadata
FROM period_snapshots
WHERE period_id = $1
ORDER BY account_id ASC
`
	rows, err := r.db.Pool.Query(ctx, q, periodID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()

	out := make([]period.PeriodSnapshot, 0, 16)
	for rows.Next() {
		var dto PeriodSnapshotDTO
		if err := rows.Scan(
			&dto.ID, &dto.TenantID, &dto.PeriodID, &dto.RequestID, &dto.AccountID,
			&dto.BalanceMinor, &dto.Currency, &dto.EntryCount, &dto.SnapshotAt, &dto.Metadata,
		); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		out = append(out, dtoToSnapshot(dto))
	}
	return out, rows.Err()
}

func (r *PeriodRepository) ListRequestsByPeriod(ctx context.Context, periodID string) ([]period.CloseRequest, error) {
	const q = `
SELECT id, tenant_id, period_id, requester_id, approver_id, status,
       trial_balance_ok, total_debit, total_credit, imbalance,
       rejection_reason, requested_at, decided_at, metadata
FROM period_close_requests
WHERE period_id = $1
ORDER BY requested_at ASC
`
	rows, err := r.db.Pool.Query(ctx, q, periodID)
	if err != nil {
		return nil, fmt.Errorf("list close requests: %w", err)
	}
	defer rows.Close()

	out := make([]period.CloseRequest, 0, 8)
	for rows.Next() {
		var dto CloseRequestDTO
		if err := rows.Scan(
			&dto.ID, &dto.TenantID, &dto.PeriodID, &dto.RequesterID, &dto.ApproverID, &dto.Status,
			&dto.TrialBalanceOK, &dto.TotalDebit, &dto.TotalCredit, &dto.Imbalance,
			&dto.RejectionReason, &dto.RequestedAt, &dto.DecidedAt, &dto.Metadata,
		); err != nil {
			return nil, fmt.Errorf("scan close request: %w", err)
		}
		out = append(out, dtoToCloseRequest(dto))
	}
	return out, rows.Err()
}

// ----- Trial balance + entries count -----

func (r *PeriodRepository) ComputeTrialBalance(ctx context.Context, tx period.Tx, periodID string) (totalDebit, totalCredit, imbalance int64, err error) {
	a, err := r.assertPeriodTx(tx)
	if err != nil {
		return 0, 0, 0, err
	}

	const q = `
SELECT
    COALESCE(SUM(CASE WHEN type = 'debit'  THEN amount ELSE 0 END), 0) AS total_debit,
    COALESCE(SUM(CASE WHEN type = 'credit' THEN amount ELSE 0 END), 0) AS total_credit
FROM ledger_entries
WHERE period_id = $1
`
	var td, tc int64
	if err = a.pgxTx.QueryRow(ctx, q, periodID).Scan(&td, &tc); err != nil {
		return 0, 0, 0, fmt.Errorf("compute trial balance: %w", err)
	}
	return td, tc, td - tc, nil
}

func (r *PeriodRepository) CountEntriesForPeriod(ctx context.Context, tx period.Tx, periodID string) (int, error) {
	a, err := r.assertPeriodTx(tx)
	if err != nil {
		return 0, err
	}

	const q = `SELECT COUNT(*) FROM ledger_entries WHERE period_id = $1`
	var n int
	if err := a.pgxTx.QueryRow(ctx, q, periodID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count entries: %w", err)
	}
	return n, nil
}

func (r *PeriodRepository) ListAccountsByTenant(ctx context.Context, tenantID string) ([]period.AccountRef, error) {
	const q = `SELECT id, currency FROM accounts WHERE tenant_id = $1 ORDER BY id ASC`
	rows, err := r.db.Pool.Query(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	out := make([]period.AccountRef, 0, 16)
	for rows.Next() {
		var ref period.AccountRef
		if err := rows.Scan(&ref.ID, &ref.Currency); err != nil {
			return nil, fmt.Errorf("scan account ref: %w", err)
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

func (r *PeriodRepository) ComputeAccountBalanceAtPeriod(ctx context.Context, tx period.Tx, accountID, periodID string) (balanceMinor int64, entryCount int, err error) {
	a, err := r.assertPeriodTx(tx)
	if err != nil {
		return 0, 0, err
	}

	const q = `
SELECT
    COALESCE(SUM(CASE WHEN type = 'debit'  THEN amount ELSE -amount END), 0) AS signed_balance,
    COUNT(*)
FROM ledger_entries
WHERE account_id = $1 AND period_id = $2
`
	var bal int64
	var cnt int
	if err = a.pgxTx.QueryRow(ctx, q, accountID, periodID).Scan(&bal, &cnt); err != nil {
		return 0, 0, fmt.Errorf("compute account balance at period: %w", err)
	}
	return bal, cnt, nil
}

// =============================================================================
// DTO helpers
// =============================================================================

func scanCloseRequest(row pgx.Row) (CloseRequestDTO, error) {
	var dto CloseRequestDTO
	err := row.Scan(
		&dto.ID, &dto.TenantID, &dto.PeriodID, &dto.RequesterID, &dto.ApproverID,
		&dto.Status, &dto.TrialBalanceOK, &dto.TotalDebit, &dto.TotalCredit, &dto.Imbalance,
		&dto.RejectionReason, &dto.RequestedAt, &dto.DecidedAt, &dto.Metadata,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CloseRequestDTO{}, apperrors.ErrNotFound
		}
		return CloseRequestDTO{}, fmt.Errorf("scan close request: %w", err)
	}
	return dto, nil
}

func dtoToPeriod(dto PeriodDTO) period.Period {
	return period.Period{
		ID:          dto.ID.String(),
		TenantID:    dto.TenantID.String(),
		PeriodStart: dto.PeriodStart,
		PeriodEnd:   dto.PeriodEnd,
		Status:      period.PeriodStatus(dto.Status),
	}
}

func dtoToCloseRequest(dto CloseRequestDTO) period.CloseRequest {
	cr := period.CloseRequest{
		ID:              dto.ID.String(),
		TenantID:        dto.TenantID.String(),
		PeriodID:        dto.PeriodID.String(),
		RequesterID:     dto.RequesterID.String(),
		Status:          period.CloseRequestStatus(dto.Status),
		TrialBalanceOK:  dto.TrialBalanceOK,
		TotalDebit:      amountFromMinor(dto.TotalDebit),
		TotalCredit:     amountFromMinor(dto.TotalCredit),
		Imbalance:       amountFromMinor(dto.Imbalance),
		RejectionReason: strPtrToString(dto.RejectionReason),
		RequestedAt:     dto.RequestedAt,
		DecidedAt:       dto.DecidedAt,
		Metadata:        parseMetadata(dto.Metadata),
	}
	if dto.ApproverID != nil {
		cr.ApproverID = dto.ApproverID.String()
	}
	return cr
}

func dtoToSnapshot(dto PeriodSnapshotDTO) period.PeriodSnapshot {
	return period.PeriodSnapshot{
		ID:           dto.ID.String(),
		TenantID:     dto.TenantID.String(),
		PeriodID:     dto.PeriodID.String(),
		RequestID:    dto.RequestID.String(),
		AccountID:    dto.AccountID.String(),
		BalanceMinor: dto.BalanceMinor,
		Currency:     dto.Currency,
		EntryCount:   dto.EntryCount,
		SnapshotAt:   dto.SnapshotAt,
		Metadata:     parseMetadata(dto.Metadata),
	}
}
