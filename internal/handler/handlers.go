// Handlers — REST endpoints for the ledger.
//
// Routes (mounted under /v1):
//   POST   /accounts                  — create account
//   GET    /accounts/:id              — fetch one account
//   GET    /accounts                  — list (with filters)
//   GET    /accounts/:id/entries      — list ledger entries for an account
//   POST   /transfers                 — create transfer (idempotent)
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// =============================================================================
// Service interfaces
// =============================================================================

type TransferAPI interface {
	Transfer(ctx context.Context, input ledger.TransferInput) (ledger.Transaction, error)
}

type AccountAPI interface {
	Create(ctx context.Context, account ledger.Account) (ledger.Account, error)
	GetByID(ctx context.Context, id string) (ledger.Account, error)
	List(ctx context.Context, filter ledger.AccountFilter) ([]ledger.Account, error)
	ListEntries(ctx context.Context, accountID string, limit int) ([]ledger.Entry, error)
}

// =============================================================================
// Handlers struct
// =============================================================================

type Handlers struct {
	Transfers    TransferAPI
	Accounts     AccountAPI
	Invoices     InvoiceAPI
	Periods      PeriodAPI
	Reconcilers  ReconcilerAPI
	Collections  CollectionAPI
	Currencies   CurrencyAPI // Sprint 12 — multi-currency
	Auth         AuthAPI    // Sprint 13 — login/refresh/MFA
	Validator    *validator.Validate
}

// New constructs the handlers. All API deps may be nil — those routes simply
// won't be mounted in that case (useful for unit-test scaffolding).
func New(
	transfers TransferAPI,
	accounts AccountAPI,
	invoices InvoiceAPI,
	periods PeriodAPI,
	reconcilers ReconcilerAPI,
	collections CollectionAPI,
	currencies CurrencyAPI,
	auth AuthAPI,
) *Handlers {
	return &Handlers{
		Transfers:   transfers,
		Accounts:    accounts,
		Invoices:    invoices,
		Periods:     periods,
		Reconcilers: reconcilers,
		Collections: collections,
		Currencies:  currencies,
		Auth:        auth,
		Validator:   validator.New(),
	}
}

// RegisterRoutes mounts all routes on the given router.
// Caller is responsible for prefixing (typically under /v1).
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Post("/accounts", h.CreateAccount)
	r.Get("/accounts", h.ListAccounts)
	r.Get("/accounts/{id}", h.GetAccount)
	r.Get("/accounts/{id}/entries", h.ListAccountEntries)
	r.Post("/transfers", h.CreateTransfer)

	if h.Invoices != nil {
		r.Post("/invoices", h.CreateInvoice)
		r.Get("/invoices/{id}", h.GetInvoice)
		r.Get("/invoices", h.ListInvoices)
		r.Post("/customers/{id}/payments", h.RecordPayment)
		r.Get("/customers/{id}/aging", h.GetCustomerAging)
		r.Post("/customers/{id}/credit-limit", h.SetCreditLimit)
	}

	if h.Periods != nil {
		r.Post("/periods/{id}/close-requests", h.RequestPeriodClose)
		r.Get("/close-requests/{id}", h.GetCloseRequest)
		r.Post("/close-requests/{id}/approve", h.ApproveCloseRequest)
		r.Post("/close-requests/{id}/reject", h.RejectCloseRequest)
		r.Post("/periods/{id}/reopen", h.ReopenPeriod)
		r.Get("/periods/{id}/snapshots", h.ListSnapshots)
	}

	if h.Reconcilers != nil {
		r.Post("/reconciler/run", h.RunReconciliation)
		r.Get("/reconciler/runs", h.ListReconcilerRuns)
		r.Get("/reconciler/runs/{id}", h.GetReconcilerRun)
		r.Get("/reconciler/runs/{id}/accounts", h.GetReconcilerRunAccounts)
	}

	if h.Collections != nil {
		r.Post("/routes", h.PlanRoute)
		r.Get("/routes", h.ListRoutes)
		r.Get("/routes/{id}", h.GetRoute)
		r.Post("/routes/{id}/start", h.StartRoute)
		r.Post("/routes/{id}/complete", h.CompleteRoute)
		r.Post("/routes/{id}/settle", h.SettleRoute)
		r.Get("/routes/{id}/stops", h.ListStops)
		r.Post("/stops/{id}/visits", h.RecordStopVisit)
		r.Post("/stops/{id}/close", h.CloseStopHandler)
		r.Get("/stops/{id}/events", h.ListStopEvents)
		r.Post("/settlements/{id}/decide", h.DecideSettlement)
	}

	if h.Currencies != nil {
		// Currencies
		r.Get("/currencies", h.ListCurrencies)
		r.Get("/currencies/{code}", h.GetCurrency)
		r.Post("/currencies", h.CreateCurrencyHandler)
		r.Patch("/currencies/{code}", h.UpdateCurrencyHandler)
		r.Post("/currencies/convert", h.ConvertCurrencyHandler)

		// FX rates
		r.Get("/fx-rates", h.ListFxRates)
		r.Get("/fx-rates/latest", h.GetLatestFxRateHandler)
		r.Get("/fx-rates/{id}", h.GetFxRate)
		r.Post("/fx-rates", h.CreateFxRateHandler)
	}

	if h.Auth != nil {
		// Sprint 13 — auth (PUBLIC routes, no auth middleware required).
		// /mfa/setup and /mfa/verify require access token — caller passes via Bearer.
		r.Post("/auth/login", h.Login)
		r.Post("/auth/refresh", h.Refresh)
		r.Post("/auth/logout", h.Logout)
		r.Post("/auth/mfa/setup", h.SetupMFA)
		r.Post("/auth/mfa/verify", h.VerifyMFA)
	}
}

// =============================================================================
// Account handlers
// =============================================================================

func (h *Handlers) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{
			"validation": err.Error(),
		})
		return
	}

	tenantID := uuid.NewString()
	account := ledger.Account{
		ID:            uuid.NewString(),
		Code:          req.Code,
		Name:          req.Name,
		Type:          ledger.AccountType(req.Type),
		Status:        ledger.AccountStatusActive,
		Currency:      req.Currency,
		CachedBalance: 0,
		OwnerID:       req.OwnerID,
		TenantID:      tenantID,
		Metadata:      req.Metadata,
	}

	created, err := h.Accounts.Create(r.Context(), account)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.JSON(w, http.StatusCreated, ToAccountResponse(created))
}

func (h *Handlers) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid account id")))
		return
	}

	account, err := h.Accounts.GetByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.JSON(w, http.StatusOK, ToAccountResponse(account))
}

func (h *Handlers) ListAccounts(w http.ResponseWriter, r *http.Request) {
	filter := ledger.AccountFilter{
		Type:   ledger.AccountType(r.URL.Query().Get("type")),
		Status: ledger.AccountStatus(r.URL.Query().Get("status")),
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			filter.Limit = n
		}
	}

	accounts, err := h.Accounts.List(r.Context(), filter)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	out := make([]AccountResponse, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, ToAccountResponse(a))
	}
	httpx.JSON(w, http.StatusOK, out)
}

// ListAccountEntries handles GET /v1/accounts/{id}/entries?limit=50.
func (h *Handlers) ListAccountEntries(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid account id")))
		return
	}

	if _, err := h.Accounts.GetByID(r.Context(), id); err != nil {
		httpx.Error(w, r, err)
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	entries, err := h.Accounts.ListEntries(r.Context(), id, limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	out := make([]EntryDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, ToEntryDTO(e))
	}
	httpx.JSON(w, http.StatusOK, out)
}

// =============================================================================
// Transfer handler
// =============================================================================

func (h *Handlers) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey == "" {
		httpx.Error(w, r, apperrors.ErrIdempotencyKeyMissing)
		return
	}

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	req.IdempotencyKey = idemKey

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

	input := req.ToTransferInput(tenantID)
	result, err := h.Transfers.Transfer(r.Context(), input)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	resp := ToTransferResponse(result)
	for _, e := range result.Entries {
		if e.Type == ledger.EntryTypeDebit {
			resp.FromAccountID = e.AccountID
			resp.AmountMinor = e.Amount.Minor()
			resp.Currency = e.Currency
		} else if e.Type == ledger.EntryTypeCredit {
			resp.ToAccountID = e.AccountID
		}
	}

	httpx.JSON(w, http.StatusCreated, resp)
}
