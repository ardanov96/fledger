// Package handler ??? HTTP handlers for Fledger Dunning.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/fledger/fledger-dunning/internal/domain"
	"github.com/fledger/fledger-dunning/internal/middleware"
	apperrors "github.com/fledger/fledger-dunning/internal/platform/errors"
	"github.com/fledger/fledger-dunning/internal/platform/httpx"
	"github.com/fledger/fledger-dunning/internal/usecase"
)

type Handlers struct {
	Validator *validator.Validate
	Dunning   *usecase.DunningService
	Statement *usecase.StatementService
	DBPinger  func() error
}

func NewHandlers(v *validator.Validate, srv *usecase.Services, pinger func() error) *Handlers {
	return &Handlers{Validator: v, Dunning: srv.Dunning, Statement: srv.Statement, DBPinger: pinger}
}

// =============================================================================
// Health & dev login
// =============================================================================

func (h *Handlers) Healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "alive", "service": "fledger-dunning"})
}

func (h *Handlers) Readyz(w http.ResponseWriter, r *http.Request) {
	if h.DBPinger == nil {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
		return
	}
	if err := h.DBPinger(); err != nil {
		httpx.Error(w, r, apperrors.New(http.StatusServiceUnavailable, "not_ready", "db ping failed").WithDetail(map[string]any{"err": err.Error()}))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// =============================================================================
// WhatsApp session
// =============================================================================

func (h *Handlers) GetWhatsAppStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantFromRequest(r)
	sess, err := h.Dunning.SessionStatus(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.EnvelopeOK{Status: "success", Data: sess})
}

type GenerateQRRequest struct {
	SessionName string `json:"session_name"`
}

func (h *Handlers) GenerateQR(w http.ResponseWriter, r *http.Request) {
	var req GenerateQRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if req.SessionName == "" {
		req.SessionName = "default"
	}
	tenantID := middleware.TenantFromRequest(r)
	qr, err := h.Dunning.GenerateQR(r.Context(), tenantID, req.SessionName)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.EnvelopeOK{Status: "success", Data: map[string]any{"qr_code": qr}})
}

func (h *Handlers) DisconnectWA(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantFromRequest(r)
	actorID := r.Header.Get("X-Actor-Id")
	if err := h.Dunning.Disconnect(r.Context(), tenantID, actorID, clientIP(r)); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.EnvelopeMessage{Status: "success", Message: "WhatsApp session disconnected"})
}

type TestSendRequest struct {
	PhoneNumber string `json:"phone_number" validate:"required,min=8,max=30"`
	Body        string `json:"body"         validate:"required,min=1,max=1000"`
}

func (h *Handlers) TestSend(w http.ResponseWriter, r *http.Request) {
	var req TestSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	tenantID := middleware.TenantFromRequest(r)
	pid, err := h.Dunning.SendTestMessage(r.Context(), tenantID, req.PhoneNumber, req.Body)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.EnvelopeOK{Status: "success", Data: map[string]any{"provider_message_id": pid}})
}

// =============================================================================
// Queues
// =============================================================================

type IngestInvoiceRequest struct {
	InvoiceID      string `json:"invoice_id"      validate:"required,min=1,max=100"`
	InvoiceNumber  string `json:"invoice_number"  validate:"required,min=1,max=100"`
	StoreID        string `json:"store_id"        validate:"required,min=1,max=100"`
	PhoneNumber    string `json:"phone_number"    validate:"required,min=8,max=30"`
	DueDate        string `json:"due_date"        validate:"required"`
	AmountDueMinor int64  `json:"amount_due_minor" validate:"required,gt=0"`
	PaymentLinkURL string `json:"payment_link_url" validate:"required,url"`
	StoreName      string `json:"store_name"`
}

func (h *Handlers) IngestInvoice(w http.ResponseWriter, r *http.Request) {
	var req IngestInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	due, err := time.Parse("2006-01-02", req.DueDate)
	if err != nil {
		httpx.Error(w, r, apperrors.ErrInvalidInputBadRequest.WithDetail(map[string]any{"due_date": "must be YYYY-MM-DD"}))
		return
	}
	tenantID := middleware.TenantFromRequest(r)
	res, err := h.Dunning.Ingest(r.Context(), usecase.IngestInput{
		TenantID:       tenantID,
		InvoiceID:      req.InvoiceID,
		InvoiceNumber:  req.InvoiceNumber,
		StoreID:        req.StoreID,
		PhoneNumber:    req.PhoneNumber,
		DueDate:        due,
		AmountDueMinor: req.AmountDueMinor,
		PaymentLinkURL: req.PaymentLinkURL,
		StoreName:      req.StoreName,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, httpx.EnvelopeOK{Status: "success", Data: res})
}

type CancelInvoiceRequest struct {
	InvoiceID string `json:"invoice_id" validate:"required"`
}

func (h *Handlers) CancelInvoice(w http.ResponseWriter, r *http.Request) {
	var req CancelInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	tenantID := middleware.TenantFromRequest(r)
	cancelled, err := h.Dunning.CancelByInvoice(r.Context(), usecase.CancelByInvoiceInput{
		TenantID:  tenantID,
		InvoiceID: req.InvoiceID,
		ActorID:   r.Header.Get("X-Actor-Id"),
		IP:        clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.EnvelopeMessage{
		Status:  "success",
		Message: "Antrian berhasil dibatalkan",
		Count:   cancelled,
	})
}

func (h *Handlers) ListQueues(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantFromRequest(r)
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	out, err := h.Dunning.ListQueues(r.Context(), tenantID, status, limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handlers) DispatchOne(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	// The dispatch path runs the full loop, not a single row. Expose this
	// endpoint for manual triggering.
	sent, failed, err := h.Dunning.DispatchDue(r.Context(), 1)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.EnvelopeMessage{Status: "success", Count: sent + failed})
}

func (h *Handlers) CronRun(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantFromRequest(r)
	_ = tenantID // cron-run is global; tenant scoping is done inside the loop.
	sent, failed, err := h.Dunning.DispatchDue(r.Context(), 16)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"sent":    sent,
		"failed":  failed,
	})
}

func (h *Handlers) QueueCounts(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantFromRequest(r)
	c, err := h.Dunning.QueueCounts(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

// GetQueueByID returns one queue row.
func (h *Handlers) GetQueueByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	tenantID := middleware.TenantFromRequest(r)
	items, err := h.Dunning.ListQueues(r.Context(), tenantID, "", 200)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	for _, it := range items {
		if it.ID == id {
			httpx.JSON(w, http.StatusOK, it)
			return
		}
	}
	httpx.Error(w, r, apperrors.New(http.StatusNotFound, "not_found", "queue not found"))
}

// =============================================================================
// Contacts
// =============================================================================

type CreateContactRequest struct {
	StoreID   string `json:"store_id"   validate:"required,min=1,max=100"`
	StoreName string `json:"store_name" validate:"required,min=1,max=255"`
	OwnerName string `json:"owner_name" validate:"required,min=1,max=255"`
	Phone     string `json:"phone_number" validate:"required,min=8,max=30"`
	Email     string `json:"email"        validate:"omitempty,email"`
}

// toContact maps the request into a domain.StoreContact.
func toContact(tenantID string, r CreateContactRequest) domain.StoreContact {
	return domain.StoreContact{
		TenantID:  tenantID,
		StoreID:   r.StoreID,
		StoreName: r.StoreName,
		OwnerName: r.OwnerName,
		Phone:     r.Phone,
		Email:     r.Email,
		IsActive:  true,
	}
}

func (h *Handlers) UpsertContact(w http.ResponseWriter, r *http.Request) {
	var req CreateContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	tenantID := middleware.TenantFromRequest(r)
	c, err := h.Dunning.Contacts().Insert(r.Context(), toContact(tenantID, req))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.EnvelopeOK{Status: "success", Data: c})
}

func (h *Handlers) ListContacts(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantFromRequest(r)
	out, err := h.Dunning.Contacts().List(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// =============================================================================
// Statements
// =============================================================================

type GenerateStatementRequest struct {
	StoreID       string `json:"store_id"       validate:"required,min=1,max=100"`
	StatementMonth string `json:"statement_month" validate:"omitempty,len=7"`
}

func (h *Handlers) GenerateStatement(w http.ResponseWriter, r *http.Request) {
	var req GenerateStatementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	tenantID := middleware.TenantFromRequest(r)
	res, err := h.Statement.Generate(r.Context(), usecase.GenerateInput{
		TenantID:       tenantID,
		StoreID:        req.StoreID,
		StatementMonth: req.StatementMonth,
		ActorID:        r.Header.Get("X-Actor-Id"),
		IP:             clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, httpx.EnvelopeOK{Status: "success", Data: res})
}

func (h *Handlers) SendStatement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	tenantID := middleware.TenantFromRequest(r)
	res, err := h.Statement.Send(r.Context(), usecase.SendInput{
		TenantID:    tenantID,
		StatementID: id,
		ActorID:     r.Header.Get("X-Actor-Id"),
		IP:          clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.EnvelopeOK{Status: "success", Data: res})
}

func (h *Handlers) ListStatements(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantFromRequest(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	out, err := h.Statement.List(r.Context(), tenantID, limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// =============================================================================
// Webhooks
// =============================================================================

type PayWebhookRequest struct {
	Event          string `json:"event" validate:"required,eq=payment.settled"`
	InvoiceID      string `json:"invoice_id" validate:"required"`
	PaidAmountMinor int64  `json:"paid_amount_minor" validate:"required,gt=0"`
	PaidAt         string `json:"paid_at"        validate:"required"`
	SettlementRef  string `json:"settlement_ref"`
}

// PayWebhook handles the payment.settled callback from Fledger Pay. It
// authenticates via a shared secret in the X-Webhook-Secret header.
func (h *Handlers) PayWebhook(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.PayWebhookSecret != "" {
			if r.Header.Get("X-Webhook-Secret") != cfg.PayWebhookSecret {
				httpx.Error(w, r, apperrors.ErrWebhookSignatureInvalid)
				return
			}
		}
		var req PayWebhookRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
			return
		}
		if err := h.Validator.Struct(&req); err != nil {
			httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
			return
		}
		tenantID := middleware.TenantFromRequest(r)
		_ = req.PaidAt // future: persist settlement timestamp
		cancelled, err := h.Dunning.CancelByInvoice(r.Context(), usecase.CancelByInvoiceInput{
			TenantID:  tenantID,
			InvoiceID: req.InvoiceID,
			ActorID:   "webhook:fledger-pay",
			IP:        clientIP(r),
		})
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{
			"status":          "success",
			"message":          "Seluruh jadwal dunning untuk faktur " + req.InvoiceID + " berhasil dibatalkan otomatis",
			"cancelled_count":  cancelled,
		})
	}
}

// Config is a tiny bag used by the webhook handler to access the secret.
// It is built once in main.go.
type Config struct {
	PayWebhookSecret string
}

func clientIP(r *http.Request) string {
	if h := r.Header.Get("X-Forwarded-For"); h != "" {
		return strings.SplitN(h, ",", 2)[0]
	}
	return r.RemoteAddr
}