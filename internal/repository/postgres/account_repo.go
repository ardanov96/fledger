package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// AccountRepository implements ledger.AccountRepository using PostgreSQL.
type AccountRepository struct {
	db *DB
}

// NewAccountRepository creates a new account repository.
func NewAccountRepository(db *DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// Create inserts a new account.
func (r *AccountRepository) Create(ctx context.Context, account ledger.Account) error {
	const q = `
INSERT INTO accounts (
    id, code, name, type, status, currency, cached_balance,
    owner_id, tenant_id, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
`
	_, err := r.db.Pool.Exec(ctx, q,
		account.ID, account.Code, account.Name, string(account.Type),
		string(account.Status), account.Currency, account.CachedBalance.Minor(),
		account.OwnerID, account.TenantID, jsonRaw(account.Metadata),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("create account: %w", apperrors.ErrAlreadyExists)
		}
		return fmt.Errorf("create account: %w", err)
	}
	return nil
}

// GetByID fetches a single account.
func (r *AccountRepository) GetByID(ctx context.Context, id string) (ledger.Account, error) {
	const q = `
SELECT id, code, name, type, status, currency, cached_balance,
       owner_id, tenant_id, metadata, created_at, updated_at
FROM accounts
WHERE id = $1
`
	dto, err := scanAccount(r.db.Pool.QueryRow(ctx, q, id))
	if err != nil {
		return ledger.Account{}, err
	}
	return dtoToAccount(dto), nil
}

// GetByCode fetches by tenant + code.
func (r *AccountRepository) GetByCode(ctx context.Context, code string) (ledger.Account, error) {
	const q = `
SELECT id, code, name, type, status, currency, cached_balance,
       owner_id, tenant_id, metadata, created_at, updated_at
FROM accounts
WHERE code = $1
`
	dto, err := scanAccount(r.db.Pool.QueryRow(ctx, q, code))
	if err != nil {
		return ledger.Account{}, err
	}
	return dtoToAccount(dto), nil
}

// List returns accounts matching the filter, cursor-paginated.
func (r *AccountRepository) List(ctx context.Context, filter ledger.AccountFilter) ([]ledger.Account, error) {
	const q = `
SELECT id, code, name, type, status, currency, cached_balance,
       owner_id, tenant_id, metadata, created_at, updated_at
FROM accounts
WHERE tenant_id = $1
  AND ($2::text IS NULL OR type = $2)
  AND ($3::text IS NULL OR status = $3)
  AND ($4::timestamptz IS NULL OR created_at < $4)
ORDER BY created_at DESC, id DESC
LIMIT $5
`
	var accountType, status *string
	var before *time.Time
	if filter.Type != "" {
		s := string(filter.Type)
		accountType = &s
	}
	if filter.Status != "" {
		s := string(filter.Status)
		status = &s
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	tenantID := uuid.Nil
	if filter.TenantID != "" {
		id, err := uuid.Parse(filter.TenantID)
		if err != nil {
			return nil, fmt.Errorf("parse tenant id: %w", err)
		}
		tenantID = id
	}

	rows, err := r.db.Pool.Query(ctx, q, tenantID, accountType, status, before, limit)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]ledger.Account, 0, limit)
	for rows.Next() {
		var dto AccountDTO
		if err := rows.Scan(
			&dto.ID, &dto.Code, &dto.Name, &dto.Type, &dto.Status, &dto.Currency,
			&dto.CachedBalance, &dto.OwnerID, &dto.TenantID, &dto.Metadata,
			&dto.CreatedAt, &dto.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, dtoToAccount(dto))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return accounts, nil
}

// Update modifies an account.
func (r *AccountRepository) Update(ctx context.Context, account ledger.Account) error {
	const q = `
UPDATE accounts
SET code = $2, name = $3, type = $4, status = $5, currency = $6,
    cached_balance = $7, owner_id = $8, metadata = $9
WHERE id = $1
`
	tag, err := r.db.Pool.Exec(ctx, q,
		account.ID, account.Code, account.Name, string(account.Type),
		string(account.Status), account.Currency, account.CachedBalance.Minor(),
		account.OwnerID, jsonRaw(account.Metadata),
	)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrAccountNotFound
	}
	return nil
}

// LockForUpdate acquires a row-level lock on the account.
// MUST be called inside a transaction. Caller is responsible for
// ordering locks by account ID to prevent deadlocks.
func (r *AccountRepository) LockForUpdate(ctx context.Context, tx ledger.Tx, id string) (ledger.Account, error) {
	pgxTx, ok := tx.(*txAdapter)
	if !ok {
		return ledger.Account{}, fmt.Errorf("expected pgx.Tx, got %T", tx)
	}

	const q = `
SELECT id, code, name, type, status, currency, cached_balance,
       owner_id, tenant_id, metadata, created_at, updated_at
FROM accounts
WHERE id = $1
FOR UPDATE
`
	dto, err := scanAccount(pgxTx.QueryRow(ctx, q, id))
	if err != nil {
		return ledger.Account{}, err
	}
	return dtoToAccount(dto), nil
}

// UpdateBalance atomically updates the cached balance within a transaction.
// The row MUST be locked via LockForUpdate first to prevent lost updates.
func (r *AccountRepository) UpdateBalance(ctx context.Context, tx ledger.Tx, id string, newBalance money.Money) error {
	pgxTx, ok := tx.(*txAdapter)
	if !ok {
		return fmt.Errorf("expected pgx.Tx, got %T", tx)
	}

	const q = `UPDATE accounts SET cached_balance = $2 WHERE id = $1`
	tag, err := pgxTx.Exec(ctx, q, id, newBalance.Minor())
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrAccountNotFound
	}
	return nil
}

// dtoToAccount converts a DB row DTO to the domain entity.
func dtoToAccount(dto AccountDTO) ledger.Account {
	return ledger.Account{
		ID:            dto.ID.String(),
		Code:          dto.Code,
		Name:          dto.Name,
		Type:          ledger.AccountType(dto.Type),
		Status:        ledger.AccountStatus(dto.Status),
		Currency:      dto.Currency,
		CachedBalance: money.NewFromMinor(dto.CachedBalance),
		OwnerID:       uuidPtrToString(dto.OwnerID),
		TenantID:      dto.TenantID.String(),
		Metadata:      parseMetadata(dto.Metadata),
		CreatedAt:     dto.CreatedAt,
		UpdatedAt:     dto.UpdatedAt,
	}
}

// uuidPtrToString safely dereferences a *uuid.UUID.
func uuidPtrToString(p *uuid.UUID) string {
	if p == nil {
		return ""
	}
	return p.String()
}
