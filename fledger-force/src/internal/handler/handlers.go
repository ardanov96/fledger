// Package handler exposes REST endpoints for Fledger Force.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/fledger/fledger-force/internal/domain/beatplan"
	"github.com/fledger/fledger-force/internal/domain/salesrep"
	"github.com/fledger/fledger-force/internal/domain/store"
	"github.com/fledger/fledger-force/internal/domain/visit"
	apperrors "github.com/fledger/fledger-force/internal/platform/errors"
	"github.com/fledger/fledger-force/internal/platform/httpx"
	"github.com/fledger/fledger-force/internal/usecase"
)

type Handlers struct {
	Validator *validator.Validate
	Rep        *usecase.RepService
	Store      *usecase.StoreService
	Visit      *usecase.VisitService
	Collection *usecase.CollectionService
	DBPinger   func(ctx context.Context) error
}

func NewHandlers(v *validator.Validate, srv *usecase.Services, pinger func(ctx context.Context) error) *Handlers {
	return &Handlers{
		Validator: v,
		Rep:       srv.Rep,
		Store:     srv.Store,
		Visit:     srv.Visit,
		Collection: srv.Collection,
		DBPinger:  pinger,
	}
}

// Health & readyz
func (h *Handlers) Healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "alive", "service": "fledger-force"})
}

func (h *Handlers) Readyz(w http.ResponseWriter, r *http.Request) {
	if h.DBPinger == nil {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.DBPinger(ctx); err != nil {
		httpx.Error(w, r, apperrors.New(http.StatusServiceUnavailable, "not_ready", "db ping failed").WithDetail(map[string]any{"err": err.Error()}))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// =============================================================================
// Sales rep endpoints
// =============================================================================

type CreateRepRequest struct {
	EmployeeCode          string `json:"employee_code"          validate:"required,min=1,max=64"`
	Name                  string `json:"name"                   validate:"required,min=1,max=255"`
	Phone                 string `json:"phone"                  validate:"required,min=1,max=32"`
	Role                  string `json:"role"                   validate:"omitempty,oneof=CANVASSER MOTORIS SUPERVISOR COLLECTOR"`
	FledgerWalletAccountID string `json:"fledger_wallet_account_id" validate:"omitempty,max=128"`
	MaxCashLimit          int64  `json:"max_cash_limit"         validate:"omitempty,gt=0"`
}

func (h *Handlers) CreateRep(w http.ResponseWriter, r *http.Request) {
	var req CreateRepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	rep, err := h.Rep.Create(r.Context(), usecase.CreateRepInput{
		TenantID:               tenantFrom(r),
		EmployeeCode:           req.EmployeeCode,
		Name:                   req.Name,
		Phone:                  req.Phone,
		Role:                   salesrepRoleOrDefault(req.Role),
		FledgerWalletAccountID: req.FledgerWalletAccountID,
		MaxCashLimit:           req.MaxCashLimit,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rep)
}

func (h *Handlers) ListReps(w http.ResponseWriter, r *http.Request) {
	out, err := h.Rep.List(r.Context(), tenantFrom(r), r.URL.Query().Get("status"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handlers) GetRep(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	rep, err := h.Rep.Get(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rep)
}

// =============================================================================
// Store endpoints
// =============================================================================

type CreateStoreRequest struct {
	StoreCode            string  `json:"store_code"            validate:"required,min=1,max=64"`
	Name                 string  `json:"name"                 validate:"required,min=1,max=255"`
	OwnerName            string  `json:"owner_name"           validate:"omitempty,max=255"`
	Phone                string  `json:"phone"                validate:"omitempty,max=32"`
	Address              string  `json:"address"              validate:"required,min=1"`
	Latitude             float64 `json:"latitude"             validate:"required,gte=-90,lte=90"`
	Longitude            float64 `json:"longitude"            validate:"required,gte=-180,lte=180"`
	GeofenceRadiusMeters int     `json:"geofence_radius_meters" validate:"omitempty,gte=10,lte=10000"`
	Tier                 string  `json:"tier"                 validate:"omitempty,oneof=GROSIR SEMI_GROSIR RETAIL STAR_OUTLET"`
	CreditLimit          int64   `json:"credit_limit"         validate:"omitempty,gte=0"`
}

func (h *Handlers) CreateStore(w http.ResponseWriter, r *http.Request) {
	var req CreateStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	s, err := h.Store.Create(r.Context(), usecase.CreateStoreInput{
		TenantID:             tenantFrom(r),
		StoreCode:            req.StoreCode,
		Name:                 req.Name,
		OwnerName:            req.OwnerName,
		Phone:                req.Phone,
		Address:              req.Address,
		Latitude:             req.Latitude,
		Longitude:            req.Longitude,
		GeofenceRadiusMeters: req.GeofenceRadiusMeters,
		Tier:                 storeTierOrDefault(req.Tier),
		CreditLimit:          req.CreditLimit,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, s)
}

func (h *Handlers) ListStores(w http.ResponseWriter, r *http.Request) {
	out, err := h.Store.List(r.Context(), tenantFrom(r), r.URL.Query().Get("tier"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handlers) GetStore(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	s, err := h.Store.Get(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

// =============================================================================
// Beat plan endpoints
// =============================================================================

type CreateBeatPlanRequest struct {
	PlanNumber        string `json:"plan_number" validate:"omitempty,max=64"`
	SalesRepID        string `json:"sales_rep_id" validate:"required,uuid"`
	PlanDate          string `json:"plan_date"   validate:"required"` // YYYY-MM-DD
	Territory         string `json:"territory"    validate:"required,min=1,max=128"`
	TargetStoresCount int    `json:"target_stores_count" validate:"omitempty,gte=0,lte=500"`
	Notes             string `json:"notes"        validate:"omitempty,max=500"`
}

func (h *Handlers) CreateBeatPlan(w http.ResponseWriter, r *http.Request) {
	var req CreateBeatPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	date, err := time.Parse("2006-01-02", req.PlanDate)
	if err != nil {
		httpx.Error(w, r, apperrors.ErrInvalidInputBadRequest.WithDetail(map[string]any{"plan_date": "must be YYYY-MM-DD"}))
		return
	}
	plan, err := h.Visit.CreateBeatPlan(r.Context(), usecase.CreateBeatPlanInput{
		TenantID:          tenantFrom(r),
		PlanNumber:        req.PlanNumber,
		SalesRepID:        req.SalesRepID,
		PlanDate:          date,
		Territory:         req.Territory,
		TargetStoresCount: req.TargetStoresCount,
		Notes:             req.Notes,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, plan)
}

func (h *Handlers) TodayBeatPlans(w http.ResponseWriter, r *http.Request) {
	repID := r.URL.Query().Get("sales_rep_id")
	plans, err := h.Visit.TodayBeatPlans(r.Context(), tenantFrom(r), repID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, plans)
}

// =============================================================================
// Visit check-in (Sprint 2)
// =============================================================================

type CheckInRequest struct {
	BeatPlanID string  `json:"beat_plan_id" validate:"required,uuid"`
	StoreID    string  `json:"store_id"     validate:"required,uuid"`
	Latitude   float64 `json:"latitude"     validate:"required,gte=-90,lte=90"`
	Longitude  float64 `json:"longitude"    validate:"required,gte=-180,lte=180"`
	VisitType  string  `json:"visit_type"   validate:"omitempty,oneof=TAKING_ORDER CASH_COLLECTION TAKING_ORDER_AND_COLLECTION CANVASSING NO_ORDER"`
}

func (h *Handlers) CheckIn(w http.ResponseWriter, r *http.Request) {
	var req CheckInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	salesRepID := r.URL.Query().Get("sales_rep_id")
	if salesRepID == "" {
		httpx.Error(w, r, apperrors.ErrInvalidInputBadRequest.WithDetail(map[string]any{"sales_rep_id": "required query param"}))
		return
	}
	res, err := h.Visit.CheckIn(r.Context(), usecase.CheckInInput{
		TenantID:   tenantFrom(r),
		BeatPlanID: req.BeatPlanID,
		StoreID:    req.StoreID,
		SalesRepID: salesRepID,
		Latitude:   req.Latitude,
		Longitude:  req.Longitude,
		VisitType:  visitTypeOrDefault(req.VisitType),
		ActorID:    actorIDFrom(r),
		IPAddress:  clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

func (h *Handlers) CompleteVisit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	v, err := h.Visit.CompleteVisit(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

// =============================================================================
// Collections + EOD Settlement (Sprint 3 + 4)
// =============================================================================

type CollectRequest struct {
	VisitID          string `json:"visit_id"           validate:"omitempty,uuid"`
	StoreID          string `json:"store_id"           validate:"required,uuid"`
	FledgerInvoiceID string `json:"fledger_invoice_id" validate:"required,uuid"`
	Amount           int64  `json:"amount"             validate:"required,gt=0"`
	PayerName        string `json:"payer_name"         validate:"required,min=1,max=255"`
	PayerPhone       string `json:"payer_phone"        validate:"omitempty,max=32"`
}

func (h *Handlers) Collect(w http.ResponseWriter, r *http.Request) {
	var req CollectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	salesRepID := r.URL.Query().Get("sales_rep_id")
	if salesRepID == "" {
		httpx.Error(w, r, apperrors.ErrInvalidInputBadRequest.WithDetail(map[string]any{"sales_rep_id": "required query param"}))
		return
	}
	res, err := h.Collection.Collect(r.Context(), usecase.CollectInput{
		TenantID:         tenantFrom(r),
		VisitID:          req.VisitID,
		SalesRepID:       salesRepID,
		StoreID:          req.StoreID,
		FledgerInvoiceID: req.FledgerInvoiceID,
		Amount:           req.Amount,
		PayerName:        req.PayerName,
		PayerPhone:       req.PayerPhone,
		ActorID:          actorIDFrom(r),
		IPAddress:        clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

func (h *Handlers) ListCollectionsToday(w http.ResponseWriter, r *http.Request) {
	repID := r.URL.Query().Get("sales_rep_id")
	if repID == "" {
		httpx.Error(w, r, apperrors.ErrInvalidInputBadRequest.WithDetail(map[string]any{"sales_rep_id": "required query param"}))
		return
	}
	out, err := h.Collection.ListByRep(r.Context(), repID, r.URL.Query().Get("status"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handlers) InquirySettlement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	res, err := h.Collection.Inquiry(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

type SettleRequest struct {
	SalesRepID           string `json:"sales_rep_id"            validate:"required,uuid"`
	PhysicalCashReceived int64  `json:"physical_cash_received" validate:"required,gte=0"`
	CashierNotes         string `json:"cashier_notes"           validate:"omitempty,max=500"`
}

func (h *Handlers) Settle(w http.ResponseWriter, r *http.Request) {
	var req SettleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	res, err := h.Collection.Settle(r.Context(), usecase.SettleInput{
		TenantID:             tenantFrom(r),
		SalesRepID:           req.SalesRepID,
		PhysicalCashReceived: req.PhysicalCashReceived,
		CashierNotes:         req.CashierNotes,
		ActorID:              actorIDFrom(r),
		IPAddress:            clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// =============================================================================
// Outbox
// =============================================================================

func (h *Handlers) OutboxCounts(w http.ResponseWriter, r *http.Request) {
	c, err := h.Collection.OutboxCounts(r.Context(), tenantFrom(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

// =============================================================================
// Helpers
// =============================================================================

func tenantFrom(r *http.Request) string {
	if h := r.Header.Get("X-Tenant-ID"); h != "" {
		return h
	}
	return ""
}
func actorIDFrom(r *http.Request) string {
	if h := r.Header.Get("X-Actor-Id"); h != "" {
		return h
	}
	return ""
}
func clientIP(r *http.Request) string {
	if h := r.Header.Get("X-Forwarded-For"); h != "" {
		return strings.SplitN(h, ",", 2)[0]
	}
	return r.RemoteAddr
}

// Helper types — use the domain enums directly. The validator above enforces
// the allowed values.

func salesrepRoleOrDefault(s string) salesrep.Role {
	if s == "" {
		return salesrep.RoleCanvasser
	}
	return salesrep.Role(s)
}

func storeTierOrDefault(s string) store.Tier {
	if s == "" {
		return store.TierRetail
	}
	return store.Tier(s)
}

func visitTypeOrDefault(s string) visit.VisitType {
	if s == "" {
		return visit.VisitTakingOrderAndCollection
	}
	return visit.VisitType(s)
}

// keep imports used
var _ = strconv.Itoa
var _ = beatplan.StatusAssigned