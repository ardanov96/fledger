// Package postgres — payment repository. All multi-row writes use
// transactions so the PaymentRequest, VAs, and QRIS rows land atomically.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-pay/internal/platform/errors"
	"github.com/fledger/fledger-pay/internal/domain/payment"
	"github.com/fledger/fledger-pay/internal/domain/qris"
	"github.com/fledger/fledger-pay/internal/domain/va"
)

type PaymentRepo struct {
	pool *pgxpool.Pool
}

func NewPaymentRepo(p *pgxpool.Pool) *PaymentRepo { return &PaymentRepo{pool: p} }

const prColumns = `id, tenant_id, fledger_invoice_id, request_number, customer_id,
	customer_name, COALESCE(customer_phone,''), currency, amount, fee_amount, total_amount,
	status, expires_at, settled_at, settled_amount, metadata, created_at, updated_at`

func scanPR(row pgx.Row) (payment.Request, error) {
	var r payment.Request
	var status string
	var meta []byte
	var phone string
	if err := row.Scan(
		&r.ID, &r.TenantID, &r.FledgerInvoiceID, &r.RequestNumber, &r.CustomerID,
		&r.CustomerName, &phone, &r.Currency, &r.Amount, &r.FeeAmount, &r.TotalAmount,
		&status, &r.ExpiresAt, &r.SettledAt, &r.SettledAmount, &meta,
		&r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return payment.Request{}, err
	}
	r.Status = payment.Status(status)
	r.CustomerPhone = phone
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &r.Metadata)
	}
	return r, nil
}

// CreateBundle inserts the payment request + N virtual accounts + (optional)
// QRIS row inside one transaction.
func (r *PaymentRepo) CreateBundle(
	ctx context.Context,
	req payment.Request,
	vas []va.Account,
	qr *qris.Code,
) (payment.Request, []va.Account, *qris.Code, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return payment.Request{}, nil, nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if len(req.Metadata) == 0 {
		req.Metadata = map[string]any{}
	}
	metaJSON, _ := json.Marshal(req.Metadata)
	row := tx.QueryRow(ctx, `
		INSERT INTO pay_payment_requests
		  (tenant_id, fledger_invoice_id, request_number, customer_id, customer_name,
		   customer_phone, currency, amount, fee_amount, total_amount, status, expires_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'PENDING', $11, $12)
		RETURNING `+prColumns,
		req.TenantID, req.FledgerInvoiceID, req.RequestNumber, req.CustomerID, req.CustomerName,
		req.CustomerPhone, req.Currency, req.Amount, req.FeeAmount, req.TotalAmount,
		req.ExpiresAt, metaJSON,
	)
	created, err := scanPR(row)
	if err != nil {
		if isUniqueViolation(err) {
			return payment.Request{}, nil, nil, fmt.Errorf("%w: request_number already exists", apperrors.ErrConflict)
		}
		return payment.Request{}, nil, nil, fmt.Errorf("insert PR: %w", err)
	}

	outVAs := make([]va.Account, 0, len(vas))
	for _, v := range vas {
		v.PaymentRequestID = created.ID
		row := tx.QueryRow(ctx, `
			INSERT INTO pay_virtual_accounts
			  (tenant_id, payment_request_id, bank_code, va_number, va_name, expected_amount, status, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'ACTIVE', $7)
			RETURNING id, tenant_id, payment_request_id, bank_code, va_number, va_name,
			          expected_amount, status, expires_at, paid_at, created_at`,
			created.TenantID, created.ID, string(v.BankCode), v.VANumber, v.VAName,
			v.ExpectedAmount, v.ExpiresAt,
		)
		var got va.Account
		if err := row.Scan(
			&got.ID, &got.TenantID, &got.PaymentRequestID, &got.BankCode, &got.VANumber,
			&got.VAName, &got.ExpectedAmount, &got.Status, &got.ExpiresAt, &got.PaidAt, &got.CreatedAt,
		); err != nil {
			if isUniqueViolation(err) {
				return payment.Request{}, nil, nil, fmt.Errorf("%w: va_number collision (%s)", apperrors.ErrConflict, v.VANumber)
			}
			return payment.Request{}, nil, nil, fmt.Errorf("insert VA: %w", err)
		}
		outVAs = append(outVAs, got)
	}

	var outQR *qris.Code
	if qr != nil {
		qr.PaymentRequestID = created.ID
		row := tx.QueryRow(ctx, `
			INSERT INTO pay_qris_codes
			  (tenant_id, payment_request_id, qr_string, qr_image_url, expected_amount, status, expires_at)
			VALUES ($1, $2, $3, $4, $5, 'ACTIVE', $6)
			RETURNING id, tenant_id, payment_request_id, qr_string, COALESCE(qr_image_url,''),
			          expected_amount, status, expires_at, paid_at, created_at`,
			created.TenantID, created.ID, qr.QRString, qr.QRImageURL, qr.ExpectedAmount, qr.ExpiresAt,
		)
		var got qris.Code
		if err := row.Scan(
			&got.ID, &got.TenantID, &got.PaymentRequestID, &got.QRString, &got.QRImageURL,
			&got.ExpectedAmount, &got.Status, &got.ExpiresAt, &got.PaidAt, &got.CreatedAt,
		); err != nil {
			return payment.Request{}, nil, nil, fmt.Errorf("insert QRIS: %w", err)
		}
		outQR = &got
	}

	if err := tx.Commit(ctx); err != nil {
		return payment.Request{}, nil, nil, fmt.Errorf("commit: %w", err)
	}
	return created, outVAs, outQR, nil
}

// Get fetches a single payment request.
func (r *PaymentRepo) Get(ctx context.Context, tenantID, id string) (payment.Request, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+prColumns+` FROM pay_payment_requests WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	pr, err := scanPR(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return payment.Request{}, fmt.Errorf("%w: payment request %s", apperrors.ErrNotFound, id)
		}
		return payment.Request{}, err
	}
	return pr, nil
}

// GetByRequestNumber is a convenience for the simulator + webhook lookups.
func (r *PaymentRepo) GetByRequestNumber(ctx context.Context, tenantID, number string) (payment.Request, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+prColumns+` FROM pay_payment_requests WHERE tenant_id = $1 AND request_number = $2`, tenantID, number)
	pr, err := scanPR(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return payment.Request{}, fmt.Errorf("%w: request_number %s", apperrors.ErrNotFound, number)
		}
		return payment.Request{}, err
	}
	return pr, nil
}

// List returns payment requests filtered by status (empty = all).
func (r *PaymentRepo) List(ctx context.Context, tenantID, status string) ([]payment.Request, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		rows, err = r.pool.Query(ctx, `SELECT `+prColumns+` FROM pay_payment_requests WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT `+prColumns+` FROM pay_payment_requests WHERE tenant_id = $1 AND status = $2 ORDER BY created_at DESC`, tenantID, status)
	}
	if err != nil {
		return nil, fmt.Errorf("list PR: %w", err)
	}
	defer rows.Close()
	out := make([]payment.Request, 0, 8)
	for rows.Next() {
		pr, err := scanPR(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}

// MarkSettled atomically transitions PENDING -> SETTLED and writes settled_at
// + settled_amount. Returns ErrConflict if the request is no longer PENDING.
func (r *PaymentRepo) MarkSettled(ctx context.Context, tenantID, id string, amount int64) (payment.Request, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE pay_payment_requests
		   SET status = 'SETTLED',
		       settled_at = NOW(),
		       settled_amount = $3,
		       updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'PENDING'
		 RETURNING `+prColumns, tenantID, id, amount)
	pr, err := scanPR(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return payment.Request{}, fmt.Errorf("%w: PR %s not in PENDING", apperrors.ErrConflict, id)
		}
		return payment.Request{}, err
	}
	return pr, nil
}

// Cancel transitions PENDING -> CANCELLED.
func (r *PaymentRepo) Cancel(ctx context.Context, tenantID, id string) (payment.Request, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE pay_payment_requests
		   SET status = 'CANCELLED', updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'PENDING'
		 RETURNING `+prColumns, tenantID, id)
	pr, err := scanPR(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return payment.Request{}, fmt.Errorf("%w: PR %s not in PENDING", apperrors.ErrConflict, id)
		}
		return payment.Request{}, err
	}
	return pr, nil
}

// MarkVAPAID is called from the webhook to flip the VA row to PAID.
func (r *PaymentRepo) MarkVAPAID(ctx context.Context, vaID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE pay_virtual_accounts SET status='PAID', paid_at=NOW() WHERE id=$1`, vaID)
	return err
}

// ListVAsByRequest returns the VA rows attached to a payment request.
func (r *PaymentRepo) ListVAsByRequest(ctx context.Context, requestID string) ([]va.Account, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, payment_request_id, bank_code, va_number, va_name,
		       expected_amount, status, expires_at, paid_at, created_at
		  FROM pay_virtual_accounts
		 WHERE payment_request_id = $1
		 ORDER BY bank_code`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]va.Account, 0, 4)
	for rows.Next() {
		var v va.Account
		if err := rows.Scan(
			&v.ID, &v.TenantID, &v.PaymentRequestID, &v.BankCode, &v.VANumber,
			&v.VAName, &v.ExpectedAmount, &v.Status, &v.ExpiresAt, &v.PaidAt, &v.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ListQRISByRequest returns the QRIS row attached to a payment request.
func (r *PaymentRepo) ListQRISByRequest(ctx context.Context, requestID string) (*qris.Code, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, payment_request_id, qr_string, COALESCE(qr_image_url,''),
		       expected_amount, status, expires_at, paid_at, created_at
		  FROM pay_qris_codes
		 WHERE payment_request_id = $1
		 ORDER BY created_at DESC LIMIT 1`, requestID)
	var q qris.Code
	if err := row.Scan(
		&q.ID, &q.TenantID, &q.PaymentRequestID, &q.QRString, &q.QRImageURL,
		&q.ExpectedAmount, &q.Status, &q.ExpiresAt, &q.PaidAt, &q.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

// MarkExpired scans for PENDING requests past their expires_at and flips
// them. Used by the outbox tick to opportunistically GC.
func (r *PaymentRepo) MarkExpired(ctx context.Context, now time.Time) (int, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE pay_payment_requests
		   SET status = 'EXPIRED', updated_at = NOW()
		 WHERE status = 'PENDING' AND expires_at <= $1`, now)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}