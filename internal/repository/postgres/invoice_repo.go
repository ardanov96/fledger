package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/invoice"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// DTOs
// =============================================================================

type InvoiceDTO struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	CustomerID  uuid.UUID
	Code        string
	Amount      int64
	PaidAmount  int64
	DueDate     time.Time
	Status      string
	IssuedAt    time.Time
	PeriodID    uuid.UUID
	Description *string
	Metadata    []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreditLimitDTO struct {
	TenantID      uuid.UUID
	CustomerID    uuid.UUID
	LimitAmount   int64
	UsedAmount    int64
	EffectiveFrom time.Time
	UpdatedAt     time.Time
}

// =============================================================================
// InvoiceRepository
// =============================================================================

type InvoiceRepository struct {
	db *DB
}

func NewInvoiceRepository(db *DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) assertInvoiceTx(tx invoice.Tx) (*invoiceTxAdapter, error) {
	a, ok := tx.(*invoiceTxAdapter)
	if !ok {
		return nil, fmt.Errorf("expected *invoiceTxAdapter, got %T", tx)
	}
	return a, nil
}

func (r *InvoiceRepository) Create(ctx context.Context, tx invoice.Tx, inv invoice.Invoice) error {
	a, err := r.assertInvoiceTx(tx)
	if err != nil {
		return err
	}

	const q = `
INSERT INTO invoices (
    id, tenant_id, customer_id, code, amount, paid_amount,
    due_date, status, issued_at, period_id, description, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12
)
`
	_, err = a.pgxTx.Exec(ctx, q,
		inv.ID, inv.TenantID, inv.CustomerID, inv.Code,
		inv.Amount.Minor(), inv.PaidAmount.Minor(),
		inv.DueDate, string(inv.Status), inv.IssuedAt,
		inv.PeriodID, nullStr(inv.Description), jsonRaw(inv.Metadata),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.ErrAlreadyExists
		}
		return fmt.Errorf("create invoice: %w", err)
	}
	return nil
}

func (r *InvoiceRepository) GetByID(ctx context.Context, id string) (invoice.Invoice, error) {
	const q = `
SELECT id, tenant_id, customer_id, code, amount, paid_amount,
       due_date, status, issued_at, period_id, description, metadata,
       created_at, updated_at
FROM invoices
WHERE id = $1
`
	dto, err := scanInvoice(r.db.Pool.QueryRow(ctx, q, id))
	if err != nil {
		return invoice.Invoice{}, err
	}
	return dtoToInvoice(dto), nil
}

func (r *InvoiceRepository) GetByCode(ctx context.Context, code string) (invoice.Invoice, error) {
	const q = `
SELECT id, tenant_id, customer_id, code, amount, paid_amount,
       due_date, status, issued_at, period_id, description, metadata,
       created_at, updated_at
FROM invoices
WHERE code = $1
LIMIT 1
`
	dto, err := scanInvoice(r.db.Pool.QueryRow(ctx, q, code))
	if err != nil {
		return invoice.Invoice{}, err
	}
	return dtoToInvoice(dto), nil
}

func (r *InvoiceRepository) List(ctx context.Context, filter invoice.InvoiceFilter) ([]invoice.Invoice, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	const q = `
SELECT id, tenant_id, customer_id, code, amount, paid_amount,
       due_date, status, issued_at, period_id, description, metadata,
       created_at, updated_at
FROM invoices
WHERE ($1::uuid IS NULL OR tenant_id = $1)
  AND ($2::uuid IS NULL OR customer_id = $2)
  AND ($3::text IS NULL OR status = $3)
  AND ($4::text IS NULL OR id < $4)
ORDER BY id DESC
LIMIT $5
`
	var tenantID, customerID *uuid.UUID
	if filter.TenantID != "" {
		id, err := uuid.Parse(filter.TenantID)
		if err != nil {
			return nil, fmt.Errorf("parse tenant id: %w", err)
		}
		tenantID = &id
	}
	if filter.CustomerID != "" {
		id, err := uuid.Parse(filter.CustomerID)
		if err != nil {
			return nil, fmt.Errorf("parse customer id: %w", err)
		}
		customerID = &id
	}

	var status *string
	if filter.Status != "" {
		s := string(filter.Status)
		status = &s
	}

	rows, err := r.db.Pool.Query(ctx, q, tenantID, customerID, status, nullStr(filter.Cursor), limit)
	if err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	out := make([]invoice.Invoice, 0, limit)
	for rows.Next() {
		var dto InvoiceDTO
		if err := rows.Scan(
			&dto.ID, &dto.TenantID, &dto.CustomerID, &dto.Code,
			&dto.Amount, &dto.PaidAmount, &dto.DueDate, &dto.Status,
			&dto.IssuedAt, &dto.PeriodID, &dto.Description, &dto.Metadata,
			&dto.CreatedAt, &dto.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}
		out = append(out, dtoToInvoice(dto))
	}
	return out, rows.Err()
}

func (r *InvoiceRepository) ListOutstandingByCustomer(ctx context.Context, tx invoice.Tx, customerID string) ([]invoice.Invoice, error) {
	a, err := r.assertInvoiceTx(tx)
	if err != nil {
		return nil, err
	}

	const q = `
SELECT id, tenant_id, customer_id, code, amount, paid_amount,
       due_date, status, issued_at, period_id, description, metadata,
       created_at, updated_at
FROM invoices
WHERE customer_id = $1
  AND status IN ('open', 'partial', 'overdue')
ORDER BY due_date ASC, id ASC
FOR UPDATE
`
	rows, err := a.pgxTx.Query(ctx, q, customerID)
	if err != nil {
		return nil, fmt.Errorf("list outstanding: %w", err)
	}
	defer rows.Close()

	out := make([]invoice.Invoice, 0, 8)
	for rows.Next() {
		var dto InvoiceDTO
		if err := rows.Scan(
			&dto.ID, &dto.TenantID, &dto.CustomerID, &dto.Code,
			&dto.Amount, &dto.PaidAmount, &dto.DueDate, &dto.Status,
			&dto.IssuedAt, &dto.PeriodID, &dto.Description, &dto.Metadata,
			&dto.CreatedAt, &dto.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}
		out = append(out, dtoToInvoice(dto))
	}
	return out, rows.Err()
}

func (r *InvoiceRepository) InsertAllocation(
	ctx context.Context, tx invoice.Tx,
	paymentID string, alloc invoice.Allocation,
	method invoice.PaymentMethod, customerID string,
) error {
	a, err := r.assertInvoiceTx(tx)
	if err != nil {
		return err
	}

	const q = `
INSERT INTO invoice_payments (
    id, tenant_id, payment_id, invoice_id, customer_id,
    amount, method, metadata
) VALUES (
    gen_random_uuid(), $1, $2, $3, $4,
    $5, $6, '{}'
)
`
	var tenantID uuid.UUID
	if err := a.pgxTx.QueryRow(ctx,
		`SELECT tenant_id FROM invoices WHERE id = $1`, alloc.InvoiceID,
	).Scan(&tenantID); err != nil {
		return fmt.Errorf("resolve invoice tenant: %w", err)
	}

	_, err = a.pgxTx.Exec(ctx, q,
		tenantID, paymentID, alloc.InvoiceID, customerID,
		alloc.Amount.Minor(), string(method),
	)
	if err != nil {
		if isCheckViolation(err) {
			return apperrors.ErrInvoiceOverpaid
		}
		return fmt.Errorf("insert allocation: %w", err)
	}
	return nil
}

func (r *InvoiceRepository) ListAllocations(ctx context.Context, paymentID string) ([]invoice.Allocation, error) {
	const q = `
SELECT invoice_id, amount
FROM invoice_payments
WHERE payment_id = $1
ORDER BY allocated_at ASC
`
	rows, err := r.db.Pool.Query(ctx, q, paymentID)
	if err != nil {
		return nil, fmt.Errorf("list allocations: %w", err)
	}
	defer rows.Close()

	out := make([]invoice.Allocation, 0, 4)
	for rows.Next() {
		var alloc invoice.Allocation
		var amountMinor int64
		if err := rows.Scan(&alloc.InvoiceID, &amountMinor); err != nil {
			return nil, fmt.Errorf("scan allocation: %w", err)
		}
		alloc.Amount = money.NewFromMinor(amountMinor)
		out = append(out, alloc)
	}
	return out, rows.Err()
}

func (r *InvoiceRepository) GetAging(ctx context.Context, tenantID, customerID string) ([]invoice.AgingSummary, error) {
	var q string
	var args []any

	if customerID != "" {
		q = `
SELECT bucket, SUM(invoice_count) AS invoice_count, SUM(outstanding_minor) AS outstanding_minor
FROM v_invoice_aging
WHERE tenant_id = $1 AND customer_id = $2
GROUP BY bucket
ORDER BY bucket
`
		args = []any{tenantID, customerID}
	} else {
		q = `
SELECT bucket, SUM(invoice_count) AS invoice_count, SUM(outstanding_minor) AS outstanding_minor
FROM v_invoice_aging
WHERE tenant_id = $1
GROUP BY bucket
ORDER BY bucket
`
		args = []any{tenantID}
	}

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get aging: %w", err)
	}
	defer rows.Close()

	out := make([]invoice.AgingSummary, 0, 6)
	for rows.Next() {
		var s invoice.AgingSummary
		if err := rows.Scan(&s.Bucket, &s.Count, &s.OutstandingMinor); err != nil {
			return nil, fmt.Errorf("scan aging: %w", err)
		}
		s.TenantID = tenantID
		s.CustomerID = customerID
		out = append(out, s)
	}
	return out, rows.Err()
}

// =============================================================================
// CreditLimitRepository
// =============================================================================

type CreditLimitRepository struct {
	db *DB
}

func NewCreditLimitRepository(db *DB) *CreditLimitRepository {
	return &CreditLimitRepository{db: db}
}

func (r *CreditLimitRepository) assertInvoiceTx(tx invoice.Tx) (*invoiceTxAdapter, error) {
	a, ok := tx.(*invoiceTxAdapter)
	if !ok {
		return nil, fmt.Errorf("expected *invoiceTxAdapter, got %T", tx)
	}
	return a, nil
}

func (r *CreditLimitRepository) Get(ctx context.Context, tx invoice.Tx, customerID string) (invoice.CreditLimit, error) {
	a, err := r.assertInvoiceTx(tx)
	if err != nil {
		return invoice.CreditLimit{}, err
	}

	const q = `
SELECT tenant_id, customer_id, limit_amount, used_amount, effective_from, updated_at
FROM credit_limits
WHERE customer_id = $1
LIMIT 1
FOR UPDATE
`
	var dto CreditLimitDTO
	err = a.pgxTx.QueryRow(ctx, q, customerID).Scan(
		&dto.TenantID, &dto.CustomerID, &dto.LimitAmount, &dto.UsedAmount,
		&dto.EffectiveFrom, &dto.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return invoice.CreditLimit{}, apperrors.ErrNotFound
		}
		return invoice.CreditLimit{}, fmt.Errorf("get credit limit: %w", err)
	}
	return dtoToCreditLimit(dto), nil
}

func (r *CreditLimitRepository) Upsert(ctx context.Context, tx invoice.Tx, limit invoice.CreditLimit) error {
	a, err := r.assertInvoiceTx(tx)
	if err != nil {
		return err
	}

	const q = `
INSERT INTO credit_limits (tenant_id, customer_id, limit_amount, used_amount, effective_from)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tenant_id, customer_id) DO UPDATE
SET limit_amount = EXCLUDED.limit_amount,
    effective_from = EXCLUDED.effective_from,
    updated_at = now()
`
	_, err = a.pgxTx.Exec(ctx, q,
		limit.TenantID, limit.CustomerID,
		limit.LimitAmount.Minor(), limit.UsedAmount.Minor(),
		limit.EffectiveFrom,
	)
	if err != nil {
		return fmt.Errorf("upsert credit limit: %w", err)
	}
	return nil
}

func (r *CreditLimitRepository) IncrementUsed(ctx context.Context, tx invoice.Tx, customerID string, delta money.Money) error {
	a, err := r.assertInvoiceTx(tx)
	if err != nil {
		return err
	}

	const q = `
UPDATE credit_limits
SET used_amount = used_amount + $2,
    updated_at = now()
WHERE customer_id = $1
  AND used_amount + $2 <= limit_amount
`
	tag, err := a.pgxTx.Exec(ctx, q, customerID, delta.Minor())
	if err != nil {
		if isCheckViolation(err) {
			return apperrors.ErrCreditLimitExceeded
		}
		return fmt.Errorf("increment used: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrCreditLimitExceeded
	}
	return nil
}

func (r *CreditLimitRepository) DecrementUsed(ctx context.Context, tx invoice.Tx, customerID string, delta money.Money) error {
	a, err := r.assertInvoiceTx(tx)
	if err != nil {
		return err
	}

	const q = `
UPDATE credit_limits
SET used_amount = used_amount - $2,
    updated_at = now()
WHERE customer_id = $1
  AND used_amount >= $2
`
	tag, err := a.pgxTx.Exec(ctx, q, customerID, delta.Minor())
	if err != nil {
		return fmt.Errorf("decrement used: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// =============================================================================
// DTO helpers
// =============================================================================

func scanInvoice(row pgx.Row) (InvoiceDTO, error) {
	var dto InvoiceDTO
	err := row.Scan(
		&dto.ID, &dto.TenantID, &dto.CustomerID, &dto.Code,
		&dto.Amount, &dto.PaidAmount, &dto.DueDate, &dto.Status,
		&dto.IssuedAt, &dto.PeriodID, &dto.Description, &dto.Metadata,
		&dto.CreatedAt, &dto.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InvoiceDTO{}, apperrors.ErrInvoiceNotFound
		}
		return InvoiceDTO{}, fmt.Errorf("scan invoice: %w", err)
	}
	return dto, nil
}

func dtoToInvoice(dto InvoiceDTO) invoice.Invoice {
	return invoice.Invoice{
		ID:          dto.ID.String(),
		TenantID:    dto.TenantID.String(),
		CustomerID:  dto.CustomerID.String(),
		Code:        dto.Code,
		Amount:      money.NewFromMinor(dto.Amount),
		PaidAmount:  money.NewFromMinor(dto.PaidAmount),
		DueDate:     dto.DueDate,
		Status:      invoice.InvoiceStatus(dto.Status),
		IssuedAt:    dto.IssuedAt,
		PeriodID:    dto.PeriodID.String(),
		Description: strPtrToString(dto.Description),
		Metadata:    parseMetadata(dto.Metadata),
		CreatedAt:   dto.CreatedAt,
		UpdatedAt:   dto.UpdatedAt,
	}
}

func dtoToCreditLimit(dto CreditLimitDTO) invoice.CreditLimit {
	return invoice.CreditLimit{
		TenantID:      dto.TenantID.String(),
		CustomerID:    dto.CustomerID.String(),
		LimitAmount:   money.NewFromMinor(dto.LimitAmount),
		UsedAmount:    money.NewFromMinor(dto.UsedAmount),
		EffectiveFrom: dto.EffectiveFrom,
		UpdatedAt:     dto.UpdatedAt,
	}
}

// nullStr returns nil if s is empty, otherwise &s.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
