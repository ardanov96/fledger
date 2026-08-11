package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// EntryRepository implements ledger.EntryRepository.
type EntryRepository struct {
	db *DB
}

// NewEntryRepository creates a new entry repository.
func NewEntryRepository(db *DB) *EntryRepository {
	return &EntryRepository{db: db}
}

// Insert persists entries. MUST be called inside a transaction.
func (r *EntryRepository) Insert(ctx context.Context, tx ledger.Tx, entries []ledger.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	const q = `
INSERT INTO ledger_entries (
    id, transaction_id, account_id, amount, type, ref_type, ref_id,
    period_id, description, currency, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
`

	for _, e := range entries {
		refID := e.RefID
		if _, err := tx.Exec(ctx, q,
			e.ID, e.TransactionID, e.AccountID, e.Amount.Minor(), string(e.Type),
			e.RefType, nullableString(refID), e.PeriodID, e.Description, e.Currency,
			jsonRaw(e.Metadata),
		); err != nil {
			return fmt.Errorf("insert entry: %w", err)
		}
	}
	return nil
}

// ListByTransaction returns all entries for a given transaction.
func (r *EntryRepository) ListByTransaction(ctx context.Context, transactionID string) ([]ledger.Entry, error) {
	const q = `
SELECT id, transaction_id, account_id, amount, type, ref_type, ref_id,
       period_id, description, currency, metadata, created_at
FROM ledger_entries
WHERE transaction_id = $1
ORDER BY created_at, id
`
	rows, err := r.db.Pool.Query(ctx, q, transactionID)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	entries := make([]ledger.Entry, 0, 4)
	for rows.Next() {
		var dto EntryDTO
		if err := rows.Scan(
			&dto.ID, &dto.TransactionID, &dto.AccountID, &dto.Amount, &dto.Type,
			&dto.RefType, &dto.RefID, &dto.PeriodID, &dto.Description,
			&dto.Currency, &dto.Metadata, &dto.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, dtoToEntry(dto))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return entries, nil
}

// ListByAccount returns paginated entries for an account.
func (r *EntryRepository) ListByAccount(ctx context.Context, accountID string, filter ledger.EntryFilter) ([]ledger.Entry, error) {
	const q = `
SELECT id, transaction_id, account_id, amount, type, ref_type, ref_id,
       period_id, description, currency, metadata, created_at
FROM ledger_entries
WHERE account_id = $1
  AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC, id DESC
LIMIT $3
`
	var before *time.Time
	if filter.From != 0 {
		t := nanoToTime(filter.From)
		before = &t
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	rows, err := r.db.Pool.Query(ctx, q, accountID, before, limit)
	if err != nil {
		return nil, fmt.Errorf("list entries by account: %w", err)
	}
	defer rows.Close()

	entries := make([]ledger.Entry, 0, limit)
	for rows.Next() {
		var dto EntryDTO
		if err := rows.Scan(
			&dto.ID, &dto.TransactionID, &dto.AccountID, &dto.Amount, &dto.Type,
			&dto.RefType, &dto.RefID, &dto.PeriodID, &dto.Description,
			&dto.Currency, &dto.Metadata, &dto.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, dtoToEntry(dto))
	}
	return entries, rows.Err()
}

// SumForAccount returns the authoritative balance for an account.
func (r *EntryRepository) SumForAccount(ctx context.Context, accountID string) (money.Money, error) {
	const q = `
SELECT COALESCE(SUM(
    CASE WHEN type = 'debit' THEN amount ELSE -amount END
), 0)::BIGINT
FROM ledger_entries
WHERE account_id = $1
`
	var balance int64
	if err := r.db.Pool.QueryRow(ctx, q, accountID).Scan(&balance); err != nil {
		return money.Money(0), fmt.Errorf("sum entries: %w", err)
	}
	return money.NewFromMinor(balance), nil
}

// TrialBalance returns global SUM(debit) - SUM(credit).
func (r *EntryRepository) TrialBalance(ctx context.Context) (TrialBalanceDTO, error) {
	const q = `
SELECT
    COALESCE(SUM(CASE WHEN type = 'debit'  THEN amount ELSE 0 END), 0)::BIGINT,
    COALESCE(SUM(CASE WHEN type = 'credit' THEN amount ELSE 0 END), 0)::BIGINT
FROM ledger_entries
`
	var tb TrialBalanceDTO
	if err := r.db.Pool.QueryRow(ctx, q).Scan(&tb.TotalDebit, &tb.TotalCredit); err != nil {
		return tb, fmt.Errorf("trial balance: %w", err)
	}
	tb.Imbalance = tb.TotalDebit - tb.TotalCredit
	return tb, nil
}

// dtoToEntry converts DB DTO to domain entity.
func dtoToEntry(dto EntryDTO) ledger.Entry {
	return ledger.Entry{
		ID:            dto.ID.String(),
		TransactionID: dto.TransactionID.String(),
		AccountID:     dto.AccountID.String(),
		Amount:        money.NewFromMinor(dto.Amount),
		Type:          ledger.EntryType(dto.Type),
		RefType:       strPtrToString(dto.RefType),
		RefID:         uuidPtrToString(dto.RefID),
		PeriodID:      dto.PeriodID.String(),
		Description:   strPtrToString(dto.Description),
		Currency:      dto.Currency,
		Metadata:      parseMetadata(dto.Metadata),
		CreatedAt:     dto.CreatedAt,
	}
}

// nullableString converts "" to nil so the column can store NULL.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
