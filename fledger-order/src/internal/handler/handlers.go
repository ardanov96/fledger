// Package handler exposes REST endpoints for Fledger Order.
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

	"github.com/fledger/fledger-order/internal/domain/audit"
	"github.com/fledger/fledger-order/internal/domain/order"
	"github.com/fledger/fledger-order/internal/domain/pricing"
	"github.com/fledger/fledger-order/internal/domain/product"
	apperrors "github.com/fledger/fledger-order/internal/platform/errors"
	"github.com/fledger/fledger-order/internal/platform/httpx"
	"github.com/fledger/fledger-order/internal/usecase"
)

type Handlers struct {
	Validator *validator.Validate
	Product   *usecase.ProductService
	Pricing   *usecase.PricingService
	Inv       *usecase.InventoryService
	Order     *usecase.OrderService
	DBPinger  func(ctx context.Context) error
}

func NewHandlers(v *validator.Validate, srv *usecase.Services, pinger func(ctx context.Context) error) *Handlers {
	return &Handlers{
		Validator: v,
		Product:   srv.Product,
		Pricing:   srv.Pricing,
		Inv:       srv.Inv,
		Order:     srv.Order,
		DBPinger:  pinger,
	}
}

// =============================================================================
// Health & dev login
// =============================================================================

func (h *Handlers) Healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "alive", "service": "fledger-order"})
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
// Products
// =============================================================================

type CreateProductRequest struct {
	SKU         string `json:"sku"          validate:"required,min=1,max=64"`
	Barcode     string `json:"barcode"`
	Name        string `json:"name"         validate:"required,min=1,max=255"`
	Category    string `json:"category"     validate:"omitempty,oneof=SEMBAKO MIE_INSTAN MINUMAN TOILETRIES ROKOK"`
	Unit        string `json:"unit"         validate:"omitempty,oneof=DUS KARTON PACK RENCENG PCS"`
	WeightGrams int    `json:"weight_grams" validate:"omitempty,gt=0"`
}

func (h *Handlers) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	cat := product.Category(req.Category)
	if cat == "" {
		cat = product.CategorySembako
	}
	unit := product.Unit(req.Unit)
	if unit == "" {
		unit = product.UnitDus
	}
	p, err := h.Product.Create(r.Context(), usecase.CreateProductInput{
		TenantID:    tenantFrom(r),
		SKU:         req.SKU,
		Barcode:     req.Barcode,
		Name:        req.Name,
		Category:    cat,
		Unit:        unit,
		WeightGrams: req.WeightGrams,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

func (h *Handlers) ListProducts(w http.ResponseWriter, r *http.Request) {
	out, err := h.Product.List(r.Context(), tenantFrom(r), r.URL.Query().Get("category"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handlers) GetProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	p, err := h.Product.Get(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

type CreatePricingRequest struct {
	Tier        string `json:"tier"         validate:"required,oneof=GROSIR SEMI_GROSIR RETAIL STAR_OUTLET"`
	MinQuantity int    `json:"min_quantity" validate:"required,gte=1"`
	UnitPrice   int64  `json:"unit_price"   validate:"required,gt=0"`
}

func (h *Handlers) CreatePricing(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(productID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	var req CreatePricingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	out, err := h.Pricing.Create(r.Context(), usecase.CreatePricingInput{
		TenantID:    tenantFrom(r),
		ProductID:   productID,
		Tier:        pricing.Tier(req.Tier),
		MinQuantity: req.MinQuantity,
		UnitPrice:   req.UnitPrice,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handlers) ListPricing(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "id")
	out, err := h.Pricing.ListByProduct(r.Context(), productID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// =============================================================================
// Inventory
// =============================================================================

type AdjustStockRequest struct {
	ProductID   string `json:"product_id"   validate:"required,uuid"`
	WarehouseID string `json:"warehouse_id"`
	Delta       int    `json:"delta"`
}

func (h *Handlers) AdjustStock(w http.ResponseWriter, r *http.Request) {
	var req AdjustStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	out, err := h.Inv.Adjust(r.Context(), usecase.AdjustStockInput{
		TenantID:    tenantFrom(r),
		ProductID:   req.ProductID,
		WarehouseID: req.WarehouseID,
		Delta:       req.Delta,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handlers) ListInventory(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Query().Get("product_id")
	if productID == "" {
		httpx.Error(w, r, apperrors.ErrInvalidInputBadRequest.WithDetail(map[string]any{"product_id": "required"}))
		return
	}
	if _, err := uuid.Parse(productID); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	out, err := h.Inv.ListByProduct(r.Context(), productID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// =============================================================================
// Orders
// =============================================================================

type CreateOrderItem struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Quantity  int    `json:"quantity"    validate:"required,gt=0"`
}

type CreateOrderRequest struct {
	CustomerID         string              `json:"customer_id"         validate:"required,uuid"`
	CustomerName       string              `json:"customer_name"       validate:"required,min=1,max=255"`
	CustomerTier       string              `json:"customer_tier"       validate:"required,oneof=GROSIR SEMI_GROSIR RETAIL STAR_OUTLET"`
	CustomerPhone      string              `json:"customer_phone"`
	DestinationAddress string              `json:"destination_address" validate:"required,min=1"`
	Items              []CreateOrderItem   `json:"items"               validate:"required,min=1,dive"`
	Notes              string              `json:"notes"`
}

func (h *Handlers) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	items := make([]usecase.CreateInputItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, usecase.CreateInputItem{ProductID: it.ProductID, Quantity: it.Quantity})
	}
	out, err := h.Order.Create(r.Context(), usecase.CreateInput{
		TenantID:           tenantFrom(r),
		CustomerID:         req.CustomerID,
		CustomerName:       req.CustomerName,
		CustomerTier:       pricing.Tier(req.CustomerTier),
		CustomerPhone:      req.CustomerPhone,
		DestinationAddress: req.DestinationAddress,
		Items:              items,
		Notes:              req.Notes,
		ActorID:            actorIDFrom(r),
		IPAddress:          clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handlers) ListOrders(w http.ResponseWriter, r *http.Request) {
	out, err := h.Order.List(r.Context(), tenantFrom(r), r.URL.Query().Get("status"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handlers) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	o, items, err := h.Order.Get(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"order": o, "items": items})
}

func (h *Handlers) EvaluateCredit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	res, err := h.Order.EvaluateCredit(r.Context(), usecase.EvaluateInput{
		TenantID:  tenantFrom(r),
		OrderID:   id,
		ActorID:   actorIDFrom(r),
		IPAddress: clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

type OverrideRequest struct {
	OverridePIN string `json:"override_pin" validate:"required,min=1"`
	Reason      string `json:"reason"       validate:"required,min=1,max=500"`
}

func (h *Handlers) OverrideCredit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	var req OverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	o, err := h.Order.OverrideCredit(r.Context(), usecase.OverrideInput{
		TenantID:   tenantFrom(r),
		OrderID:    id,
		OverridePIN: req.OverridePIN,
		Reason:     req.Reason,
		ActorID:    actorIDFrom(r),
		IPAddress:  clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, o)
}

func (h *Handlers) CancelOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	o, err := h.Order.Cancel(r.Context(), usecase.CancelInput{
		TenantID:  tenantFrom(r),
		OrderID:   id,
		ActorID:   actorIDFrom(r),
		IPAddress: clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, o)
}

func (h *Handlers) OutboxCounts(w http.ResponseWriter, r *http.Request) {
	c, err := h.Order.OutboxCounts(r.Context(), tenantFrom(r))
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

// avoid unused import
var _ = strconv.Itoa
var _ = audit.ActionCreateOrder
var _ = order.StatusDraft