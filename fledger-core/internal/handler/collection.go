// Handlers — REST endpoints for Collection & Route module (Sprint 11).
//
// Routes (mounted under /v1):
//   POST   /routes                           — plan a new route
//   GET    /routes                           — list routes (filter by date / sales_rep / status)
//   GET    /routes/{id}                      — get route details
//   POST   /routes/{id}/start                — flip planned → in_progress
//   POST   /routes/{id}/complete             — flip in_progress → completed (requires all stops closed)
//   POST   /routes/{id}/settle               — submit settlement (sales rep)
//   GET    /routes/{id}/stops                — list stops for a route
//   POST   /routes/{id}/stops                — add manual stop to a planned route
//   GET    /stops/{id}/events                — list collection events for a stop
//   POST   /stops/{id}/visits                — record a collection event
//   POST   /stops/{id}/close                 — mark stop as closed
//   GET    /settlements/{id}                 — get settlement by id
//   POST   /settlements/{id}/decide          — supervisor approves/rejects settlement
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

	"github.com/runut/fmcg-wallet/internal/domain/collection"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// =============================================================================
// Service interface exposed to handlers
// =============================================================================

// CollectionAPI is the use-case surface exposed to handlers.
type CollectionAPI interface {
	PlanRoute(ctx context.Context, in collection.PlanRouteInput) (collection.CollectionRoute, []collection.RouteStop, error)
	StartRoute(ctx context.Context, routeID string) (collection.CollectionRoute, error)
	CompleteRoute(ctx context.Context, routeID string) (collection.CollectionRoute, error)
	SettleRoute(ctx context.Context, in collection.SettleRouteInput) (collection.Settlement, collection.CollectionRoute, error)
	ApproveSettlement(ctx context.Context, in collection.ApproveSettlementInput) (collection.Settlement, error)
	RecordVisit(ctx context.Context, in collection.RecordVisitInput) (collection.CollectionEvent, collection.RouteStop, error)
	CloseStop(ctx context.Context, in collection.CloseStopInput) (collection.RouteStop, error)

	GetRoute(ctx context.Context, id string) (collection.CollectionRoute, error)
	ListStopsByRoute(ctx context.Context, routeID string) ([]collection.RouteStop, error)
	ListEventsByStop(ctx context.Context, stopID string) ([]collection.CollectionEvent, error)
	GetSettlementByRoute(ctx context.Context, routeID string) (collection.Settlement, error)
	ListRoutesBySalesRep(ctx context.Context, tenantID, salesRepID string, limit int) ([]collection.CollectionRoute, error)
	ListRoutesByDate(ctx context.Context, tenantID string, date time.Time) ([]collection.CollectionRoute, error)
}

// =============================================================================
// DTOs
// =============================================================================

// PlanRouteRequest is the JSON body for POST /v1/routes.
type PlanRouteRequest struct {
	TenantID     string    `json:"tenant_id" validate:"required,uuid"`
	SalesRepID   string    `json:"sales_rep_id" validate:"required,uuid"`
	RouteDate    string    `json:"route_date" validate:"required"` // YYYY-MM-DD
	AutoPopulate bool      `json:"auto_populate"`
	CustomerIDs  []string  `json:"customer_ids,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// RecordVisitRequest is the JSON body for POST /v1/stops/{id}/visits.
type RecordVisitRequest struct {
	AmountMinor   int64  `json:"amount_minor" validate:"required,gt=0"`
	PaymentMethod string `json:"payment_method" validate:"required,oneof=cash qris transfer cheque"`
	Reference     string `json:"reference"`
	Notes         string `json:"notes"`
}

// CloseStopRequest is the JSON body for POST /v1/stops/{id}/close.
type CloseStopRequest struct {
	Notes string `json:"notes"`
}

// SettleRouteRequest is the JSON body for POST /v1/routes/{id}/settle.
type SettleRouteRequest struct {
	SettledAmountMinor int64  `json:"settled_amount_minor" validate:"gte=0"`
	Notes              string `json:"notes"`
}

// ApproveSettlementRequest is the JSON body for POST /v1/settlements/{id}/decide.
type ApproveSettlementRequest struct {
	Approve bool   `json:"approve"`
	Notes   string `json:"notes"`
}

// =============================================================================
// Response DTOs
// =============================================================================

// CollectionRouteResponse is the public representation of a route.
type CollectionRouteResponse struct {
	ID                  string     `json:"id"`
	TenantID            string     `json:"tenant_id"`
	SalesRepID          string     `json:"sales_rep_id"`
	RouteDate           string     `json:"route_date"`
	Status              string     `json:"status"`
	TotalPlannedMinor   int64      `json:"total_planned_minor"`
	TotalCollectedMinor int64      `json:"total_collected_minor"`
	CreatedAt           string     `json:"created_at"`
	StartedAt           *string    `json:"started_at,omitempty"`
	CompletedAt         *string    `json:"completed_at,omitempty"`
	SettledAt           *string    `json:"settled_at,omitempty"`
}

// RouteStopResponse is the public representation of a stop.
type RouteStopResponse struct {
	ID                    string  `json:"id"`
	RouteID               string  `json:"route_id"`
	CustomerID            string  `json:"customer_id"`
	Sequence              int     `json:"sequence"`
	PlannedInvoiceIDs     []string `json:"planned_invoice_ids"`
	ActualCollectionMinor int64   `json:"actual_collection_minor"`
	Status                string  `json:"status"`
	VisitedAt             *string `json:"visited_at,omitempty"`
	ClosedAt              *string `json:"closed_at,omitempty"`
	Notes                 string  `json:"notes"`
}

// CollectionEventResponse is the public representation of a collection event.
type CollectionEventResponse struct {
	ID            string `json:"id"`
	StopID        string `json:"stop_id"`
	AmountMinor   int64  `json:"amount_minor"`
	PaymentMethod string `json:"payment_method"`
	Reference     string `json:"reference"`
	CollectedAt   string `json:"collected_at"`
	Notes         string `json:"notes"`
	RecordedBy    string `json:"recorded_by"`
}

// SettlementResponse is the public representation of a settlement.
type SettlementResponse struct {
	ID                  string  `json:"id"`
	RouteID             string  `json:"route_id"`
	ExpectedAmountMinor int64   `json:"expected_amount_minor"`
	SettledAmountMinor  int64   `json:"settled_amount_minor"`
	DiscrepancyMinor    int64   `json:"discrepancy_minor"`
	Status              string  `json:"status"`
	SubmittedAt         *string `json:"submitted_at,omitempty"`
	ApprovedAt          *string `json:"approved_at,omitempty"`
	ApprovedBy          string  `json:"approved_by,omitempty"`
	Notes               string  `json:"notes"`
}

// =============================================================================
// Converters
// =============================================================================

func ToCollectionRouteResponse(r collection.CollectionRoute) CollectionRouteResponse {
	out := CollectionRouteResponse{
		ID:                  r.ID,
		TenantID:            r.TenantID,
		SalesRepID:          r.SalesRepID,
		RouteDate:           r.RouteDate.Format("2006-01-02"),
		Status:              string(r.Status),
		TotalPlannedMinor:   r.TotalPlannedMinor,
		TotalCollectedMinor: r.TotalCollectedMinor,
		CreatedAt:           r.CreatedAt.UTC().Format(time.RFC3339),
	}
	if r.StartedAt != nil {
		s := r.StartedAt.UTC().Format(time.RFC3339)
		out.StartedAt = &s
	}
	if r.CompletedAt != nil {
		s := r.CompletedAt.UTC().Format(time.RFC3339)
		out.CompletedAt = &s
	}
	if r.SettledAt != nil {
		s := r.SettledAt.UTC().Format(time.RFC3339)
		out.SettledAt = &s
	}
	return out
}

func ToRouteStopResponse(s collection.RouteStop) RouteStopResponse {
	invoiceIDs := s.PlannedInvoiceIDs
	if invoiceIDs == nil {
		invoiceIDs = []string{}
	}
	out := RouteStopResponse{
		ID:                    s.ID,
		RouteID:               s.RouteID,
		CustomerID:            s.CustomerID,
		Sequence:              s.Sequence,
		PlannedInvoiceIDs:     invoiceIDs,
		ActualCollectionMinor: s.ActualCollectionMinor,
		Status:                string(s.Status),
		Notes:                 s.Notes,
	}
	if s.VisitedAt != nil {
		t := s.VisitedAt.UTC().Format(time.RFC3339)
		out.VisitedAt = &t
	}
	if s.ClosedAt != nil {
		t := s.ClosedAt.UTC().Format(time.RFC3339)
		out.ClosedAt = &t
	}
	return out
}

func ToCollectionEventResponse(e collection.CollectionEvent) CollectionEventResponse {
	return CollectionEventResponse{
		ID:            e.ID,
		StopID:        e.StopID,
		AmountMinor:   e.AmountMinor,
		PaymentMethod: string(e.PaymentMethod),
		Reference:     e.Reference,
		CollectedAt:   e.CollectedAt.UTC().Format(time.RFC3339),
		Notes:         e.Notes,
		RecordedBy:    e.RecordedBy,
	}
}

func ToSettlementResponse(s collection.Settlement) SettlementResponse {
	out := SettlementResponse{
		ID:                  s.ID,
		RouteID:             s.RouteID,
		ExpectedAmountMinor: s.ExpectedAmountMinor,
		SettledAmountMinor:  s.SettledAmountMinor,
		DiscrepancyMinor:    s.DiscrepancyMinor,
		Status:              string(s.Status),
		ApprovedBy:          s.ApprovedBy,
		Notes:               s.Notes,
	}
	if s.SubmittedAt != nil {
		t := s.SubmittedAt.UTC().Format(time.RFC3339)
		out.SubmittedAt = &t
	}
	if s.ApprovedAt != nil {
		t := s.ApprovedAt.UTC().Format(time.RFC3339)
		out.ApprovedAt = &t
	}
	return out
}

// =============================================================================
// Handlers
// =============================================================================

// PlanRoute handles POST /v1/routes.
func (h *Handlers) PlanRoute(w http.ResponseWriter, r *http.Request) {
	var body PlanRouteRequest
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
	routeDate, perr := time.Parse("2006-01-02", body.RouteDate)
	if perr != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid route_date format (YYYY-MM-DD)")))
		return
	}
	principal := principalIDFromContext(r)
	route, _, err := h.Collections.PlanRoute(r.Context(), collection.PlanRouteInput{
		TenantID:     body.TenantID,
		SalesRepID:   body.SalesRepID,
		RouteDate:    routeDate,
		AutoPopulate: body.AutoPopulate,
		CustomerIDs:  body.CustomerIDs,
		Metadata:     body.Metadata,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	_ = principal // future: use for audit
	httpx.JSON(w, http.StatusCreated, ToCollectionRouteResponse(route))
}

// ListRoutes handles GET /v1/routes?tenant_id=...&date=...&sales_rep_id=...
func (h *Handlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("tenant_id required")))
		return
	}
	if _, err := uuid.Parse(tenantID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid tenant_id")))
		return
	}

	// Optional filters.
	q := r.URL.Query()
	if dateStr := q.Get("date"); dateStr != "" {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid date")))
			return
		}
		out, err := h.Collections.ListRoutesByDate(r.Context(), tenantID, date)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		resp := make([]CollectionRouteResponse, 0, len(out))
		for _, x := range out {
			resp = append(resp, ToCollectionRouteResponse(x))
		}
		httpx.JSON(w, http.StatusOK, resp)
		return
	}
	if salesRepID := q.Get("sales_rep_id"); salesRepID != "" {
		limit := 50
		if l := q.Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		out, err := h.Collections.ListRoutesBySalesRep(r.Context(), tenantID, salesRepID, limit)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		resp := make([]CollectionRouteResponse, 0, len(out))
		for _, x := range out {
			resp = append(resp, ToCollectionRouteResponse(x))
		}
		httpx.JSON(w, http.StatusOK, resp)
		return
	}

	httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("must specify date or sales_rep_id")))
}

// GetRoute handles GET /v1/routes/{id}.
func (h *Handlers) GetRoute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid route id")))
		return
	}
	route, err := h.Collections.GetRoute(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToCollectionRouteResponse(route))
}

// StartRoute handles POST /v1/routes/{id}/start.
func (h *Handlers) StartRoute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid route id")))
		return
	}
	route, err := h.Collections.StartRoute(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToCollectionRouteResponse(route))
}

// CompleteRoute handles POST /v1/routes/{id}/complete.
func (h *Handlers) CompleteRoute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid route id")))
		return
	}
	route, err := h.Collections.CompleteRoute(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToCollectionRouteResponse(route))
}

// SettleRoute handles POST /v1/routes/{id}/settle.
func (h *Handlers) SettleRoute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid route id")))
		return
	}
	var body SettleRouteRequest
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
	principal := principalIDFromContext(r)
	settlement, _, err := h.Collections.SettleRoute(r.Context(), collection.SettleRouteInput{
		RouteID:             id,
		SettledAmountMinor:  body.SettledAmountMinor,
		RecordedBy:          principal,
		Notes:               body.Notes,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ToSettlementResponse(settlement))
}

// GetSettlement handles GET /v1/settlements/{id}.
func (h *Handlers) GetSettlement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid settlement id")))
		return
	}
	// We have GetSettlementByRoute; expose it via URL pattern /settlements/{id} by route lookup.
	// Simplest: fetch all routes (or use repository directly). For now, expose via /routes/{id}/settlement.
	httpx.Error(w, r, errors.New("not implemented — use GET /v1/routes/{id}/settlement"))
}

// DecideSettlement handles POST /v1/settlements/{id}/decide.
func (h *Handlers) DecideSettlement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid settlement id")))
		return
	}
	var body ApproveSettlementRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	principal := principalIDFromContext(r)
	settlement, err := h.Collections.ApproveSettlement(r.Context(), collection.ApproveSettlementInput{
		SettlementID: id,
		ApproverID:   principal,
		Approve:      body.Approve,
		Notes:        body.Notes,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToSettlementResponse(settlement))
}

// ListStops handles GET /v1/routes/{id}/stops.
func (h *Handlers) ListStops(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid route id")))
		return
	}
	stops, err := h.Collections.ListStopsByRoute(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	resp := make([]RouteStopResponse, 0, len(stops))
	for _, s := range stops {
		resp = append(resp, ToRouteStopResponse(s))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// ListStopEvents handles GET /v1/stops/{id}/events.
func (h *Handlers) ListStopEvents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid stop id")))
		return
	}
	events, err := h.Collections.ListEventsByStop(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	resp := make([]CollectionEventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, ToCollectionEventResponse(e))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// RecordStopVisit handles POST /v1/stops/{id}/visits.
func (h *Handlers) RecordStopVisit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid stop id")))
		return
	}
	var body RecordVisitRequest
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
	principal := principalIDFromContext(r)
	event, stop, err := h.Collections.RecordVisit(r.Context(), collection.RecordVisitInput{
		StopID:        id,
		AmountMinor:   body.AmountMinor,
		PaymentMethod: collection.PaymentMethod(body.PaymentMethod),
		Reference:     body.Reference,
		RecordedBy:    principal,
		Notes:         body.Notes,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"event": ToCollectionEventResponse(event),
		"stop":  ToRouteStopResponse(stop),
	})
}

// CloseStop handles POST /v1/stops/{id}/close.
func (h *Handlers) CloseStopHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid stop id")))
		return
	}
	var body CloseStopRequest
	_ = json.NewDecoder(r.Body).Decode(&body) // body is optional
	stop, err := h.Collections.CloseStop(r.Context(), collection.CloseStopInput{
		StopID: id,
		Notes:  body.Notes,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToRouteStopResponse(stop))
}
