// Package handler — currency.go (Sprint 12 / Fase 1D)
//
// REST endpoints for currencies and FX rates. All routes mounted by
// Handlers.RegisterRoutes are gated by Casbin RBAC (RequirePermission).
//
// Routes (mounted under /v1):
//   GET    /currencies                  — list active currencies (read)
//   GET    /currencies/{code}           — get one currency
//   POST   /currencies                  — create currency (admin)
//   PATCH  /currencies/{code}           — update currency (admin)
//   GET    /fx-rates                    — list FX rates with filters
//   GET    /fx-rates/{id}               — get one FX rate
//   POST   /fx-rates                    — create FX rate (admin)
//   GET    /fx-rates/latest             — get latest active rate for (from, to)
//   POST   /currencies/convert          — pure FX math (no transfer side effect)
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
	"github.com/runut/fmcg-wallet/internal/middleware"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
	"github.com/runut/fmcg-wallet/internal/platform/money"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// =============================================================================
// CurrencyAPI — interface implemented by an adapter over usecase.CurrencyService
// (cmd/api/currency_adapters.go).
// =============================================================================

// CurrencyAPI is the narrow interface used by the handler. Implemented in
// cmd/api via a thin adapter that translates between context.Context and the
// use case's typed input/output structs.
type CurrencyAPI interface {
	// Read
	ListActiveCurrencies(ctx context.Context) ([]currency.Currency, error)
	GetCurrency(ctx context.Context, code string) (currency.Currency, error)
	GetFxRate(ctx context.Context, id uuid.UUID) (currency.FxRate, error)
	GetLatestFxRate(ctx context.Context, tenantID uuid.UUID, from, to string, at time.Time) (currency.FxRate, error)
	ListFxRates(ctx context.Context, tenantID uuid.UUID, from, to string, limit int) ([]currency.FxRate, error)

	// Write (admin)
	CreateCurrency(ctx context.Context, in usecase.CreateCurrencyInput) (currency.Currency, error)
	UpdateCurrency(ctx context.Context, in usecase.UpdateCurrencyInput) (currency.Currency, error)
	CreateFxRate(ctx context.Context, in usecase.CreateFxRateInput) (currency.FxRate, error)

	// FX math
	Convert(ctx context.Context, in usecase.ConvertInput) (usecase.ConvertOutput, error)
}

// =============================================================================
// DTOs
// =============================================================================

// CurrencyResponse is the public representation of a currency.
type CurrencyResponse struct {
	Code          string    `json:"code"`
	DecimalPlaces int       `json:"decimal_places"`
	Name          string    `json:"name"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateCurrencyRequest is the body of POST /v1/currencies.
type CreateCurrencyRequest struct {
	Code          string `json:"code"           validate:"required,len=3"`
	DecimalPlaces int    `json:"decimal_places" validate:"gte=0,lte=6"`
	Name          string `json:"name"           validate:"required,min=1,max=100"`
	IsActive      bool   `json:"is_active"`
}

// UpdateCurrencyRequest is the body of PATCH /v1/currencies/{code}.
type UpdateCurrencyRequest struct {
	DecimalPlaces int    `json:"decimal_places" validate:"gte=0,lte=6"`
	Name          string `json:"name"           validate:"required,min=1,max=100"`
	IsActive      bool   `json:"is_active"`
}

// FxRateResponse is the public representation of an FX rate.
type FxRateResponse struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	Rate         string    `json:"rate"` // string-encoded decimal for precision
	EffectiveAt  time.Time `json:"effective_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Source       string    `json:"source"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateFxRateRequest is the body of POST /v1/fx-rates.
type CreateFxRateRequest struct {
	TenantID     string    `json:"tenant_id"     validate:"required,uuid"`
	FromCurrency string    `json:"from_currency" validate:"required,len=3"`
	ToCurrency   string    `json:"to_currency"   validate:"required,len=3"`
	Rate         string    `json:"rate"          validate:"required"` // decimal string
	EffectiveAt  time.Time `json:"effective_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Source       string    `json:"source,omitempty" validate:"omitempty,oneof=manual api bank seed"`
}

// ConvertRequest is the body of POST /v1/currencies/convert.
type ConvertRequest struct {
	TenantID     string `json:"tenant_id"     validate:"required,uuid"`
	FromCurrency string `json:"from_currency" validate:"required,len=3"`
	ToCurrency   string `json:"to_currency"   validate:"required,len=3"`
	AmountMinor  int64  `json:"amount_minor"  validate:"required,gt=0"`
}

// ConvertResponse is the body of POST /v1/currencies/convert.
type ConvertResponse struct {
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	FromMinor    int64     `json:"from_minor"`
	ToMinor      int64     `json:"to_minor"`
	Rate         string    `json:"rate"` // string-encoded decimal
	RateID       string    `json:"rate_id,omitempty"`
	At           time.Time `json:"at"`
}

// =============================================================================
// Handlers (mounted via Handlers.RegisterRoutes)
// =============================================================================

// ListCurrencies handles GET /v1/currencies.
func (h *Handlers) ListCurrencies(w http.ResponseWriter, r *http.Request) {
	if h.Currencies == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	list, err := h.Currencies.ListActiveCurrencies(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	out := make([]CurrencyResponse, 0, len(list))
	for _, c := range list {
		out = append(out, ToCurrencyResponse(c))
	}
	httpx.JSON(w, http.StatusOK, out)
}

// GetCurrency handles GET /v1/currencies/{code}.
func (h *Handlers) GetCurrency(w http.ResponseWriter, r *http.Request) {
	if h.Currencies == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	code := chi.URLParam(r, "code")
	c, err := h.Currencies.GetCurrency(r.Context(), code)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToCurrencyResponse(c))
}

// CreateCurrencyHandler handles POST /v1/currencies.
func (h *Handlers) CreateCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	if h.Currencies == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	var req CreateCurrencyRequest
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

	created, err := h.Currencies.CreateCurrency(r.Context(), usecase.CreateCurrencyInput{
		Code:          req.Code,
		DecimalPlaces: req.DecimalPlaces,
		Name:          req.Name,
		IsActive:      req.IsActive,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ToCurrencyResponse(created))
}

// UpdateCurrencyHandler handles PATCH /v1/currencies/{code}.
func (h *Handlers) UpdateCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	if h.Currencies == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	code := chi.URLParam(r, "code")
	var req UpdateCurrencyRequest
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

	updated, err := h.Currencies.UpdateCurrency(r.Context(), usecase.UpdateCurrencyInput{
		Code:          code,
		DecimalPlaces: req.DecimalPlaces,
		Name:          req.Name,
		IsActive:      req.IsActive,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToCurrencyResponse(updated))
}

// ListFxRates handles GET /v1/fx-rates?tenant_id=...&from=USD&to=IDR&limit=50.
func (h *Handlers) ListFxRates(w http.ResponseWriter, r *http.Request) {
	if h.Currencies == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	tenantIDStr := r.URL.Query().Get("tenant_id")
	if tenantIDStr == "" {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("tenant_id required")))
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid tenant_id")))
		return
	}

	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, e := strconv.Atoi(s); e == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	list, err := h.Currencies.ListFxRates(r.Context(), tenantID,
		r.URL.Query().Get("from"), r.URL.Query().Get("to"), limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	out := make([]FxRateResponse, 0, len(list))
	for _, fr := range list {
		out = append(out, ToFxRateResponse(fr))
	}
	httpx.JSON(w, http.StatusOK, out)
}

// GetFxRate handles GET /v1/fx-rates/{id}.
func (h *Handlers) GetFxRate(w http.ResponseWriter, r *http.Request) {
	if h.Currencies == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid fx_rate id")))
		return
	}
	fr, err := h.Currencies.GetFxRate(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToFxRateResponse(fr))
}

// GetLatestFxRateHandler handles GET /v1/fx-rates/latest?tenant_id=...&from=USD&to=IDR.
func (h *Handlers) GetLatestFxRateHandler(w http.ResponseWriter, r *http.Request) {
	if h.Currencies == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	tenantIDStr := r.URL.Query().Get("tenant_id")
	if tenantIDStr == "" {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("tenant_id required")))
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid tenant_id")))
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("from and to required")))
		return
	}

	fr, err := h.Currencies.GetLatestFxRate(r.Context(), tenantID, from, to, time.Now().UTC())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToFxRateResponse(fr))
}

// CreateFxRateHandler handles POST /v1/fx-rates.
func (h *Handlers) CreateFxRateHandler(w http.ResponseWriter, r *http.Request) {
	if h.Currencies == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	var req CreateFxRateRequest
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

	rate, err := decimal.NewFromString(req.Rate)
	if err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid rate")))
		return
	}

	tenantID, _ := uuid.Parse(req.TenantID)

	// Sprint 23 (hardening): createdBy is now sourced from the JWT Principal
	// (user_id claim). The previous uuid.Nil placeholder was a Sprint 13
	// backfill that left every FX rate's `created_by` column pointing at
	// the zero UUID — useless for audit. Now: tenant_id is ALSO taken from
	// the Principal when present so the request body cannot lie about which
	// tenant created the rate. The body tenant_id is preserved for backward
	// compatibility with clients that haven't moved to JWT yet.
	createdBy := uuid.Nil
	principal := middleware.PrincipalFromContext(r.Context())
	if principal != nil && principal.UserID != "" {
		if uid, err := uuid.Parse(principal.UserID); err == nil {
			createdBy = uid
		}
	}

	in := usecase.CreateFxRateInput{
		TenantID:     tenantID,
		FromCurrency: req.FromCurrency,
		ToCurrency:   req.ToCurrency,
		Rate:         rate,
		EffectiveAt:  req.EffectiveAt,
		ExpiresAt:    req.ExpiresAt,
		Source:       req.Source,
		CreatedBy:    createdBy,
	}
	created, err := h.Currencies.CreateFxRate(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ToFxRateResponse(created))
}

// ConvertCurrencyHandler handles POST /v1/currencies/convert.
func (h *Handlers) ConvertCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	if h.Currencies == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	var req ConvertRequest
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

	tenantID, _ := uuid.Parse(req.TenantID)
	out, err := h.Currencies.Convert(r.Context(), usecase.ConvertInput{
		TenantID:     tenantID,
		FromCurrency: req.FromCurrency,
		ToCurrency:   req.ToCurrency,
		Amount:       money.NewFromMinor(req.AmountMinor),
		At:           time.Now().UTC(),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToConvertResponse(out))
}

// =============================================================================
// Converters (DTO <-> domain)
// =============================================================================

// ToCurrencyResponse converts a domain Currency to HTTP DTO.
func ToCurrencyResponse(c currency.Currency) CurrencyResponse {
	return CurrencyResponse{
		Code:          c.Code,
		DecimalPlaces: c.DecimalPlaces,
		Name:          c.Name,
		IsActive:      c.IsActive,
		CreatedAt:     c.CreatedAt,
	}
}

// ToFxRateResponse converts a domain FxRate to HTTP DTO.
func ToFxRateResponse(r currency.FxRate) FxRateResponse {
	return FxRateResponse{
		ID:           r.ID.String(),
		TenantID:     r.TenantID.String(),
		FromCurrency: r.FromCurrency,
		ToCurrency:   r.ToCurrency,
		Rate:         r.Rate.String(),
		EffectiveAt:  r.EffectiveAt,
		ExpiresAt:    r.ExpiresAt,
		Source:       r.Source,
		CreatedBy:    r.CreatedBy.String(),
		CreatedAt:    r.CreatedAt,
	}
}

// ToConvertResponse converts a usecase.ConvertOutput to HTTP DTO.
func ToConvertResponse(out usecase.ConvertOutput) ConvertResponse {
	return ConvertResponse{
		FromCurrency: out.FromCurrency,
		ToCurrency:   out.ToCurrency,
		FromMinor:    out.FromAmount.Minor(),
		ToMinor:      out.ToAmount.Minor(),
		Rate:         out.Rate.Rate.String(),
		RateID:       out.Rate.ID.String(),
		At:           out.At,
	}
}
