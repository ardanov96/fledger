// Package invoice defines the invoice & receivables domain.
//
// This package is interface-only; the Postgres implementation lives in
// internal/repository/postgres. Domain has zero dependencies on infrastructure
// (no SQL, no HTTP) so it can be unit-tested without spinning up services.
//
// Design notes:
//   - An Invoice tracks money owed by a customer (receivable).
//   - A Payment is the incoming cash event; one Payment can be allocated
//     across many Invoices via PaymentAllocation rows.
//   - CreditLimit is a per-customer cap; exceeded → ErrCreditLimitExceeded.
package invoice

import (
	"context"
	"time"

	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// InvoiceStatus is the lifecycle state of an invoice.
type InvoiceStatus string

const (
	InvoiceStatusOpen        InvoiceStatus = "open"
	InvoiceStatusPartial     InvoiceStatus = "partial"
	InvoiceStatusPaid        InvoiceStatus = "paid"
	InvoiceStatusOverdue     InvoiceStatus = "overdue"
	InvoiceStatusWrittenOff  InvoiceStatus = "written_off"
)

// AgingBucket classifies outstanding invoices by days-overdue.
type AgingBucket string

const (
	BucketCurrent   AgingBucket = "current"
	BucketD1To7     AgingBucket = "d_1_7"
	BucketD8To30    AgingBucket = "d_8_30"
	BucketD31To60   AgingBucket = "d_31_60"
	BucketD61To90   AgingBucket = "d_61_90"
	BucketD90Plus   AgingBucket = "d_90_plus"
)

// Invoice is an outstanding receivable from a customer.
type Invoice struct {
	ID          string
	TenantID    string
	CustomerID  string
	Code        string
	Amount      money.Money
	PaidAmount  money.Money
	DueDate     time.Time
	Status      InvoiceStatus
	IssuedAt    time.Time
	PeriodID    string
	Description string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Outstanding returns the unpaid portion (amount - paid).
func (i Invoice) Outstanding() money.Money {
	return i.Amount.Sub(i.PaidAmount)
}

// IsFullyPaid returns true when paid_amount >= amount.
func (i Invoice) IsFullyPaid() bool {
	return i.PaidAmount.Cmp(i.Amount) >= 0
}

// AllocationMode controls how a payment is distributed across invoices.
type AllocationMode string

const (
	AllocationFIFO   AllocationMode = "fifo"
	AllocationManual AllocationMode = "manual"
)

// PaymentMethod is how the customer paid.
type PaymentMethod string

const (
	MethodCash          PaymentMethod = "cash"
	MethodTransfer      PaymentMethod = "transfer"
	MethodQRIS          PaymentMethod = "qris"
	MethodManualAdjust  PaymentMethod = "manual_adjust"
)

// Allocation is one slice of a payment applied to one invoice.
type Allocation struct {
	InvoiceID string
	Amount    money.Money
}

// CreateInvoiceInput is the request payload for CreateInvoice.
// Lives in domain (not usecase) so handlers can import it without a cycle.
type CreateInvoiceInput struct {
	TenantID    string
	CustomerID  string
	Code        string
	Amount      money.Money
	DueDate     time.Time
	Description string
	Metadata    map[string]any
}

// PaymentInput is what the use case receives for RecordPayment.
type PaymentInput struct {
	TenantID       string
	CustomerID     string
	Amount         money.Money
	Method         PaymentMethod
	Mode           AllocationMode
	Allocations    []Allocation
	IdempotencyKey string
	InitiatorID    string
	Description    string
}

// PaymentResult is the result of a successful RecordPayment.
//
// PaymentID groups all allocation rows from the same payment event — useful
// for reversing the payment later (Fase 4+) by deleting allocations in a tx.
type PaymentResult struct {
	PaymentID   string
	Allocations []Allocation
	Method      PaymentMethod
	CustomerID  string
	TotalMinor  int64
}

// InvoiceFilter holds filter criteria for List operations.
type InvoiceFilter struct {
	TenantID    string
	CustomerID  string
	Status      InvoiceStatus
	AgingBucket AgingBucket
	Limit       int
	Cursor      string
}

// AgingSummary is the aggregated aging per customer (or global).
type AgingSummary struct {
	TenantID         string
	CustomerID       string
	Bucket           AgingBucket
	Count            int
	OutstandingMinor int64
}

// CreditLimit represents the cap and current usage for a customer.
type CreditLimit struct {
	TenantID      string
	CustomerID    string
	LimitAmount   money.Money
	UsedAmount    money.Money
	EffectiveFrom time.Time
	UpdatedAt     time.Time
}

// Available returns limit - used (remaining headroom).
func (c CreditLimit) Available() money.Money {
	return c.LimitAmount.Sub(c.UsedAmount)
}

// IsExceeded returns true if used > limit.
func (c CreditLimit) IsExceeded() bool {
	return c.UsedAmount.Cmp(c.LimitAmount) > 0
}

// =============================================================================
// Repository interfaces
// =============================================================================

// InvoiceRepository defines persistence operations for invoices.
type InvoiceRepository interface {
	Create(ctx context.Context, tx Tx, inv Invoice) error
	GetByID(ctx context.Context, id string) (Invoice, error)
	GetByCode(ctx context.Context, code string) (Invoice, error)
	List(ctx context.Context, filter InvoiceFilter) ([]Invoice, error)
	ListOutstandingByCustomer(ctx context.Context, tx Tx, customerID string) ([]Invoice, error)
	InsertAllocation(ctx context.Context, tx Tx, paymentID string, alloc Allocation, method PaymentMethod, customerID string) error
	ListAllocations(ctx context.Context, paymentID string) ([]Allocation, error)
	GetAging(ctx context.Context, tenantID, customerID string) ([]AgingSummary, error)
}

// AgingSnapshotRepository manages the denormalized aging snapshot table
// populated by the AgingRecalculator worker (Sprint 25 / Fase 4D).
//
// Pattern: worker truncates + reinserts in one transaction. API reads from
// the snapshot for O(indexed lookup) instead of the live GROUP BY view.
//
// All methods are read/write only (no Tx parameter) because:
//   - Truncate + insert is one atomic batch operation
//   - Snapshot reads use the pool directly (no RLS context needed — same
//     RLS policy as the underlying table)
type AgingSnapshotRepository interface {
	// UpsertAgingSnapshots replaces ALL snapshot rows for the given run.
	// Caller passes a fresh slice (typically one entry per (customer, bucket)).
	// Implementation wraps in one tx for atomicity.
	UpsertAgingSnapshots(ctx context.Context, runID string, snapshots []AgingSnapshot) error

	// GetAgingSnapshot reads from aging_snapshots. Empty slice + nil error
	// when no snapshot exists yet (caller should fall back to live view).
	GetAgingSnapshot(ctx context.Context, tenantID, customerID string) ([]AgingSummary, error)

	// ListCustomersWithOutstanding returns distinct (tenant_id, customer_id)
	// pairs that have open invoices — used by the worker to iterate.
	ListCustomersWithOutstanding(ctx context.Context) ([]CustomerRef, error)

	// StartRun creates a row in aging_snapshot_runs with status='running'
	// and returns the new run ID.
	StartRun(ctx context.Context) (string, error)

	// FinishRun updates a run with status='ok'/'failed' + duration + counts.
	FinishRun(ctx context.Context, runID string, status RunStatus, tenants, customers, rows int, durationMs int64, errMsg string) error

	// LatestRun returns the most recent run (for diagnostics / /readyz).
	LatestRun(ctx context.Context) (*SnapshotRun, error)
}

// AgingSnapshot is one row in aging_snapshots (denormalized cache).
type AgingSnapshot struct {
	TenantID         string
	CustomerID       string
	Bucket           AgingBucket
	Count            int
	OutstandingMinor int64
	SnapshotAt       time.Time
	SnapshotRunID    string
}

// CustomerRef is a minimal (tenant_id, customer_id) reference for the
// worker's iteration loop.
type CustomerRef struct {
	TenantID   string
	CustomerID string
}

// RunStatus mirrors the CHECK constraint on aging_snapshot_runs.status.
type RunStatus string

const (
	RunStatusRunning RunStatus = "running"
	RunStatusOK      RunStatus = "ok"
	RunStatusFailed  RunStatus = "failed"
)

// SnapshotRun is one row in aging_snapshot_runs — audit trail for ops.
type SnapshotRun struct {
	ID                 string
	StartedAt          time.Time
	FinishedAt         *time.Time
	TenantsProcessed   int
	CustomersProcessed int
	RowsWritten        int
	DurationMs         int64
	Status             RunStatus
	Error              string
}

// CreditLimitRepository defines persistence operations for credit limits.
type CreditLimitRepository interface {
	Get(ctx context.Context, tx Tx, customerID string) (CreditLimit, error)
	Upsert(ctx context.Context, tx Tx, limit CreditLimit) error
	IncrementUsed(ctx context.Context, tx Tx, customerID string, delta money.Money) error
	DecrementUsed(ctx context.Context, tx Tx, customerID string, delta money.Money) error
}

// =============================================================================
// Transaction abstraction
// =============================================================================

type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

type CommandTag interface {
	RowsAffected() int64
}

type Row interface {
	Scan(dest ...any) error
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}
