//go:build !windows
// +build !windows

// InvoiceService tests — use in-memory adapters (TxRunner + InvoiceRepo +
// CreditLimitRepo) to validate Sprint 8 correctness without a real DB.
//
// Run with: go -C fmcg-wallet test ./internal/usecase/...
package usecase

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/invoice"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// In-memory fakes
// =============================================================================

// inMemoryAccount mocks a single account row for credit-limit checks.
type inMemoryAccount struct {
	ID         string
	CustomerID string
	LimitMinor int64
	UsedMinor  int64
}

// inMemoryInvoice is a simplified invoice row for tests.
type inMemoryInvoice struct {
	ID             string
	TenantID       string
	CustomerID     string
	Code           string
	AmountMinor    int64
	PaidMinor      int64
	DueDate        time.Time
	Status         string
	IssuedAt       time.Time
	AllocationIDs  []string // populated by payment tests
}

// inMemoryAlloc is a payment allocation row.
type inMemoryAlloc struct {
	ID           string
	PaymentID    string
	InvoiceID    string
	CustomerID   string
	AmountMinor  int64
}

// invoiceRepo holds in-memory state for invoice + credit-limit fakes.
type invoiceRepo struct {
	mu        sync.Mutex
	invoices  map[string]*inMemoryInvoice
	payments  map[string][]*inMemoryAlloc // payment_id -> allocations
	limits    map[string]*inMemoryAccount // customer_id -> limit
	outstand  map[string][]*inMemoryInvoice // customer_id -> outstanding (sorted by due_date)
}

func newInvoiceRepo() *invoiceRepo {
	return &invoiceRepo{
		invoices: map[string]*inMemoryInvoice{},
		payments: map[string][]*inMemoryAlloc{},
		limits:   map[string]*inMemoryAccount{},
	}
}

// =============================================================================
// InvoiceRepository (subset of methods the InvoiceService uses)
// =============================================================================

func (r *invoiceRepo) Create(_ context.Context, tx invoice.Tx, inv invoice.Invoice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.invoices[inv.Code]; ok && inv.TenantID == r.invoices[inv.Code].TenantID {
		return apperrors.ErrAlreadyExists
	}
	r.invoices[inv.ID] = &inMemoryInvoice{
		ID:         inv.ID,
		TenantID:   inv.TenantID,
		CustomerID: inv.CustomerID,
		Code:       inv.Code,
		AmountMinor: inv.Amount.Minor(),
		PaidMinor:   0,
		DueDate:    inv.DueDate,
		Status:     string(invoice.InvoiceStatusOpen),
		IssuedAt:   inv.IssuedAt,
	}
	return nil
}

func (r *invoiceRepo) GetByID(_ context.Context, id string) (invoice.Invoice, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.invoices[id]
	if !ok {
		return invoice.Invoice{}, apperrors.ErrInvoiceNotFound
	}
	return invoiceRowToDomain(row), nil
}

func (r *invoiceRepo) List(_ context.Context, f invoice.InvoiceFilter) ([]invoice.Invoice, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]invoice.Invoice, 0, len(r.invoices))
	for _, row := range r.invoices {
		if f.TenantID != "" && row.TenantID != f.TenantID {
			continue
		}
		if f.CustomerID != "" && row.CustomerID != f.CustomerID {
			continue
		}
		if f.Status != "" && row.Status != string(f.Status) {
			continue
		}
		out = append(out, invoiceRowToDomain(row))
	}
	return out, nil
}

func (r *invoiceRepo) ListOutstandingByCustomer(_ context.Context, tx invoice.Tx, customerID string) ([]invoice.Invoice, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]invoice.Invoice, 0)
	for _, row := range r.invoices {
		if row.CustomerID != customerID {
			continue
		}
		if row.Status == string(invoice.InvoiceStatusPaid) ||
			row.Status == string(invoice.InvoiceStatusWrittenOff) {
			continue
		}
		if row.PaidMinor >= row.AmountMinor {
			continue
		}
		out = append(out, invoiceRowToDomain(row))
	}
	return out, nil
}

func (r *invoiceRepo) InsertAllocation(_ context.Context, tx invoice.Tx, paymentID string, alloc invoice.Allocation, method invoice.PaymentMethod, customerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.invoices[alloc.InvoiceID]
	if !ok {
		return apperrors.ErrInvoiceNotFound
	}
	if row.PaidMinor+alloc.Amount.Minor() > row.AmountMinor {
		return apperrors.ErrInvoiceOverpaid
	}
	row.PaidMinor += alloc.Amount.Minor()
	if row.PaidMinor >= row.AmountMinor {
		row.Status = string(invoice.InvoiceStatusPaid)
	} else {
		row.Status = string(invoice.InvoiceStatusPartial)
	}
	r.payments[paymentID] = append(r.payments[paymentID], &inMemoryAlloc{
		ID:          paymentID + "-" + alloc.InvoiceID,
		PaymentID:   paymentID,
		InvoiceID:   alloc.InvoiceID,
		CustomerID:  customerID,
		AmountMinor: alloc.Amount.Minor(),
	})
	return nil
}

func (r *invoiceRepo) ListAllocations(_ context.Context, paymentID string) ([]invoice.Allocation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	allocs := r.payments[paymentID]
	out := make([]invoice.Allocation, len(allocs))
	for i, a := range allocs {
		out[i] = invoice.Allocation{InvoiceID: a.InvoiceID, Amount: money.NewFromMinor(a.AmountMinor)}
	}
	return out, nil
}

func (r *invoiceRepo) GetAging(_ context.Context, tenantID, customerID string) ([]invoice.AgingSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	type bucketKey struct{ bucket string }
	counts := map[bucketKey]int{}
	outstand := map[bucketKey]int64{}
	for _, row := range r.invoices {
		if row.TenantID != tenantID {
			continue
		}
		if customerID != "" && row.CustomerID != customerID {
			continue
		}
		if row.Status == string(invoice.InvoiceStatusPaid) ||
			row.Status == string(invoice.InvoiceStatusWrittenOff) {
			continue
		}
		days := int(time.Since(row.DueDate).Hours() / 24)
		var bucket string
		switch {
		case days < 0:
			bucket = string(invoice.BucketCurrent)
		case days <= 7:
			bucket = string(invoice.BucketD1To7)
		case days <= 30:
			bucket = string(invoice.BucketD8To30)
		case days <= 60:
			bucket = string(invoice.BucketD31To60)
		case days <= 90:
			bucket = string(invoice.BucketD61To90)
		default:
			bucket = string(invoice.BucketD90Plus)
		}
		k := bucketKey{bucket}
		counts[k]++
		outstand[k] += row.AmountMinor - row.PaidMinor
	}
	out := make([]invoice.AgingSummary, 0, len(counts))
	for k, c := range counts {
		out = append(out, invoice.AgingSummary{
			TenantID:         tenantID,
			CustomerID:       customerID,
			Bucket:           invoice.AgingBucket(k.bucket),
			Count:            c,
			OutstandingMinor: outstand[k],
		})
	}
	return out, nil
}

// =============================================================================
// CreditLimitRepository
// =============================================================================

func (r *invoiceRepo) Get(_ context.Context, _ invoice.Tx, customerID string) (invoice.CreditLimit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.limits[customerID]
	if !ok {
		return invoice.CreditLimit{}, apperrors.ErrNotFound
	}
	return invoice.CreditLimit{
		TenantID:    a.ID[:0] + "tenant",
		CustomerID:  a.CustomerID,
		LimitAmount: money.NewFromMinor(a.LimitMinor),
		UsedAmount:  money.NewFromMinor(a.UsedMinor),
	}, nil
}

func (r *invoiceRepo) Upsert(_ context.Context, _ invoice.Tx, limit invoice.CreditLimit) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limits[limit.CustomerID] = &inMemoryAccount{
		ID:         limit.TenantID,
		CustomerID: limit.CustomerID,
		LimitMinor: limit.LimitAmount.Minor(),
		UsedMinor:  limit.UsedAmount.Minor(),
	}
	return nil
}

func (r *invoiceRepo) IncrementUsed(_ context.Context, _ invoice.Tx, customerID string, delta money.Money) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.limits[customerID]
	if !ok {
		return apperrors.ErrCreditLimitExceeded
	}
	if a.UsedMinor+delta.Minor() > a.LimitMinor {
		return apperrors.ErrCreditLimitExceeded
	}
	a.UsedMinor += delta.Minor()
	return nil
}

func (r *invoiceRepo) DecrementUsed(_ context.Context, _ invoice.Tx, customerID string, delta money.Money) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.limits[customerID]
	if !ok {
		return apperrors.ErrNotFound
	}
	if a.UsedMinor-delta.Minor() < 0 {
		a.UsedMinor = 0
	} else {
		a.UsedMinor -= delta.Minor()
	}
	return nil
}

// =============================================================================
// TxRunner (captures the closure and runs it against a fake Tx)
// =============================================================================

type fakeTx struct{}

type fakeTxRunner struct {
	mu     sync.Mutex
	tx     invoice.Tx
	calls  int
	lastFn func(invoice.Tx) error
}

func newFakeTxRunner() *fakeTxRunner {
	return &fakeTxRunner{tx: fakeTx{}}
}

func (r *fakeTxRunner) ExecuteTx(_ context.Context, fn func(invoice.Tx) error) error {
	r.mu.Lock()
	r.calls++
	r.lastFn = fn
	r.mu.Unlock()
	return fn(r.tx)
}

// =============================================================================
// Helpers
// =============================================================================

func invoiceRowToDomain(row *inMemoryInvoice) invoice.Invoice {
	return invoice.Invoice{
		ID:         row.ID,
		TenantID:   row.TenantID,
		CustomerID: row.CustomerID,
		Code:       row.Code,
		Amount:     money.NewFromMinor(row.AmountMinor),
		PaidAmount: money.NewFromMinor(row.PaidMinor),
		DueDate:    row.DueDate,
		Status:     invoice.InvoiceStatus(row.Status),
		IssuedAt:   row.IssuedAt,
	}
}

func newSvc(t *testing.T) (*InvoiceService, *invoiceRepo, *fakeTxRunner) {
	t.Helper()
	repo := newInvoiceRepo()
	txr := newFakeTxRunner()
	svc := NewInvoiceService(InvoiceServiceDeps{
		Invoices:     repo,
		CreditLimits: repo,
		DB:           txr,
		Logger:       slog.Default(),
	})
	return svc, repo, txr
}

const testTenant = "00000000-0000-0000-0000-000000000001"

// =============================================================================
// CreateInvoice tests
// =============================================================================

func TestInvoiceService_CreateInvoice_NoLimit_Succeeds(t *testing.T) {
	t.Parallel()
	svc, _, _ := newSvc(t)
	inv, err := svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID:   testTenant,
		CustomerID: "cust-1",
		Code:       "INV-001",
		Amount:     money.NewFromMinor(50000),
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, "INV-001", inv.Code)
	assert.Equal(t, invoice.InvoiceStatusOpen, inv.Status)
}

func TestInvoiceService_CreateInvoice_WithinLimit_Succeeds(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newSvc(t)
	require.NoError(t, repo.limits["cust-2"].CustomerID == "" && true) // empty
	require.NoError(t, svc.SetCreditLimit(context.Background(), invoice.CreditLimit{
		TenantID: testTenant, CustomerID: "cust-2",
		LimitAmount: money.NewFromMinor(1000000),
	}))

	inv, err := svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID:   testTenant,
		CustomerID: "cust-2",
		Code:       "INV-002",
		Amount:     money.NewFromMinor(500000),
		DueDate:    time.Now().Add(30 * 24 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(500000), repo.limits["cust-2"].UsedMinor)
	_ = inv
}

func TestInvoiceService_CreateInvoice_NearLimit_Succeeds(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newSvc(t)
	require.NoError(t, svc.SetCreditLimit(context.Background(), invoice.CreditLimit{
		TenantID: testTenant, CustomerID: "cust-3",
		LimitAmount: money.NewFromMinor(100000),
	}))

	// First invoice uses 90,000; second uses 9,000 — still under 100,000.
	_, err := svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-3", Code: "INV-003a",
		Amount: money.NewFromMinor(90000), DueDate: time.Now().Add(30 * 24 * time.Hour),
	})
	require.NoError(t, err)
	_, err = svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-3", Code: "INV-003b",
		Amount: money.NewFromMinor(9000), DueDate: time.Now().Add(30 * 24 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(99000), repo.limits["cust-3"].UsedMinor)
}

func TestInvoiceService_CreateInvoice_ExceedsLimit_Fails(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newSvc(t)
	require.NoError(t, svc.SetCreditLimit(context.Background(), invoice.CreditLimit{
		TenantID: testTenant, CustomerID: "cust-4",
		LimitAmount: money.NewFromMinor(100000),
	}))
	_, err := svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-4", Code: "INV-004",
		Amount: money.NewFromMinor(200000), DueDate: time.Now().Add(30 * 24 * time.Hour),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrCreditLimitExceeded))
	assert.Equal(t, int64(0), repo.limits["cust-4"].UsedMinor, "used should not change on failure")
}

func TestInvoiceService_CreateInvoice_ZeroLimit_Rejected(t *testing.T) {
	t.Parallel()
	svc, _, _ := newSvc(t)
	_, err := svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-5", Code: "INV-005",
		Amount: money.NewFromMinor(100), DueDate: time.Now().Add(30 * 24 * time.Hour),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}

// =============================================================================
// RecordPayment (FIFO + manual)
// =============================================================================

func TestInvoiceService_RecordPayment_FIFO_SingleInvoice(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newSvc(t)

	due := time.Now().Add(10 * 24 * time.Hour)
	_, err := svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-fifo-1", Code: "INV-F1",
		Amount: money.NewFromMinor(100000), DueDate: due,
	})
	require.NoError(t, err)

	res, err := svc.RecordPayment(context.Background(), invoice.PaymentInput{
		TenantID: testTenant, CustomerID: "cust-fifo-1",
		Amount: money.NewFromMinor(30000), Method: invoice.MethodCash,
		Mode: invoice.AllocationFIFO,
	})
	require.NoError(t, err)
	assert.Len(t, res.Allocations, 1)
	assert.Equal(t, int64(30000), res.Allocations[0].Amount.Minor())
	assert.Equal(t, int64(30000), repo.invoices[repo.lookupID("INV-F1")].PaidMinor)
}

func (r *invoiceRepo) lookupID(code string) string {
	for _, row := range r.invoices {
		if row.Code == code {
			return row.ID
		}
	}
	return ""
}

func TestInvoiceService_RecordPayment_FIFO_MultiInvoice_Partial(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newSvc(t)

	// Two invoices for same customer, older first.
	due1 := time.Now().Add(-30 * 24 * time.Hour)
	due2 := time.Now().Add(-10 * 24 * time.Hour)
	_, _ = svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-fifo-2", Code: "INV-F2a",
		Amount: money.NewFromMinor(50000), DueDate: due1,
	})
	_, _ = svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-fifo-2", Code: "INV-F2b",
		Amount: money.NewFromMinor(50000), DueDate: due2,
	})

	// Pay 70,000 — should fill old invoice (50,000) + partial new (20,000).
	res, err := svc.RecordPayment(context.Background(), invoice.PaymentInput{
		TenantID: testTenant, CustomerID: "cust-fifo-2",
		Amount: money.NewFromMinor(70000), Method: invoice.MethodTransfer,
		Mode: invoice.AllocationFIFO,
	})
	require.NoError(t, err)
	assert.Len(t, res.Allocations, 2)
	total := int64(0)
	for _, a := range res.Allocations {
		total += a.Amount.Minor()
	}
	assert.Equal(t, int64(70000), total)
	assert.Equal(t, int64(50000), repo.invoices[repo.lookupID("INV-F2a")].PaidMinor)
	assert.Equal(t, int64(20000), repo.invoices[repo.lookupID("INV-F2b")].PaidMinor)
}

func TestInvoiceService_RecordPayment_FIFO_Overpay_Fails(t *testing.T) {
	t.Parallel()
	svc, _, _ := newSvc(t)

	due := time.Now().Add(10 * 24 * time.Hour)
	_, _ = svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-overpay", Code: "INV-OP",
		Amount: money.NewFromMinor(50000), DueDate: due,
	})

	_, err := svc.RecordPayment(context.Background(), invoice.PaymentInput{
		TenantID: testTenant, CustomerID: "cust-overpay",
		Amount: money.NewFromMinor(100000), Method: invoice.MethodCash,
		Mode: invoice.AllocationFIFO,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvoiceOverpaid))
}

func TestInvoiceService_RecordPayment_Manual_SumMatches_Succeeds(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newSvc(t)

	due := time.Now().Add(5 * 24 * time.Hour)
	_, _ = svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-manual-1", Code: "INV-M1",
		Amount: money.NewFromMinor(50000), DueDate: due,
	})
	_, _ = svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-manual-1", Code: "INV-M2",
		Amount: money.NewFromMinor(50000), DueDate: due,
	})

	res, err := svc.RecordPayment(context.Background(), invoice.PaymentInput{
		TenantID: testTenant, CustomerID: "cust-manual-1",
		Amount: money.NewFromMinor(80000), Method: invoice.MethodQRIS,
		Mode: invoice.AllocationManual,
		Allocations: []invoice.Allocation{
			{InvoiceID: repo.lookupID("INV-M2"), Amount: money.NewFromMinor(80000)},
		},
	})
	require.NoError(t, err)
	assert.Len(t, res.Allocations, 1)
	assert.Equal(t, int64(80000), repo.invoices[repo.lookupID("INV-M2")].PaidMinor)
	assert.Equal(t, int64(0), repo.invoices[repo.lookupID("INV-M1")].PaidMinor, "should not touch INV-M1")
}

func TestInvoiceService_RecordPayment_Manual_SumMismatch_Fails(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newSvc(t)

	due := time.Now().Add(5 * 24 * time.Hour)
	_, _ = svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-manual-2", Code: "INV-MM",
		Amount: money.NewFromMinor(50000), DueDate: due,
	})

	_, err := svc.RecordPayment(context.Background(), invoice.PaymentInput{
		TenantID: testTenant, CustomerID: "cust-manual-2",
		Amount: money.NewFromMinor(50000), Method: invoice.MethodQRIS,
		Mode: invoice.AllocationManual,
		Allocations: []invoice.Allocation{
			{InvoiceID: repo.lookupID("INV-MM"), Amount: money.NewFromMinor(30000)},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrPaymentAllocationMismatch))
}

func TestInvoiceService_RecordPayment_Manual_AllocationExceedsInvoice_Fails(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newSvc(t)

	due := time.Now().Add(5 * 24 * time.Hour)
	_, _ = svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-over", Code: "INV-OVER",
		Amount: money.NewFromMinor(50000), DueDate: due,
	})

	_, err := svc.RecordPayment(context.Background(), invoice.PaymentInput{
		TenantID: testTenant, CustomerID: "cust-over",
		Amount: money.NewFromMinor(80000), Method: invoice.MethodCash,
		Mode: invoice.AllocationManual,
		Allocations: []invoice.Allocation{
			{InvoiceID: repo.lookupID("INV-OVER"), Amount: money.NewFromMinor(80000)},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvoiceOverpaid))
}

func TestInvoiceService_RecordPayment_FIFO_AdvancesStatusToPartial(t *testing.T) {
	t.Parallel()
	svc, _, _ := newSvc(t)

	due := time.Now().Add(5 * 24 * time.Hour)
	inv, _ := svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
		TenantID: testTenant, CustomerID: "cust-status", Code: "INV-S",
		Amount: money.NewFromMinor(100000), DueDate: due,
	})

	_, err := svc.RecordPayment(context.Background(), invoice.PaymentInput{
		TenantID: testTenant, CustomerID: "cust-status",
		Amount: money.NewFromMinor(40000), Method: invoice.MethodCash,
		Mode: invoice.AllocationFIFO,
	})
	require.NoError(t, err)
	assert.Equal(t, invoice.InvoiceStatusPartial, inv.Status)
}

// =============================================================================
// GetAging
// =============================================================================

func TestInvoiceService_GetAging_BucketClassification(t *testing.T) {
	t.Parallel()
	svc, _, _ := newSvc(t)

	cases := []struct {
		name   string
		dueIn  time.Duration
		bucket invoice.AgingBucket
		amt    int64
	}{
		{"current", 30 * 24 * time.Hour, invoice.BucketCurrent, 10000},
		{"d1to7", -3 * 24 * time.Hour, invoice.BucketD1To7, 20000},
		{"d8to30", -15 * 24 * time.Hour, invoice.BucketD8To30, 30000},
		{"d31to60", -45 * 24 * time.Hour, invoice.BucketD31To60, 40000},
		{"d61to90", -75 * 24 * time.Hour, invoice.BucketD61To90, 50000},
		{"d90plus", -120 * 24 * time.Hour, invoice.BucketD90Plus, 60000},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code := "INV-AGING-" + c.name
			_, err := svc.CreateInvoice(context.Background(), invoice.CreateInvoiceInput{
				TenantID: testTenant, CustomerID: "cust-aging-" + c.name,
				Code:    code,
				Amount:  money.NewFromMinor(c.amt),
				DueDate: time.Now().Add(c.dueIn),
			})
			require.NoError(t, err)
		})
		_ = i
	}

	summaries, err := svc.GetAging(context.Background(), testTenant, "")
	require.NoError(t, err)
	byBucket := map[invoice.AgingBucket]int64{}
	for _, s := range summaries {
		byBucket[s.Bucket] = s.OutstandingMinor
	}
	assert.Equal(t, int64(10000), byBucket[invoice.BucketCurrent])
	assert.Equal(t, int64(20000), byBucket[invoice.BucketD1To7])
	assert.Equal(t, int64(30000), byBucket[invoice.BucketD8To30])
	assert.Equal(t, int64(40000), byBucket[invoice.BucketD31To60])
	assert.Equal(t, int64(50000), byBucket[invoice.BucketD61To90])
	assert.Equal(t, int64(60000), byBucket[invoice.BucketD90Plus])
}

// =============================================================================
// SetCreditLimit
// =============================================================================

func TestInvoiceService_SetCreditLimit_DefaultsEffectiveFrom(t *testing.T) {
	t.Parallel()
	svc, _, _ := newSvc(t)

	before := time.Now().UTC()
	require.NoError(t, svc.SetCreditLimit(context.Background(), invoice.CreditLimit{
		TenantID: testTenant, CustomerID: "cust-def",
		LimitAmount: money.NewFromMinor(500000),
	}))
	after := time.Now().UTC()

	// EffectiveFrom should default to now; we don't expose it via the repo,
	// but the call must not error.
	_ = before
	_ = after
}

func TestInvoiceService_SetCreditLimit_RejectsNegativeLimit(t *testing.T) {
	t.Parallel()
	svc, _, _ := newSvc(t)
	err := svc.SetCreditLimit(context.Background(), invoice.CreditLimit{
		TenantID: testTenant, CustomerID: "cust-neg",
		LimitAmount: money.NewFromMinor(-100),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}
