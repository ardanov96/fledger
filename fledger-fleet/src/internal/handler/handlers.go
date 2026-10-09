// Package handler exposes thin REST endpoints for Fledger Fleet. Handlers
// translate HTTP requests, call usecase methods, and translate errors to
// the platform's HTTP envelope.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/fledger/fledger-fleet/internal/domain/delivery_order"
	"github.com/fledger/fledger-fleet/internal/domain/driver"
	"github.com/fledger/fledger-fleet/internal/domain/pod"
	"github.com/fledger/fledger-fleet/internal/domain/trip"
	"github.com/fledger/fledger-fleet/internal/domain/vehicle"
	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/platform/httpx"
	"github.com/fledger/fledger-fleet/internal/usecase"
)

// Handlers bundles the use-case layer behind a single value. Use NewHandlers
// to wire it; tests can inject fakes via the setter methods.
type Handlers struct {
	Validator *validator.Validate
	Vehicle   *usecase.VehicleService
	Driver    *usecase.DriverService
	Trip      *usecase.TripService
	DO        *usecase.DeliveryOrderService
	POD       *usecase.ProofOfDeliveryService
	Outbox    *usecase.OutboxService
}

// NewHandlers builds a Handlers value.
func NewHandlers(v *validator.Validate, srv *usecase.Services) *Handlers {
	return &Handlers{
		Validator: v,
		Vehicle:   srv.Vehicle,
		Driver:    srv.Driver,
		Trip:      srv.Trip,
		DO:        srv.DO,
		POD:       srv.POD,
		Outbox:    srv.Outbox,
	}
}

// =============================================================================
// Vehicle endpoints
// =============================================================================

type CreateVehicleRequest struct {
	PlateNumber string  `json:"plate_number" validate:"required,min=1,max=20"`
	VehicleType string  `json:"vehicle_type"  validate:"required,oneof=CDE_BOX CDD_BOX BLIND_VAN MOTOR_CARGO"`
	BrandModel string  `json:"brand_model"   validate:"omitempty,max=100"`
	CapacityKg float64 `json:"capacity_kg"   validate:"required,gt=0"`
}

func toVehicleCreate(req CreateVehicleRequest, tenantID string) vehicle.Vehicle {
	return vehicle.Vehicle{
		TenantID:    tenantID,
		PlateNumber: req.PlateNumber,
		VehicleType: req.VehicleType,
		BrandModel:  req.BrandModel,
		CapacityKg:  req.CapacityKg,
		Status:      vehicle.StatusAvailable,
	}
}

// ListVehicles — GET /v1/fleet/vehicles?status=
func (h *Handlers) ListVehicles(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFrom(r)
	status := r.URL.Query().Get("status")
	out, err := h.Vehicle.List(r.Context(), tenantID, status)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// CreateVehicle — POST /v1/fleet/vehicles
func (h *Handlers) CreateVehicle(w http.ResponseWriter, r *http.Request) {
	var req CreateVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	out, err := h.Vehicle.Create(r.Context(), toVehicleCreate(req, tenantFrom(r)))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

// GetVehicle — GET /v1/fleet/vehicles/:id
func (h *Handlers) GetVehicle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	out, err := h.Vehicle.Get(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// =============================================================================
// Driver endpoints
// =============================================================================

type CreateDriverRequest struct {
	FullName      string `json:"full_name"      validate:"required,min=1,max=100"`
	PhoneNumber   string `json:"phone_number"   validate:"required,min=6,max=30"`
	LicenseNumber string `json:"license_number" validate:"required,min=1,max=50"`
}

func toDriverCreate(req CreateDriverRequest, tenantID string) driver.Driver {
	return driver.Driver{
		TenantID:      tenantID,
		FullName:      req.FullName,
		PhoneNumber:   req.PhoneNumber,
		LicenseNumber: req.LicenseNumber,
		Status:        driver.StatusActive,
	}
}

// ListDrivers — GET /v1/fleet/drivers?status=
func (h *Handlers) ListDrivers(w http.ResponseWriter, r *http.Request) {
	out, err := h.Driver.List(r.Context(), tenantFrom(r), r.URL.Query().Get("status"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// CreateDriver — POST /v1/fleet/drivers
func (h *Handlers) CreateDriver(w http.ResponseWriter, r *http.Request) {
	var req CreateDriverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	out, err := h.Driver.Create(r.Context(), toDriverCreate(req, tenantFrom(r)))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

// GetDriver — GET /v1/fleet/drivers/:id
func (h *Handlers) GetDriver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	out, err := h.Driver.Get(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// =============================================================================
// Trip endpoints
// =============================================================================

type CreateTripRequest struct {
	TripNumber string   `json:"trip_number" validate:"required,min=1,max=50"`
	VehicleID  string   `json:"vehicle_id"  validate:"required,uuid"`
	DriverID   string   `json:"driver_id"   validate:"required,uuid"`
	DoIDs      []string `json:"do_ids"      validate:"required,min=1,dive,uuid"`
	Notes      string   `json:"notes"       validate:"omitempty,max=500"`
}

// CreateTrip — POST /v1/fleet/trips
func (h *Handlers) CreateTrip(w http.ResponseWriter, r *http.Request) {
	var req CreateTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	out, err := h.Trip.Create(r.Context(), usecase.CreateTripInput{
		TenantID:   tenantFrom(r),
		TripNumber: req.TripNumber,
		VehicleID:  req.VehicleID,
		DriverID:   req.DriverID,
		DoIDs:      req.DoIDs,
		Notes:      req.Notes,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

// ListTrips — GET /v1/fleet/trips?status=
func (h *Handlers) ListTrips(w http.ResponseWriter, r *http.Request) {
	out, err := h.Trip.List(r.Context(), tenantFrom(r), r.URL.Query().Get("status"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// ListTodayTrips — GET /v1/fleet/trips/today
func (h *Handlers) ListTodayTrips(w http.ResponseWriter, r *http.Request) {
	out, err := h.Trip.ListToday(r.Context(), tenantFrom(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// GetTrip — GET /v1/fleet/trips/:id
func (h *Handlers) GetTrip(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	out, err := h.Trip.Get(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// DispatchTrip — POST /v1/fleet/trips/:id/dispatch
func (h *Handlers) DispatchTrip(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	out, err := h.Trip.Dispatch(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// =============================================================================
// Delivery Order endpoints
// =============================================================================

type CreateDOItemRequest struct {
	ProductSKU     string `json:"product_sku"     validate:"required,min=1,max=50"`
	ProductName    string `json:"product_name"    validate:"required,min=1,max=150"`
	QtyOrdered     int    `json:"qty_ordered"     validate:"required,gt=0"`
	UnitPriceCents int64  `json:"unit_price_cents" validate:"required,gt=0"`
}

type CreateDORequest struct {
	DoNumber           string                `json:"do_number"            validate:"required,min=1,max=50"`
	CustomerID         string                `json:"customer_id"          validate:"required,uuid"`
	CustomerName       string                `json:"customer_name"        validate:"required,min=1,max=150"`
	DestinationAddress string                `json:"destination_address"  validate:"required,min=1"`
	DestinationLat     *float64              `json:"destination_lat"      validate:"omitempty,gte=-90,lte=90"`
	DestinationLng     *float64              `json:"destination_lng"      validate:"omitempty,gte=-180,lte=180"`
	Items              []CreateDOItemRequest `json:"items"                validate:"required,min=1,dive"`
}

// CreateDO — POST /v1/fleet/delivery-orders
func (h *Handlers) CreateDO(w http.ResponseWriter, r *http.Request) {
	var req CreateDORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	items := make([]delivery_order.DOItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, delivery_order.DOItem{
			ProductSKU:     it.ProductSKU,
			ProductName:    it.ProductName,
			QtyOrdered:     it.QtyOrdered,
			UnitPriceCents: it.UnitPriceCents,
		})
	}
	do, _, err := h.DO.Create(r.Context(), usecase.CreateInput{
		TenantID:           tenantFrom(r),
		DoNumber:           req.DoNumber,
		CustomerID:         req.CustomerID,
		CustomerName:       req.CustomerName,
		DestinationAddress: req.DestinationAddress,
		DestinationLat:     req.DestinationLat,
		DestinationLng:     req.DestinationLng,
		Items:              items,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, do)
}

// ListDOs — GET /v1/fleet/delivery-orders?status=
func (h *Handlers) ListDOs(w http.ResponseWriter, r *http.Request) {
	out, err := h.DO.List(r.Context(), tenantFrom(r), r.URL.Query().Get("status"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// GetDO — GET /v1/fleet/delivery-orders/:id
func (h *Handlers) GetDO(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	do, items, err := h.DO.Get(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"do": do, "items": items})
}

// =============================================================================
// POD endpoint
// =============================================================================

type PODItemResult struct {
	ProductSKU      string `json:"product_sku"      validate:"required,min=1,max=50"`
	QtyDelivered    int    `json:"qty_delivered"    validate:"gte=0"`
	QtyRejected     int    `json:"qty_rejected"     validate:"gte=0"`
	RejectionReason string `json:"rejection_reason" validate:"omitempty,max=100"`
}

type SubmitPODRequest struct {
	RecipientName     string          `json:"recipient_name"      validate:"required,min=1,max=100"`
	RecipientPhone    string          `json:"recipient_phone"     validate:"omitempty,max=30"`
	SignatureDataURL  string          `json:"signature_data_url"  validate:"required,min=1"`
	PhotoEvidenceURLs []string        `json:"photo_evidence_urls" validate:"omitempty,dive"`
	DeliveredLat      float64         `json:"delivered_lat"       validate:"gte=-90,lte=90"`
	DeliveredLng      float64         `json:"delivered_lng"       validate:"gte=-180,lte=180"`
	DriverNotes       string          `json:"driver_notes"        validate:"omitempty,max=1000"`
	Items             []PODItemResult `json:"items"              validate:"omitempty,dive"`
	ItemsResult       []PODItemResult `json:"items_result"       validate:"omitempty,dive"`
}

// SubmitPOD — POST /v1/fleet/delivery-orders/:id/pod
func (h *Handlers) SubmitPOD(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	var req SubmitPODRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	// Support both "items" and "items_result" per API-SPECIFICATION.md §3.2
	if len(req.Items) == 0 && len(req.ItemsResult) > 0 {
		req.Items = req.ItemsResult
	}
	if len(req.Items) == 0 {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": "items or items_result is required and must contain at least 1 item"})
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	items := make([]pod.ItemResult, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, pod.ItemResult{
			ProductSKU:      it.ProductSKU,
			QtyDelivered:    it.QtyDelivered,
			QtyRejected:     it.QtyRejected,
			RejectionReason: it.RejectionReason,
		})
	}
	res, err := h.POD.Submit(r.Context(), usecase.SubmitInput{
		TenantID:          tenantFrom(r),
		DOID:              id,
		ActorID:           actorIDFrom(r),
		RecipientName:     req.RecipientName,
		RecipientPhone:    req.RecipientPhone,
		SignatureDataURL:  req.SignatureDataURL,
		PhotoEvidenceURLs: req.PhotoEvidenceURLs,
		DeliveredLat:      req.DeliveredLat,
		DeliveredLng:      req.DeliveredLng,
		DriverNotes:       req.DriverNotes,
		Items:             items,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// =============================================================================
// Ops endpoints
// =============================================================================

// OutboxCounts — GET /v1/fleet/outbox/counts
func (h *Handlers) OutboxCounts(w http.ResponseWriter, r *http.Request) {
	c, err := h.Outbox.Counts(r.Context(), tenantFrom(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

// Ping — GET /v1/fleet/ping  (liveness probe)
func (h *Handlers) Ping(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"message": "pong", "service": "fledger-fleet"})
}

// tenantFrom extracts the tenant id from the request context (set by auth
// middleware). Falls back to the X-Tenant-ID header for /v1/ping.
func tenantFrom(r *http.Request) string {
	if p := principalFromCtx(r.Context()); p != nil && p.TenantID != "" {
		return p.TenantID
	}
	if h := r.Header.Get("X-Tenant-ID"); h != "" {
		return h
	}
	return ""
}

// limit parses the ?limit=N query param, defaulting to 100.
func limitFrom(r *http.Request) int {
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			return n
		}
	}
	return 100
}

// Avoid unused symbol on the unused limitFrom for now.
var _ = limitFrom

// Avoid unused symbol on trip.Status import.
var _ = trip.Status("")