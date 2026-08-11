// Package usecase — InvoiceService orchestrates invoice creation, payment
// recording, and aging queries.
//
// Critical invariants enforced here:
//   1. Credit limit is checked atomically BEFORE invoice insert.
//   2. Every RecordPayment allocates to invoices using either:
//      a) FIFO (oldest due_date first) — default
//      b) explicit allocations (caller specifies which invoices + amounts)
//   3. Sum of allocations MUST equal payment amount (for manual mode).
//   4. All writes inside ONE DB transaction.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/invoice"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// InvoiceTxRunner is the invoice-flavored transaction runner.
// Mirrors the ledger TxRunner but exposes invoice.Tx to the closure
// (different return-type wrappers for Exec/Query).
type InvoiceTxRunner interface {
	ExecuteTx(ctx context.Context, fn func(invoice.Tx) error) error
}

// EnsureOpenPeriodFunc resolves the current accounting period for a tenant.
type EnsureOpenPeriodFunc func(ctx context.Context, tx invoice.Tx, tenantID string, now time.Time) (string, error)

// InvoiceService implements the invoice use case.
type InvoiceService struct {
	invoices     invoice.InvoiceRepository
	creditLimits invoice.CreditLimitRepository
	db           InvoiceTxRunner
	ensurePeriod EnsureOpenPeriodFunc
	log          *slog.Logger
}

// InvoiceServiceDeps bundles dependencies.
type InvoiceServiceDeps struct {
	Invoices     invoice.InvoiceRepository
	CreditLimits invoice.CreditLimitRepository
	DB           InvoiceTxRunner
	EnsurePeriod EnsureOpenPeriodFunc
	Logger       *slog.Logger
}

// NewInvoiceService creates a new invoice service.
func NewInvoiceService(deps InvoiceServiceDeps) *InvoiceService {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	ensure := deps.EnsurePeriod
	if ensure == nil {
		ensure = func(_ context.Context, _ invoice.Tx, _ string, _ time.Time) (string, error) {
			return "00000000-0000-0000-0000-000000000001", nil
		}
	}
	return &InvoiceService{
		invoices:     deps.Invoices,
		creditLimits: deps.CreditLimits,
		db:           deps.DB,
		ensurePeriod: ensure,
		log:          log,
	}
}

// =============================================================================
// Use case: CreateInvoice
// =============================================================================

// CreateInvoice creates a new invoice, atomically enforcing the credit limit.
func (s *InvoiceService) CreateInvoice(ctx context.Context, input invoice.CreateInvoiceInput) (invoice.Invoice, error) {
	if err := s.validateCreateInput(input); err != nil {
		return invoice.Invoice{}, err
	}

	now := time.Now().UTC()
	id := uuid.NewString()
	var out invoice.Invoice

	err := s.db.ExecuteTx(ctx, func(tx invoice.Tx) error {
		limit, err := s.creditLimits.Get(ctx, tx, input.CustomerID)
		if err != nil {
			if errors.Is(err, apperrors.ErrNotFound) {
				s.log.Info("create invoice: no credit limit set, proceeding",
					"customer_id", input.CustomerID)
			} else {
				return fmt.Errorf("get credit limit: %w", err)
			}
		} else {
			projectedUsed := limit.UsedAmount.Add(input.Amount)
			if projectedUsed.Cmp(limit.LimitAmount) > 0 {
				return fmt.Errorf(
					"credit limit exceeded: customer %s, limit %s, used %s, requested %s: %w",
					input.CustomerID, limit.LimitAmount, limit.UsedAmount, input.Amount,
					apperrors.ErrCreditLimitExceeded,
				)
			}
			if err := s.creditLimits.IncrementUsed(ctx, tx, input.CustomerID, input.Amount); err != nil {
				return fmt.Errorf("increment credit used: %w", err)
			}
		}

		periodID, err := s.ensurePeriod(ctx, tx, input.TenantID, now)
		if err != nil {
			return fmt.Errorf("ensure period: %w", err)
		}

		inv := invoice.Invoice{
			ID:          id,
			TenantID:    input.TenantID,
			CustomerID:  input.CustomerID,
			Code:        input.Code,
			Amount:      input.Amount,
			PaidAmount:  money.NewFromMinor(0),
			DueDate:     input.DueDate,
			Status:      invoice.InvoiceStatusOpen,
			IssuedAt:    now,
			PeriodID:    periodID,
			Description: input.Description,
			Metadata:    input.Metadata,
		}
		if err := s.invoices.Create(ctx, tx, inv); err != nil {
			return fmt.Errorf("create invoice: %w", err)
		}

		out = inv
		s.log.Info("invoice created",
			"invoice_id", id,
			"customer_id", input.CustomerID,
			"amount", input.Amount.String(),
			"code", input.Code,
		)
		return nil
	})

	if err != nil {
		return invoice.Invoice{}, err
	}
	return out, nil
}

func (s *InvoiceService) validateCreateInput(input invoice.CreateInvoiceInput) error {
	if input.TenantID == "" {
		return fmt.Errorf("%w: tenant_id required", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(input.CustomerID); err != nil {
		return fmt.Errorf("%w: invalid customer_id", apperrors.ErrInvalidInput)
	}
	if input.Code == "" {
		return fmt.Errorf("%w: code required", apperrors.ErrInvalidInput)
	}
	if !input.Amount.IsPositive() {
		return fmt.Errorf("%w: amount must be positive", apperrors.ErrInvalidInput)
	}
	if input.DueDate.IsZero() {
		return fmt.Errorf("%w: due_date required", apperrors.ErrInvalidInput)
	}
	return nil
}

// =============================================================================
// Use case: RecordPayment
// =============================================================================

// RecordPayment allocates a payment across invoices (FIFO default, or manual).
func (s *InvoiceService) RecordPayment(ctx context.Context, input invoice.PaymentInput) (invoice.PaymentResult, error) {
	if err := s.validatePaymentInput(input); err != nil {
		return invoice.PaymentResult{}, err
	}

	paymentID := uuid.NewString()
	var result invoice.PaymentResult

	err := s.db.ExecuteTx(ctx, func(tx invoice.Tx) error {
		outstanding, err := s.invoices.ListOutstandingByCustomer(ctx, tx, input.CustomerID)
		if err != nil {
			return fmt.Errorf("list outstanding: %w", err)
		}

		allocations, err := s.computeAllocations(ctx, input, outstanding, tx)
		if err != nil {
			return err
		}

		var allocatedSum money.Money
		for _, alloc := range allocations {
			if err := s.invoices.InsertAllocation(ctx, tx, paymentID, alloc, input.Method, input.CustomerID); err != nil {
				return fmt.Errorf("insert allocation: %w", err)
			}
			allocatedSum = allocatedSum.Add(alloc.Amount)
		}

		if err := s.creditLimits.DecrementUsed(ctx, tx, input.CustomerID, allocatedSum); err != nil {
			if !errors.Is(err, apperrors.ErrNotFound) {
				return fmt.Errorf("decrement credit used: %w", err)
			}
		}

		result = invoice.PaymentResult{
			PaymentID:   paymentID,
			Allocations: allocations,
			Method:      input.Method,
			CustomerID:  input.CustomerID,
			TotalMinor:  allocatedSum.Minor(),
		}

		s.log.Info("payment recorded",
			"payment_id", paymentID,
			"customer_id", input.CustomerID,
			"amount", input.Amount.String(),
			"allocations", len(allocations),
			"method", string(input.Method),
			"mode", string(input.Mode),
		)
		return nil
	})

	if err != nil {
		return invoice.PaymentResult{}, err
	}
	return result, nil
}

func (s *InvoiceService) validatePaymentInput(input invoice.PaymentInput) error {
	if input.TenantID == "" {
		return fmt.Errorf("%w: tenant_id required", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(input.CustomerID); err != nil {
		return fmt.Errorf("%w: invalid customer_id", apperrors.ErrInvalidInput)
	}
	if !input.Amount.IsPositive() {
		return fmt.Errorf("%w: amount must be positive", apperrors.ErrInvalidInput)
	}
	if input.Method == "" {
		input.Method = invoice.MethodCash
	}
	if input.Mode == "" {
		input.Mode = invoice.AllocationFIFO
	}

	if input.Mode == invoice.AllocationManual {
		if len(input.Allocations) == 0 {
			return fmt.Errorf("%w: manual mode requires allocations", apperrors.ErrInvalidInput)
		}
		var sum money.Money
		for _, a := range input.Allocations {
			if _, err := uuid.Parse(a.InvoiceID); err != nil {
				return fmt.Errorf("%w: invalid invoice_id in allocations", apperrors.ErrInvalidInput)
			}
			if !a.Amount.IsPositive() {
				return fmt.Errorf("%w: allocation amount must be positive", apperrors.ErrInvalidInput)
			}
			sum = sum.Add(a.Amount)
		}
		if sum.Cmp(input.Amount) != 0 {
			return fmt.Errorf(
				"%w: allocations sum %s != payment amount %s",
				apperrors.ErrPaymentAllocationMismatch, sum, input.Amount,
			)
		}
	}
	return nil
}

func (s *InvoiceService) computeAllocations(
	_ context.Context, input invoice.PaymentInput,
	outstanding []invoice.Invoice, _ invoice.Tx,
) ([]invoice.Allocation, error) {
	switch input.Mode {
	case invoice.AllocationManual:
		for _, a := range input.Allocations {
			inv := findInvoice(outstanding, a.InvoiceID)
			if inv == nil {
				return nil, fmt.Errorf(
					"%w: invoice %s not outstanding for customer %s",
					apperrors.ErrInvoiceNotFound, a.InvoiceID, input.CustomerID,
				)
			}
			if a.Amount.Cmp(inv.Outstanding()) > 0 {
				return nil, fmt.Errorf(
					"%w: allocation %s exceeds outstanding %s on invoice %s",
					apperrors.ErrInvoiceOverpaid, a.Amount, inv.Outstanding(), a.InvoiceID,
				)
			}
		}
		return input.Allocations, nil

	case invoice.AllocationFIFO, "":
		var remaining money.Money = input.Amount
		var allocations []invoice.Allocation
		for _, inv := range outstanding {
			if remaining.IsZero() {
				break
			}
			outstandingAmt := inv.Outstanding()
			if outstandingAmt.IsZero() {
				continue
			}
			allocate := outstandingAmt
			if remaining.Cmp(outstandingAmt) < 0 {
				allocate = remaining
			}
			allocations = append(allocations, invoice.Allocation{
				InvoiceID: inv.ID,
				Amount:    allocate,
			})
			remaining = remaining.Sub(allocate)
		}
		if remaining.IsPositive() {
			return nil, fmt.Errorf(
				"%w: payment %s exceeds total outstanding by %s",
				apperrors.ErrInvoiceOverpaid, input.Amount, remaining,
			)
		}
		return allocations, nil

	default:
		return nil, fmt.Errorf("%w: unknown allocation mode %q", apperrors.ErrInvalidInput, input.Mode)
	}
}

func findInvoice(invoices []invoice.Invoice, id string) *invoice.Invoice {
	for i := range invoices {
		if invoices[i].ID == id {
			return &invoices[i]
		}
	}
	return nil
}

// =============================================================================
// Use case: queries
// =============================================================================

// GetInvoice returns one invoice by id.
func (s *InvoiceService) GetInvoice(ctx context.Context, id string) (invoice.Invoice, error) {
	return s.invoices.GetByID(ctx, id)
}

// ListInvoices returns invoices matching the filter.
func (s *InvoiceService) ListInvoices(ctx context.Context, filter invoice.InvoiceFilter) ([]invoice.Invoice, error) {
	return s.invoices.List(ctx, filter)
}

// GetAging returns the aging summary for a customer (or tenant-wide if "").
func (s *InvoiceService) GetAging(ctx context.Context, tenantID, customerID string) ([]invoice.AgingSummary, error) {
	return s.invoices.GetAging(ctx, tenantID, customerID)
}

// SetCreditLimit upserts a customer's credit limit.
func (s *InvoiceService) SetCreditLimit(ctx context.Context, limit invoice.CreditLimit) error {
	if limit.TenantID == "" {
		return fmt.Errorf("%w: tenant_id required", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(limit.CustomerID); err != nil {
		return fmt.Errorf("%w: invalid customer_id", apperrors.ErrInvalidInput)
	}
	if limit.LimitAmount.IsNegative() {
		return fmt.Errorf("%w: limit_amount must be non-negative", apperrors.ErrInvalidInput)
	}
	if limit.EffectiveFrom.IsZero() {
		limit.EffectiveFrom = time.Now().UTC()
	}

	return s.db.ExecuteTx(ctx, func(tx invoice.Tx) error {
		return s.creditLimits.Upsert(ctx, tx, limit)
	})
}
