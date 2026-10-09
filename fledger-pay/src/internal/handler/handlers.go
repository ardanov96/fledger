// Package handler exposes REST + webhook handlers for Fledger Pay.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/fledger/fledger-pay/internal/domain/transaction"
	apperrors "github.com/fledger/fledger-pay/internal/platform/errors"
	"github.com/fledger/fledger-pay/internal/platform/httpx"
	"github.com/fledger/fledger-pay/internal/usecase"
	"github.com/fledger/fledger-pay/internal/webhook"
)

// Handlers bundles the use-case layer.
type Handlers struct {
	Validator    *validator.Validate
	Payment      *usecase.PaymentService
	Settlement   *usecase.SettlementService
	WebhookSecret string
	DBPinger     func(ctx context.Context) error
}

func NewHandlers(v *validator.Validate, srv *usecase.Services, webhookSecret string, pinger func(ctx context.Context) error) *Handlers {
	return &Handlers{
		Validator:    v,
		Payment:      srv.Payment,
		Settlement:   srv.Settlement,
		WebhookSecret: webhookSecret,
		DBPinger:     pinger,
	}
}

// =============================================================================
// Health probes
// =============================================================================

func (h *Handlers) Healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "alive", "service": "fledger-pay"})
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
// Payment request endpoints
// =============================================================================

type CreatePaymentRequest struct {
	FledgerInvoiceID string   `json:"fledger_invoice_id" validate:"required,uuid"`
	CustomerID       string   `json:"customer_id"        validate:"required,uuid"`
	CustomerName     string   `json:"customer_name"      validate:"required,min=1,max=255"`
	CustomerPhone    string   `json:"customer_phone"     validate:"omitempty,max=32"`
	Amount           int64    `json:"amount"             validate:"required,gt=0"`
	ExpiryMinutes    int      `json:"expiry_minutes"     validate:"omitempty,gte=1,lte=43200"`
	EnabledBanks     []string `json:"enabled_banks"      validate:"omitempty,dive,oneof=BCA MANDIRI BRI BNI PERMATA CIMB"`
	EnableQRIS       bool     `json:"enable_qris"`
	MerchantCity     string   `json:"merchant_city"      validate:"omitempty,max=32"`
}

func (h *Handlers) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	out, err := h.Payment.Create(r.Context(), usecase.CreateInput{
		TenantID:         tenantFrom(r),
		FledgerInvoiceID: req.FledgerInvoiceID,
		CustomerID:       req.CustomerID,
		CustomerName:     req.CustomerName,
		CustomerPhone:    req.CustomerPhone,
		Amount:           req.Amount,
		ExpiryMinutes:    req.ExpiryMinutes,
		EnabledBanks:     req.EnabledBanks,
		EnableQRIS:       req.EnableQRIS,
		MerchantCity:     req.MerchantCity,
		ActorID:          actorIDFrom(r),
		IPAddress:        clientIP(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handlers) ListPayments(w http.ResponseWriter, r *http.Request) {
	out, err := h.Payment.List(r.Context(), tenantFrom(r), r.URL.Query().Get("status"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handlers) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	out, err := h.Payment.Get(r.Context(), tenantFrom(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handlers) CancelPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	pr, err := h.Payment.Cancel(r.Context(), tenantFrom(r), id, actorIDFrom(r), clientIP(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, pr)
}

// =============================================================================
// Webhook callback
// =============================================================================

type MidtransPayload struct {
	OrderID           string `json:"order_id"`
	TransactionStatus string `json:"transaction_status"`
	GrossAmount       string `json:"gross_amount"`
	PaymentType       string `json:"payment_type"`
	Bank              string `json:"bank,omitempty"`
	VANumber          string `json:"va_number,omitempty"`
	TransactionID     string `json:"transaction_id"`
	SignatureKey      string `json:"signature_key"`
	TransactionTime   string `json:"transaction_time"`
}

// WebhookHandler dispatches by gateway.
func (h *Handlers) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	gateway := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "gateway")))
	tenantID := tenantFrom(r)
	bodyBytes, err := readAll(r)
	if err != nil {
		httpx.Error(w, r, apperrors.Wrap(http.StatusBadRequest, "invalid_input", "cannot read body", err))
		return
	}
	switch gateway {
	case "midtrans":
		h.handleMidtrans(w, r, tenantID, bodyBytes)
	case "xendit":
		h.handleXendit(w, r, tenantID, bodyBytes)
	case "direct":
		h.handleDirect(w, r, tenantID, bodyBytes)
	default:
		httpx.Error(w, r, apperrors.New(http.StatusBadRequest, "invalid_gateway", "unsupported gateway: "+gateway))
	}
}

func (h *Handlers) handleMidtrans(w http.ResponseWriter, r *http.Request, tenantID string, raw []byte) {
	var p MidtransPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		httpx.Error(w, r, apperrors.Wrap(http.StatusBadRequest, "invalid_input", "bad midtrans payload", err))
		return
	}
	if p.SignatureKey == "" {
		httpx.Error(w, r, apperrors.ErrSignatureInvalid401.WithDetail(map[string]any{"reason": "missing signature_key"}))
		return
	}
	statusCode := midtransStatusCode(p.TransactionStatus)
	if err := webhook.VerifyMidtransSignature(p.OrderID, statusCode, p.GrossAmount, h.WebhookSecret, p.SignatureKey); err != nil {
		httpx.Error(w, r, apperrors.ErrSignatureInvalid401.WithDetail(map[string]any{"reason": err.Error()}))
		return
	}
	amount, _ := parseGrossAmount(p.GrossAmount)
	channel := midtransChannel(p.PaymentType, p.Bank)
	res, err := h.Settlement.ProcessCallback(r.Context(), usecase.WebhookInput{
		TenantID:    tenantID,
		Gateway:     "midtrans",
		OrderID:     p.OrderID,
		Status:      p.TransactionStatus,
		GrossAmount: amount,
		Channel:     channel,
		ExternalRef: p.TransactionID,
		IdempKey:    p.SignatureKey,
		PayerName:   p.VANumber,
		PayerBank:   p.Bank,
		RawPayload:  rawMap(raw),
		SignatureOK: true,
		IPAddress:   clientIP(r),
		ActorID:     "gateway:midtrans",
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handlers) handleXendit(w http.ResponseWriter, r *http.Request, tenantID string, raw []byte) {
	token := r.Header.Get("x-callback-token")
	if err := webhook.VerifyXenditToken(h.WebhookSecret, token); err != nil {
		httpx.Error(w, r, apperrors.ErrSignatureInvalid401.WithDetail(map[string]any{"reason": err.Error()}))
		return
	}
	var p struct {
		ID         string `json:"id"`
		ExternalID string `json:"external_id"`
		Amount     int64  `json:"amount"`
		Status     string `json:"status"`
		BankCode   string `json:"bank_code"`
		PayerEmail string `json:"payer_email"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		httpx.Error(w, r, apperrors.Wrap(http.StatusBadRequest, "invalid_input", "bad xendit payload", err))
		return
	}
	channel := xenditChannel(p.BankCode)
	res, err := h.Settlement.ProcessCallback(r.Context(), usecase.WebhookInput{
		TenantID:    tenantID,
		Gateway:     "xendit",
		OrderID:     p.ExternalID,
		Status:      p.Status,
		GrossAmount: p.Amount,
		Channel:     channel,
		ExternalRef: p.ID,
		PayerName:   p.PayerEmail,
		PayerBank:   p.BankCode,
		RawPayload:  rawMap(raw),
		SignatureOK: true,
		IPAddress:   clientIP(r),
		ActorID:     "gateway:xendit",
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handlers) handleDirect(w http.ResponseWriter, r *http.Request, tenantID string, raw []byte) {
	var p struct {
		OrderID   string `json:"request_number"`
		Channel   string `json:"channel"`
		Amount    int64  `json:"amount"`
		PayerName string `json:"payer_name"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		httpx.Error(w, r, apperrors.Wrap(http.StatusBadRequest, "invalid_input", "bad direct payload", err))
		return
	}
	channel := transaction.Channel(strings.ToUpper(p.Channel))
	if !channel.Valid() {
		channel = transaction.ChannelManual
	}
	res, err := h.Settlement.ProcessCallback(r.Context(), usecase.WebhookInput{
		TenantID:    tenantID,
		Gateway:     "direct",
		OrderID:     p.OrderID,
		GrossAmount: p.Amount,
		Channel:     channel,
		ExternalRef: "sim-" + uuid.NewString(),
		PayerName:   p.PayerName,
		RawPayload:  rawMap(raw),
		SignatureOK: true,
		IPAddress:   clientIP(r),
		ActorID:     "simulator",
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// =============================================================================
// Simulator endpoint
// =============================================================================

type SimulatorSettleRequest struct {
	RequestNumber string `json:"request_number" validate:"required"`
	Channel       string `json:"channel"        validate:"required,oneof=VA_BCA VA_MANDIRI VA_BRI VA_BNI VA_PERMATA QRIS MANUAL_TRANSFER"`
	Amount        int64  `json:"amount"         validate:"required,gt=0"`
	PayerName     string `json:"payer_name"     validate:"omitempty,max=255"`
}

func (h *Handlers) SimulatorSettle(w http.ResponseWriter, r *http.Request) {
	var req SimulatorSettleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(&req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}
	channel := transaction.Channel(req.Channel)
	res, err := h.Settlement.ProcessCallback(r.Context(), usecase.WebhookInput{
		TenantID:    tenantFrom(r),
		Gateway:     "simulator",
		OrderID:     req.RequestNumber,
		GrossAmount: req.Amount,
		Channel:     channel,
		ExternalRef: "sim-" + uuid.NewString(),
		PayerName:   req.PayerName,
		RawPayload: map[string]any{
			"request_number": req.RequestNumber,
			"channel":         req.Channel,
			"amount":          req.Amount,
			"payer_name":      req.PayerName,
		},
		SignatureOK: true,
		IPAddress:   clientIP(r),
		ActorID:     actorIDFrom(r),
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
	c, err := h.Settlement.OutboxCounts(r.Context(), tenantFrom(r))
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

func readAll(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return buf, nil
			}
			return buf, err
		}
	}
}

func rawMap(b []byte) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	return m
}

func parseGrossAmount(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "."); i >= 0 {
		s = s[:i]
	}
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func midtransStatusCode(s string) string {
	switch strings.ToLower(s) {
	case "settlement", "capture":
		return "200"
	case "pending":
		return "201"
	case "deny", "cancel", "expire":
		return "407"
	default:
		return "404"
	}
}

func midtransChannel(paymentType, bank string) transaction.Channel {
	pt := strings.ToLower(paymentType)
	b := strings.ToLower(bank)
	switch pt {
	case "qris":
		return transaction.ChannelQRIS
	case "bank_transfer":
		switch b {
		case "bca":
			return transaction.ChannelVABCA
		case "mandiri":
			return transaction.ChannelVAMandiri
		case "bri":
			return transaction.ChannelVABRI
		case "bni":
			return transaction.ChannelVABNI
		case "permata":
			return transaction.ChannelVAPermata
		}
	}
	return transaction.ChannelManual
}

func xenditChannel(bankCode string) transaction.Channel {
	switch strings.ToLower(bankCode) {
	case "bca":
		return transaction.ChannelVABCA
	case "mandiri":
		return transaction.ChannelVAMandiri
	case "bri":
		return transaction.ChannelVABRI
	case "bni":
		return transaction.ChannelVABNI
	case "permata":
		return transaction.ChannelVAPermata
	case "qris":
		return transaction.ChannelQRIS
	}
	return transaction.ChannelManual
}