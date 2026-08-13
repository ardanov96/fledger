// Handlers — REST endpoints for reconciler (Fase 1B).
//
// Routes (mounted under /v1):
//   POST /reconciler/run              — manual trigger
//   GET  /reconciler/runs             — list recent runs (dashboard)
//   GET  /reconciler/runs/{id}        — get one run (with details)
//   GET  /reconciler/runs/{id}/accounts — per-account breakdown
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

	"github.com/runut/fmcg-wallet/internal/domain/reconciler"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// =============================================================================
// ReconcilerAPI — narrow interface exposed to handlers
// =============================================================================

// ReconcilerAPI is the use-case surface exposed to handlers.
type ReconcilerAPI interface {
	RunReconciliation(ctx context.Context, in ReconcilerRunInput) (ReconcilerRunResult, error)
	GetRun(ctx context.Context, id string) (reconciler.ReconcilerRun, error)
	ListRunsByTenant(ctx context.Context, tenantID string, limit int) ([]reconciler.ReconcilerRun, error)
	ListAccountResultsByRun(ctx context.Context, runID string) ([]reconciler.ReconcilerAccountResult, error)
}

// ReconcilerRunInput mirrors usecase.RunReconciliationInput.
type ReconcilerRunInput struct {
	TenantID     string
	PeriodID     string
	TriggeredBy  reconciler.TriggerSource
	RunHashCheck bool
}

// ReconcilerRunResult mirrors usecase.RunReconciliationResult.
type ReconcilerRunResult struct {
	Run             reconciler.ReconcilerRun
	AccountResults  []reconciler.ReconcilerAccountResult
	HashChainErrors int
}

// =============================================================================
// DTOs
// =============================================================================

// ReconcilerRunResponse is the public representation of a reconciler run.
type ReconcilerRunResponse struct {
	ID               string  `json:"id"`
	TenantID         string  `json:"tenant_id"`
	PeriodID         string  `json:"period_id"`
	StartedAt        string  `json:"started_at"`
	FinishedAt       *string `json:"finished_at,omitempty"`
	Status           string  `json:"status"`
	TotalDebitMinor  int64   `json:"total_debit_minor"`
	TotalCreditMinor int64   `json:"total_credit_minor"`
	ImbalanceMinor   int64   `json:"imbalance_minor"`
	HashChainOK      *bool   `json:"hash_chain_ok,omitempty"`
	HashChainErrors  int     `json:"hash_chain_errors"`
	TriggeredBy      string  `json:"triggered_by"`
}

// ReconcilerAccountResultResponse is the public per-account breakdown row.
type ReconcilerAccountResultResponse struct {
	ID            string `json:"id"`
	PeriodID      string `json:"period_id"`
	AccountID     string `json:"account_id"`
	DebitMinor    int64  `json:"debit_minor"`
	CreditMinor   int64  `json:"credit_minor"`
	SignedBalance int64  `json:"signed_balance"`
	EntryCount    int    `json:"entry_count"`
	Currency      string `json:"currency"`
}

// =============================================================================
// Converters
// =============================================================================

func ToReconcilerRunResponse(r reconciler.ReconcilerRun) ReconcilerRunResponse {
	out := ReconcilerRunResponse{
		ID:               r.ID,
		TenantID:         r.TenantID,
		PeriodID:         r.PeriodID,
		StartedAt:        r.StartedAt.UTC().Format(time.RFC3339),
		Status:           string(r.Status),
		TotalDebitMinor:  r.TotalDebit.Minor(),
		TotalCreditMinor: r.TotalCredit.Minor(),
		ImbalanceMinor:   r.Imbalance.Minor(),
		HashChainOK:      r.HashChainOK,
		HashChainErrors:  r.HashChainErrors,
		TriggeredBy:      string(r.TriggeredBy),
	}
	if r.FinishedAt != nil {
		s := r.FinishedAt.UTC().Format(time.RFC3339)
		out.FinishedAt = &s
	}
	return out
}

func ToReconcilerAccountResultResponse(r reconciler.ReconcilerAccountResult) ReconcilerAccountResultResponse {
	return ReconcilerAccountResultResponse{
		ID:            r.ID,
		PeriodID:      r.PeriodID,
		AccountID:     r.AccountID,
		DebitMinor:    r.DebitMinor,
		CreditMinor:   r.CreditMinor,
		SignedBalance: r.SignedBalance,
		EntryCount:    r.EntryCount,
		Currency:      r.Currency,
	}
}

// =============================================================================
// Handlers
// =============================================================================

// RunReconciliation handles POST /v1/reconciler/run.
func (h *Handlers) RunReconciliation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TenantID     string `json:"tenant_id" validate:"required,uuid"`
		PeriodID     string `json:"period_id" validate:"omitempty,uuid"`
		RunHashCheck bool   `json:"run_hash_check"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if h.Validator != nil {
		if err := h.Validator.Struct(&body); err != nil {
			httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{
				"validation": err.Error(),
			})
			return
		}
	}

	res, err := h.Reconcilers.RunReconciliation(r.Context(), ReconcilerRunInput{
		TenantID:     body.TenantID,
		PeriodID:     body.PeriodID,
		TriggeredBy:  reconciler.TriggerAPI,
		RunHashCheck: body.RunHashCheck,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToReconcilerRunResponse(res.Run))
}

// GetReconcilerRun handles GET /v1/reconciler/runs/{id}.
func (h *Handlers) GetReconcilerRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid run id")))
		return
	}
	out, err := h.Reconcilers.GetRun(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToReconcilerRunResponse(out))
}

// ListReconcilerRuns handles GET /v1/reconciler/runs?tenant_id=...&limit=...
func (h *Handlers) ListReconcilerRuns(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("tenant_id required")))
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	out, err := h.Reconcilers.ListRunsByTenant(r.Context(), tenantID, limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	resp := make([]ReconcilerRunResponse, 0, len(out))
	for _, r := range out {
		resp = append(resp, ToReconcilerRunResponse(r))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// GetReconcilerRunAccounts handles GET /v1/reconciler/runs/{id}/accounts.
func (h *Handlers) GetReconcilerRunAccounts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid run id")))
		return
	}
	results, err := h.Reconcilers.ListAccountResultsByRun(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	resp := make([]ReconcilerAccountResultResponse, 0, len(results))
	for _, x := range results {
		resp = append(resp, ToReconcilerAccountResultResponse(x))
	}
	httpx.JSON(w, http.StatusOK, resp)
}
