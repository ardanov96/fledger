// Handlers — REST endpoints for invoices, payments, and aging queries.
//
// Routes (mounted under /v1):
//   POST   /invoices                           — create invoice (credit check enforced)
//   GET    /invoices/:id                       — fetch one invoice
//   GET    /invoices                           — list (filter by customer_id, status)
//   POST   /invoices/:id/payments              — record payment for one invoice (allocates FIFO or manual)
//   GET    /customers/:id/aging                — aging summary for a customer
//   POST   /customers/:id/credit-limit         — set credit limit
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/invoice"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// InvoiceAPI is the use-case interface exposed to handlers.
//
// Kept narrow so we don't pull usecase into handler deps.
type InvoiceAPI interface {
	CreateInvoice(ctx context.Context, input invoice.CreateInvoiceInput) (invoice.Invoice, error)
	GetInvoice(ctx context.Context, id string) (invoice.Invoice, error)
	ListInvoices(ctx context.Context, filter invoice.InvoiceFilter) ([]invoice.Invoice, error)
	RecordPayment(ctx context.Context, input invoice.PaymentInput) (invoice.PaymentResult, error)
	GetAging(ctx context.Context, tenantID, customerID string) ([]invoice.AgingSummary, error)
	SetCreditLimit(ctx context.Context, limit invoice.CreditLimit) error
}

// (Invoice handler methods live on *Handlers — extending the existing struct.)

// CreateInvoice handles POST /v1/invoices.
func (h *Handlers) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	var req CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{
			"validation": err.Error(),
		})
		return
	}

	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "00000000-0000-0000-0000-000000000001"
	}

	input := req.ToCreateInvoiceInput(tenantID)
	inv, err := h.Invoices.CreateInvoice(r.Context(), input)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ToInvoiceResponse(inv))
}

// GetInvoice handles GET /v1/invoices/:id.
func (h *Handlers) GetInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid invoice id")))
		return
	}

	inv, err := h.Invoices.GetInvoice(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToInvoiceResponse(inv))
}

// ListInvoices handles GET /v1/invoices?customer_id=&status=&limit=.
func (h *Handlers) ListInvoices(w http.ResponseWriter, r *http.Request) {
	filter := invoice.InvoiceFilter{
		CustomerID: r.URL.Query().Get("customer_id"),
		Status:     invoice.InvoiceStatus(r.URL.Query().Get("status")),
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		filter.Cursor = cursor
	}

	invs, err := h.Invoices.ListInvoices(r.Context(), filter)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	out := make([]InvoiceResponse, 0, len(invs))
	for _, i := range invs {
		out = append(out, ToInvoiceResponse(i))
	}
	httpx.JSON(w, http.StatusOK, out)
}

// RecordPayment handles POST /v1/customers/{id}/payments.
//
// Use X-Allocation-Mode header to switch between FIFO (default) and manual.
func (h *Handlers) RecordPayment(w http.ResponseWriter, r *http.Request) {
	var req RecordPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{
			"validation": err.Error(),
		})
		return
	}

	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "00000000-0000-0000-0000-000000000001"
	}

	mode := invoice.AllocationMode(r.Header.Get("X-Allocation-Mode"))
	if mode == "" {
		mode = invoice.AllocationFIFO
	}

	customerID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(customerID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid customer id")))
		return
	}

	var allocs []invoice.Allocation
	if mode == invoice.AllocationManual {
		allocs = make([]invoice.Allocation, len(req.Allocations))
		for i, a := range req.Allocations {
			allocs[i] = invoice.Allocation{
				InvoiceID: a.InvoiceID,
				Amount:    money.NewFromMinor(a.AmountMinor),
			}
		}
	}

	input := invoice.PaymentInput{
		TenantID:    tenantID,
		CustomerID:  customerID,
		Amount:      money.NewFromMinor(req.AmountMinor),
		Method:      invoice.PaymentMethod(req.Method),
		Mode:        mode,
		Allocations: allocs,
		Description: req.Description,
	}

	result, err := h.Invoices.RecordPayment(r.Context(), input)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.JSON(w, http.StatusCreated, ToPaymentResultResponse(result))
}

// GetCustomerAging handles GET /v1/customers/:id/aging.
func (h *Handlers) GetCustomerAging(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(customerID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid customer id")))
		return
	}

	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "00000000-0000-0000-0000-000000000001"
	}

	summaries, err := h.Invoices.GetAging(r.Context(), tenantID, customerID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	out := make([]AgingSummaryResponse, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, ToAgingSummaryResponse(s))
	}
	httpx.JSON(w, http.StatusOK, out)
}

// SetCreditLimit handles POST /v1/customers/:id/credit-limit.
func (h *Handlers) SetCreditLimit(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(customerID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid customer id")))
		return
	}

	var req SetCreditLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{
			"validation": err.Error(),
		})
		return
	}

	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "00000000-0000-0000-0000-000000000001"
	}

	limit := invoice.CreditLimit{
		TenantID:    tenantID,
		CustomerID:  customerID,
		LimitAmount: money.NewFromMinor(req.LimitAmountMinor),
	}

	if err := h.Invoices.SetCreditLimit(r.Context(), limit); err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"customer_id":    customerID,
		"limit_minor":    req.LimitAmountMinor,
		"effective_from": limit.EffectiveFrom,
	})
}
