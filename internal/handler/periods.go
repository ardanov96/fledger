// Handlers — REST endpoints for period-close workflow (Fase 1A).
//
// Routes (mounted under /v1):
//   POST   /periods/{id}/close-requests     — request a close (status → closing)
//   GET    /close-requests/{id}              — fetch one request (audit detail)
//   POST   /close-requests/{id}/approve      — approve (compute trial balance + snapshots + status → closed)
//   POST   /close-requests/{id}/reject       — reject with reason (status → open)
//   POST   /periods/{id}/reopen              — admin only, reopen a closed period
//   GET    /periods/{id}/snapshots           — list frozen snapshots for a period
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"

	"github.com/runut/fmcg-wallet/internal/domain/period"
)

// =============================================================================
// Input types — exported, used by handler AND by adapter in main.go
// =============================================================================

// PeriodRequestCloseInput is the input for requesting a period close.
type PeriodRequestCloseInput struct {
	TenantID    string
	PeriodID    string
	RequesterID string
	Metadata    map[string]any
}

// PeriodApproveCloseInput is the input for approving a close request.
type PeriodApproveCloseInput struct {
	RequestID  string
	ApproverID string
}

// PeriodRejectCloseInput is the input for rejecting a close request.
type PeriodRejectCloseInput struct {
	RequestID       string
	ApproverID      string
	RejectionReason string
}

// PeriodReopenInput is the input for reopening a closed period.
type PeriodReopenInput struct {
	PeriodID string
	AdminID  string
	Reason   string
}

// =============================================================================
// Service interface — narrow; adapter in main.go wraps usecase.PeriodService
// =============================================================================

// PeriodAPI is the use-case surface exposed to handlers.
type PeriodAPI interface {
	RequestClose(ctx context.Context, in PeriodRequestCloseInput) (period.CloseRequest, error)
	ApproveClose(ctx context.Context, in PeriodApproveCloseInput) (period.CloseRequest, error)
	RejectClose(ctx context.Context, in PeriodRejectCloseInput) (period.CloseRequest, error)
	Reopen(ctx context.Context, in PeriodReopenInput) (period.Period, error)
	GetRequest(ctx context.Context, id string) (period.CloseRequest, error)
	ListSnapshotsByPeriod(ctx context.Context, periodID string) ([]period.PeriodSnapshot, error)
}

// =============================================================================
// Handler methods (mounted on *Handlers)
// =============================================================================

// RequestPeriodClose handles POST /v1/periods/{id}/close-requests.
func (h *Handlers) RequestPeriodClose(w http.ResponseWriter, r *http.Request) {
	periodID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(periodID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid period id")))
		return
	}

	var body struct {
		Metadata map[string]any `json:"metadata,omitempty"`
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
			return
		}
	}

	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "00000000-0000-0000-0000-000000000001"
	}

	requesterID := principalIDFromContext(r)
	if requesterID == "" {
		requesterID = uuid.NewString()
	}

	out, err := h.Periods.RequestClose(r.Context(), PeriodRequestCloseInput{
		TenantID:    tenantID,
		PeriodID:    periodID,
		RequesterID: requesterID,
		Metadata:    body.Metadata,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ToCloseRequestResponse(out))
}

// ApproveCloseRequest handles POST /v1/close-requests/{id}/approve.
func (h *Handlers) ApproveCloseRequest(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(requestID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid request id")))
		return
	}
	approverID := principalIDFromContext(r)
	if approverID == "" {
		approverID = uuid.NewString()
	}

	out, err := h.Periods.ApproveClose(r.Context(), PeriodApproveCloseInput{
		RequestID:  requestID,
		ApproverID: approverID,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToCloseRequestResponse(out))
}

// RejectCloseRequest handles POST /v1/close-requests/{id}/reject.
func (h *Handlers) RejectCloseRequest(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(requestID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid request id")))
		return
	}

	var body struct {
		RejectionReason string `json:"rejection_reason" validate:"required,min=1,max=500"`
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

	approverID := principalIDFromContext(r)
	if approverID == "" {
		approverID = uuid.NewString()
	}

	out, err := h.Periods.RejectClose(r.Context(), PeriodRejectCloseInput{
		RequestID:       requestID,
		ApproverID:      approverID,
		RejectionReason: body.RejectionReason,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToCloseRequestResponse(out))
}

// ReopenPeriod handles POST /v1/periods/{id}/reopen.
func (h *Handlers) ReopenPeriod(w http.ResponseWriter, r *http.Request) {
	periodID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(periodID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid period id")))
		return
	}

	var body struct {
		Reason string `json:"reason" validate:"required,min=1,max=500"`
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

	adminID := principalIDFromContext(r)
	if adminID == "" {
		adminID = uuid.NewString()
	}

	out, err := h.Periods.Reopen(r.Context(), PeriodReopenInput{
		PeriodID: periodID,
		AdminID:  adminID,
		Reason:   body.Reason,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToPeriodResponse(out))
}

// GetCloseRequest handles GET /v1/close-requests/{id}.
func (h *Handlers) GetCloseRequest(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(requestID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid request id")))
		return
	}
	out, err := h.Periods.GetRequest(r.Context(), requestID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToCloseRequestResponse(out))
}

// ListSnapshots handles GET /v1/periods/{id}/snapshots.
func (h *Handlers) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	periodID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(periodID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid period id")))
		return
	}
	snaps, err := h.Periods.ListSnapshotsByPeriod(r.Context(), periodID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	out := make([]PeriodSnapshotResponse, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, ToPeriodSnapshotResponse(s))
	}
	httpx.JSON(w, http.StatusOK, out)
}
