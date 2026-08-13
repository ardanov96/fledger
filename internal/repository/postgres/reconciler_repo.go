// Package postgres — ReconcilerRepository implements reconciler.Repository.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"

	"github.com/runut/fmcg-wallet/internal/domain/reconciler"
)

// =============================================================================
// DTOs
// =============================================================================

type ReconcilerRunDTO struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	PeriodID        uuid.UUID
	StartedAt       time.Time
	FinishedAt      *time.Time
	Status          string
	TotalDebit      int64
	TotalCredit     int64
	Imbalance       int64
	HashChainOK     *bool
	HashChainErrors int
	TriggeredBy     string
	Metadata        []byte
}

type ReconcilerAccountResultDTO struct {
	ID            uuid.UUID
	RunID         uuid.UUID
	PeriodID      uuid.UUID
	AccountID     uuid.UUID
	DebitMinor    int64
	CreditMinor   int64
	SignedBalance int64
	EntryCount    int
	Currency      string
}

// =============================================================================
// Repository
// =============================================================================

type ReconcilerRepository struct {
	db *DB
}

func NewReconcilerRepository(db *DB) *ReconcilerRepository {
	return &ReconcilerRepository{db: db}
}

// assertReconcilerTx type-asserts a reconciler.Tx.
func (r *ReconcilerRepository) assertReconcilerTx(tx reconciler.Tx) (*reconcilerTxAdapter, error) {
	a, ok := tx.(*reconcilerTxAdapter)
	if !ok {
		return nil, fmt.Errorf("expected *reconcilerTxAdapter, got %T", tx)
	}
	return a, nil
}

// ----- Run operations -----

func (r *ReconcilerRepository) CreateRun(ctx context.Context, tx reconciler.Tx, run reconciler.ReconcilerRun) error {
	a, err := r.assertReconcilerTx(tx)
	if err != nil {
		return err
	}

	const q = `
INSERT INTO reconciler_runs (
    id, tenant_id, period_id, started_at, status,
    total_debit, total_credit, imbalance,
    triggered_by, metadata
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, $10
)
`
	_, err = a.pgxTx.Exec(ctx, q,
		run.ID, run.TenantID, run.PeriodID, run.StartedAt, string(run.Status),
		run.TotalDebit.Minor(), run.TotalCredit.Minor(), run.Imbalance.Minor(),
		string(run.TriggeredBy), jsonRaw(run.Metadata),
	)
	if err != nil {
		return fmt.Errorf("create reconciler run: %w", err)
	}
	return nil
}

func (r *ReconcilerRepository) FinishRun(ctx context.Context, tx reconciler.Tx, id string, status reconciler.RunStatus, totalDebit, totalCredit, imbalance int64, hashChainOK *bool, hashChainErrors int, finishedAt time.Time) error {
	a, err := r.assertReconcilerTx(tx)
	if err != nil {
		return err
	}

	const q = `
UPDATE reconciler_runs
SET status = $2,
    total_debit = $3,
    total_credit = $4,
    imbalance = $5,
    hash_chain_ok = $6,
    hash_chain_errors = $7,
    finished_at = $8
WHERE id = $1
`
	_, err = a.pgxTx.Exec(ctx, q,
		id, string(status), totalDebit, totalCredit, imbalance,
		hashChainOK, hashChainErrors, finishedAt,
	)
	if err != nil {
		return fmt.Errorf("finish reconciler run: %w", err)
	}
	return nil
}

func (r *ReconcilerRepository) InsertAccountResult(ctx context.Context, tx reconciler.Tx, result reconciler.ReconcilerAccountResult) error {
	a, err := r.assertReconcilerTx(tx)
	if err != nil {
		return err
	}

	const q = `
INSERT INTO reconciler_account_results (
    id, run_id, period_id, account_id,
    debit_minor, credit_minor, signed_balance,
    entry_count, currency
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7,
    $8, $9
)
`
	_, err = a.pgxTx.Exec(ctx, q,
		result.ID, result.RunID, result.PeriodID, result.AccountID,
		result.DebitMinor, result.CreditMinor, result.SignedBalance,
		result.EntryCount, result.Currency,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.ErrAlreadyExists
		}
		return fmt.Errorf("insert account result: %w", err)
	}
	return nil
}

// ----- Read operations -----

func (r *ReconcilerRepository) GetRun(ctx context.Context, id string) (reconciler.ReconcilerRun, error) {
	const q = `
SELECT id, tenant_id, period_id, started_at, finished_at, status,
       total_debit, total_credit, imbalance,
       hash_chain_ok, hash_chain_errors, triggered_by, metadata
FROM reconciler_runs
WHERE id = $1
`
	dto, err := scanReconcilerRun(r.db.Pool.QueryRow(ctx, q, id))
	if err != nil {
		return reconciler.ReconcilerRun{}, err
	}
	return dtoToReconcilerRun(dto), nil
}

func (r *ReconcilerRepository) ListRunsByPeriod(ctx context.Context, periodID string, limit int) ([]reconciler.ReconcilerRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
SELECT id, tenant_id, period_id, started_at, finished_at, status,
       total_debit, total_credit, imbalance,
       hash_chain_ok, hash_chain_errors, triggered_by, metadata
FROM reconciler_runs
WHERE period_id = $1
ORDER BY started_at DESC
LIMIT $2
`
	rows, err := r.db.Pool.Query(ctx, q, periodID, limit)
	if err != nil {
		return nil, fmt.Errorf("list runs by period: %w", err)
	}
	defer rows.Close()

	out := make([]reconciler.ReconcilerRun, 0, limit)
	for rows.Next() {
		var dto ReconcilerRunDTO
		if err := rows.Scan(
			&dto.ID, &dto.TenantID, &dto.PeriodID, &dto.StartedAt, &dto.FinishedAt, &dto.Status,
			&dto.TotalDebit, &dto.TotalCredit, &dto.Imbalance,
			&dto.HashChainOK, &dto.HashChainErrors, &dto.TriggeredBy, &dto.Metadata,
		); err != nil {
			return nil, fmt.Errorf("scan reconciler run: %w", err)
		}
		out = append(out, dtoToReconcilerRun(dto))
	}
	return out, rows.Err()
}

func (r *ReconcilerRepository) ListRunsByTenant(ctx context.Context, tenantID string, limit int) ([]reconciler.ReconcilerRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
SELECT id, tenant_id, period_id, started_at, finished_at, status,
       total_debit, total_credit, imbalance,
       hash_chain_ok, hash_chain_errors, triggered_by, metadata
FROM reconciler_runs
WHERE tenant_id = $1
ORDER BY started_at DESC
LIMIT $2
`
	rows, err := r.db.Pool.Query(ctx, q, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list runs by tenant: %w", err)
	}
	defer rows.Close()

	out := make([]reconciler.ReconcilerRun, 0, limit)
	for rows.Next() {
		var dto ReconcilerRunDTO
		if err := rows.Scan(
			&dto.ID, &dto.TenantID, &dto.PeriodID, &dto.StartedAt, &dto.FinishedAt, &dto.Status,
			&dto.TotalDebit, &dto.TotalCredit, &dto.Imbalance,
			&dto.HashChainOK, &dto.HashChainErrors, &dto.TriggeredBy, &dto.Metadata,
		); err != nil {
			return nil, fmt.Errorf("scan reconciler run: %w", err)
		}
		out = append(out, dtoToReconcilerRun(dto))
	}
	return out, rows.Err()
}

func (r *ReconcilerRepository) ListAccountResultsByRun(ctx context.Context, runID string) ([]reconciler.ReconcilerAccountResult, error) {
	const q = `
SELECT id, run_id, period_id, account_id,
       debit_minor, credit_minor, signed_balance,
       entry_count, currency
FROM reconciler_account_results
WHERE run_id = $1
ORDER BY account_id ASC
`
	rows, err := r.db.Pool.Query(ctx, q, runID)
	if err != nil {
		return nil, fmt.Errorf("list account results: %w", err)
	}
	defer rows.Close()

	out := make([]reconciler.ReconcilerAccountResult, 0, 16)
	for rows.Next() {
		var dto ReconcilerAccountResultDTO
		if err := rows.Scan(
			&dto.ID, &dto.RunID, &dto.PeriodID, &dto.AccountID,
			&dto.DebitMinor, &dto.CreditMinor, &dto.SignedBalance,
			&dto.EntryCount, &dto.Currency,
		); err != nil {
			return nil, fmt.Errorf("scan account result: %w", err)
		}
		out = append(out, reconciler.ReconcilerAccountResult{
			ID:            dto.ID.String(),
			RunID:         dto.RunID.String(),
			PeriodID:      dto.PeriodID.String(),
			AccountID:     dto.AccountID.String(),
			DebitMinor:    dto.DebitMinor,
			CreditMinor:   dto.CreditMinor,
			SignedBalance: dto.SignedBalance,
			EntryCount:    dto.EntryCount,
			Currency:      dto.Currency,
		})
	}
	return out, rows.Err()
}

func (r *ReconcilerRepository) ListOpenPeriods(ctx context.Context, tenantID string) ([]reconciler.PeriodRef, error) {
	const q = `
SELECT id, tenant_id
FROM accounting_periods
WHERE tenant_id = $1 AND status IN ('open', 'closing')
ORDER BY period_start ASC
`
	rows, err := r.db.Pool.Query(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list open periods: %w", err)
	}
	defer rows.Close()

	out := make([]reconciler.PeriodRef, 0, 8)
	for rows.Next() {
		var p reconciler.PeriodRef
		if err := rows.Scan(&p.ID, &p.TenantID); err != nil {
			return nil, fmt.Errorf("scan period ref: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *ReconcilerRepository) ListTenants(ctx context.Context) ([]string, error) {
	const q = `SELECT DISTINCT tenant_id FROM accounting_periods ORDER BY tenant_id`
	rows, err := r.db.Pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0, 4)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// =============================================================================
// DTO helpers
// =============================================================================

func scanReconcilerRun(row pgx.Row) (ReconcilerRunDTO, error) {
	var dto ReconcilerRunDTO
	err := row.Scan(
		&dto.ID, &dto.TenantID, &dto.PeriodID, &dto.StartedAt, &dto.FinishedAt, &dto.Status,
		&dto.TotalDebit, &dto.TotalCredit, &dto.Imbalance,
		&dto.HashChainOK, &dto.HashChainErrors, &dto.TriggeredBy, &dto.Metadata,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReconcilerRunDTO{}, apperrors.ErrNotFound
		}
		return ReconcilerRunDTO{}, fmt.Errorf("scan reconciler run: %w", err)
	}
	return dto, nil
}

func dtoToReconcilerRun(dto ReconcilerRunDTO) reconciler.ReconcilerRun {
	return reconciler.ReconcilerRun{
		ID:              dto.ID.String(),
		TenantID:        dto.TenantID.String(),
		PeriodID:        dto.PeriodID.String(),
		StartedAt:       dto.StartedAt,
		FinishedAt:      dto.FinishedAt,
		Status:          reconciler.RunStatus(dto.Status),
		TotalDebit:      amountFromMinor(dto.TotalDebit),
		TotalCredit:     amountFromMinor(dto.TotalCredit),
		Imbalance:       amountFromMinor(dto.Imbalance),
		HashChainOK:     dto.HashChainOK,
		HashChainErrors: dto.HashChainErrors,
		TriggeredBy:     reconciler.TriggerSource(dto.TriggeredBy),
		Metadata:        parseMetadata(dto.Metadata),
	}
}
