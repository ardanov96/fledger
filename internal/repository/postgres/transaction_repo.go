package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
)

// TransactionRepository implements ledger.TransactionRepository.
type TransactionRepository struct {
	db *DB
}

// NewTransactionRepository creates a new transaction repository.
func NewTransactionRepository(db *DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create inserts a new transaction (status: pending).
// MUST be called inside a transaction.
func (r *TransactionRepository) Create(ctx context.Context, tx ledger.Tx, transaction ledger.Transaction) error {
	pgxTx, ok := tx.(*txAdapter)
	if !ok {
		return fmt.Errorf("expected pgx.Tx, got %T", tx)
	}

	const q = `
INSERT INTO transactions (
    id, idempotency_key, status, description, ref_type, ref_id,
    initiator_id, tenant_id, period_id, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
`
	_, err := pgxTx.Exec(ctx, q,
		transaction.ID, transaction.IdempotencyKey, string(transaction.Status),
		transaction.Description, transaction.RefType, transaction.RefID,
		transaction.InitiatorID, transaction.TenantID, transaction.PeriodID,
		jsonRaw(transaction.Metadata),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("create transaction: %w", apperrors.ErrIdempotencyConflict)
		}
		return fmt.Errorf("create transaction: %w", err)
	}
	return nil
}

// GetByID returns a transaction by ID.
//
// Sprint 37 fix: was bare Pool.QueryRow which would return 0 rows after
// RLS filter. Now uses RunInReadTx so the tenant GUC is bound.
func (r *TransactionRepository) GetByID(ctx context.Context, id string, withEntries bool) (ledger.Transaction, error) {
	const q = `
SELECT id, idempotency_key, status, description, ref_type, ref_id,
       initiator_id, tenant_id, period_id, metadata, posted_at,
       created_at, updated_at
FROM transactions
WHERE id = $1
`
	var dto TransactionDTO
	err := r.db.RunInReadTx(ctx, func(tx pgx.Tx) error {
		var scanErr error
		dto, scanErr = scanTransaction(tx.QueryRow(ctx, q, id))
		return scanErr
	})
	if err != nil {
		return ledger.Transaction{}, err
	}
	txn := dtoToTransaction(dto)
	if withEntries {
		txn.Entries = []ledger.Entry{}
	}
	return txn, nil
}

// GetByIdempotencyKey returns the transaction matching the key.
//
// Sprint 37 fix: same as GetByID.
func (r *TransactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (ledger.Transaction, error) {
	const q = `
SELECT id, idempotency_key, status, description, ref_type, ref_id,
       initiator_id, tenant_id, period_id, metadata, posted_at,
       created_at, updated_at
FROM transactions
WHERE idempotency_key = $1
LIMIT 1
`
	var dto TransactionDTO
	err := r.db.RunInReadTx(ctx, func(tx pgx.Tx) error {
		var scanErr error
		dto, scanErr = scanTransaction(tx.QueryRow(ctx, q, key))
		return scanErr
	})
	if err != nil {
		if err.Error() == apperrors.ErrNotFound.Error() {
			return ledger.Transaction{}, apperrors.ErrIdempotencyConflict
		}
		return ledger.Transaction{}, err
	}
	return dtoToTransaction(dto), nil
}

// MarkPosted transitions status from pending to posted.
//
// Sprint 37 fix: now uses RunInTx so RLS WITH CHECK passes.
func (r *TransactionRepository) MarkPosted(ctx context.Context, id string) error {
	const q = `UPDATE transactions SET status = 'posted', posted_at = now() WHERE id = $1 AND status = 'pending'`
	var rowsAffected int64
	err := r.db.RunInTx(ctx, func(pgxTx pgx.Tx) error {
		tag, err := pgxTx.Exec(ctx, q, id)
		if err != nil {
			return fmt.Errorf("mark posted: %w", err)
		}
		rowsAffected = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// MarkFailed transitions status from pending to failed.
//
// Sprint 37 fix: now uses RunInTx so RLS WITH CHECK passes.
func (r *TransactionRepository) MarkFailed(ctx context.Context, id string) error {
	const q = `UPDATE transactions SET status = 'failed' WHERE id = $1 AND status = 'pending'`
	return r.db.RunInTx(ctx, func(pgxTx pgx.Tx) error {
		_, err := pgxTx.Exec(ctx, q, id)
		if err != nil {
			return fmt.Errorf("mark failed: %w", err)
		}
		return nil
	})
}

// MarkReversed transitions status to reversed.
//
// Sprint 37 fix: now uses RunInTx so RLS WITH CHECK passes.
func (r *TransactionRepository) MarkReversed(ctx context.Context, id string) error {
	const q = `UPDATE transactions SET status = 'reversed' WHERE id = $1 AND status = 'posted'`
	return r.db.RunInTx(ctx, func(pgxTx pgx.Tx) error {
		_, err := pgxTx.Exec(ctx, q, id)
		if err != nil {
			return fmt.Errorf("mark reversed: %w", err)
		}
		return nil
	})
}

// dtoToTransaction converts a DB DTO to the domain entity.
func dtoToTransaction(dto TransactionDTO) ledger.Transaction {
	return ledger.Transaction{
		ID:             dto.ID.String(),
		IdempotencyKey: dto.IdempotencyKey,
		Status:         ledger.TransactionStatus(dto.Status),
		Description:    strPtrToString(dto.Description),
		RefType:        strPtrToString(dto.RefType),
		RefID:          uuidPtrToString(dto.RefID),
		InitiatorID:    uuidPtrToString(dto.InitiatorID),
		TenantID:       dto.TenantID.String(),
		PeriodID:       dto.PeriodID.String(),
		Metadata:       parseMetadata(dto.Metadata),
		PostedAt:       dto.PostedAt,
		CreatedAt:      dto.CreatedAt,
		UpdatedAt:      dto.UpdatedAt,
		Entries:        nil,
	}
}

func strPtrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
