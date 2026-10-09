package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-pay/internal/platform/errors"
	"github.com/fledger/fledger-pay/internal/domain/transaction"
)

type TransactionRepo struct {
	pool *pgxpool.Pool
}

func NewTransactionRepo(p *pgxpool.Pool) *TransactionRepo { return &TransactionRepo{pool: p} }

const txColumns = `id, tenant_id, payment_request_id, channel, external_reference,
	idempotency_key, gross_amount, net_amount, fee_amount, paid_at,
	COALESCE(payer_name,''), COALESCE(payer_bank,''), raw_payload, signature_verified, created_at`

func scanTx(row pgx.Row) (transaction.Transaction, error) {
	var t transaction.Transaction
	var channel string
	var raw []byte
	if err := row.Scan(
		&t.ID, &t.TenantID, &t.PaymentRequestID, &channel, &t.ExternalReference,
		&t.IdempotencyKey, &t.GrossAmount, &t.NetAmount, &t.FeeAmount, &t.PaidAt,
		&t.PayerName, &t.PayerBank, &raw, &t.SignatureVerified, &t.CreatedAt,
	); err != nil {
		return transaction.Transaction{}, err
	}
	t.Channel = transaction.Channel(channel)
	return t, nil
}

// Insert persists one bank transaction row. Returns ErrConflict on duplicate
// external_reference or idempotency_key (idempotency guard for webhooks).
func (r *TransactionRepo) Insert(ctx context.Context, t transaction.Transaction) (transaction.Transaction, error) {
	if len(t.RawPayload) == 0 {
		t.RawPayload = map[string]any{}
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO pay_transactions
		  (tenant_id, payment_request_id, channel, external_reference, idempotency_key,
		   gross_amount, net_amount, fee_amount, paid_at, payer_name, payer_bank,
		   raw_payload, signature_verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10,''), NULLIF($11,''), $12, $13)
		RETURNING `+txColumns,
		t.TenantID, t.PaymentRequestID, string(t.Channel), t.ExternalReference, t.IdempotencyKey,
		t.GrossAmount, t.NetAmount, t.FeeAmount, t.PaidAt, t.PayerName, t.PayerBank,
		t.RawPayload, t.SignatureVerified,
	)
	out, err := scanTx(row)
	if err != nil {
		if isUniqueViolation(err) {
			return transaction.Transaction{}, fmt.Errorf("%w: duplicate external_reference or idempotency_key", apperrors.ErrConflict)
		}
		return transaction.Transaction{}, fmt.Errorf("insert tx: %w", err)
	}
	return out, nil
}

// GetByExternalRef returns the existing row for a duplicate-webhook scenario.
func (r *TransactionRepo) GetByExternalRef(ctx context.Context, tenantID, ref string) (transaction.Transaction, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+txColumns+` FROM pay_transactions WHERE tenant_id = $1 AND external_reference = $2`, tenantID, ref)
	t, err := scanTx(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return transaction.Transaction{}, fmt.Errorf("%w: external_ref %s", apperrors.ErrNotFound, ref)
		}
		return transaction.Transaction{}, err
	}
	return t, nil
}

// ListByRequest returns the transactions for one payment request.
func (r *TransactionRepo) ListByRequest(ctx context.Context, requestID string) ([]transaction.Transaction, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+txColumns+` FROM pay_transactions WHERE payment_request_id = $1 ORDER BY paid_at DESC`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]transaction.Transaction, 0, 4)
	for rows.Next() {
		t, err := scanTx(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// List returns recent transactions for the tenant.
func (r *TransactionRepo) List(ctx context.Context, tenantID string, limit int) ([]transaction.Transaction, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `SELECT `+txColumns+` FROM pay_transactions WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]transaction.Transaction, 0, limit)
	for rows.Next() {
		t, err := scanTx(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get fetches a single transaction by id.
func (r *TransactionRepo) Get(ctx context.Context, tenantID, id string) (transaction.Transaction, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+txColumns+` FROM pay_transactions WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	t, err := scanTx(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return transaction.Transaction{}, fmt.Errorf("%w: transaction %s", apperrors.ErrNotFound, id)
		}
		return transaction.Transaction{}, err
	}
	return t, nil
}