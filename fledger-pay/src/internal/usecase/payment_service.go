package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/fledger/fledger-pay/internal/platform/errors"
	"github.com/fledger/fledger-pay/internal/domain/audit"
	"github.com/fledger/fledger-pay/internal/domain/payment"
	"github.com/fledger/fledger-pay/internal/domain/qris"
	"github.com/fledger/fledger-pay/internal/domain/va"
	"github.com/fledger/fledger-pay/internal/repository/postgres"
)

// PaymentService orchestrates payment request creation, cancellation, and
// listing. It does NOT touch the outbox directly — that's the job of the
// SettlementService.
type PaymentService struct {
	payments *postgres.PaymentRepo
	audit    *postgres.AuditRepo
}

func NewPaymentService(p *postgres.PaymentRepo, a *postgres.AuditRepo) *PaymentService {
	return &PaymentService{payments: p, audit: a}
}

// CreateInput bundles the input of POST /v1/pay/requests.
type CreateInput struct {
	TenantID         string
	FledgerInvoiceID string
	CustomerID       string
	CustomerName     string
	CustomerPhone    string
	Amount           int64
	ExpiryMinutes    int
	EnabledBanks     []string
	EnableQRIS       bool
	MerchantCity     string
	ActorID          string
	IPAddress        string
}

// CreateResult is what the use case returns to the handler.
type CreateResult struct {
	Request         payment.Request `json:"request"`
	VirtualAccounts []va.Account    `json:"virtual_accounts"`
	QRIS            *qris.Code      `json:"qris,omitempty"`
}

// Create validates the input, generates VA numbers and (optionally) a QRIS
// string, and persists the bundle inside one DB transaction.
func (s *PaymentService) Create(ctx context.Context, in CreateInput) (*CreateResult, error) {
	if _, err := uuid.Parse(in.FledgerInvoiceID); err != nil {
		return nil, fmt.Errorf("%w: fledger_invoice_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(in.CustomerID); err != nil {
		return nil, fmt.Errorf("%w: customer_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if strings.TrimSpace(in.CustomerName) == "" {
		return nil, fmt.Errorf("%w: customer_name required", apperrors.ErrInvalidInput)
	}
	if in.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be > 0", apperrors.ErrInvalidInput)
	}
	if in.ExpiryMinutes <= 0 {
		in.ExpiryMinutes = 1440
	}
	if len(in.EnabledBanks) == 0 && !in.EnableQRIS {
		in.EnabledBanks = []string{"BCA"}
		in.EnableQRIS = true
	}
	if in.MerchantCity == "" {
		in.MerchantCity = "JAKARTA"
	}

	requestNumber := "PAY-" + time.Now().UTC().Format("200601") + "-" + strings.ToUpper(uuid.NewString()[:8])
	expiresAt := time.Now().UTC().Add(time.Duration(in.ExpiryMinutes) * time.Minute)

	req := payment.Request{
		TenantID:         in.TenantID,
		FledgerInvoiceID: in.FledgerInvoiceID,
		RequestNumber:    requestNumber,
		CustomerID:       in.CustomerID,
		CustomerName:     strings.TrimSpace(in.CustomerName),
		CustomerPhone:    strings.TrimSpace(in.CustomerPhone),
		Currency:         "IDR",
		Amount:           in.Amount,
		FeeAmount:        0,
		TotalAmount:      in.Amount,
		Status:           payment.StatusPending,
		ExpiresAt:        expiresAt,
		Metadata:         map[string]any{},
	}

	outVAs := make([]va.Account, 0, len(in.EnabledBanks))
	for _, b := range in.EnabledBanks {
		bank := va.BankCode(strings.ToUpper(strings.TrimSpace(b)))
		if !bank.Valid() {
			return nil, fmt.Errorf("%w: unsupported bank %q", apperrors.ErrInvalidInput, b)
		}
		outVAs = append(outVAs, va.Account{
			TenantID:       in.TenantID,
			BankCode:       bank,
			VANumber:       GenerateVANumber(bank, "PREVIEW-"+requestNumber, in.CustomerPhone),
			VAName:         VADisplayName(in.CustomerName),
			ExpectedAmount: in.Amount,
			Status:         "ACTIVE",
			ExpiresAt:      expiresAt,
		})
	}

	var qrCode *qris.Code
	if in.EnableQRIS {
		qrString := QRISPayload(in.MerchantCity, VADisplayName(in.CustomerName), in.Amount, requestNumber)
		qrCode = &qris.Code{
			TenantID:       in.TenantID,
			QRString:       qrString,
			ExpectedAmount: in.Amount,
			Status:         "ACTIVE",
			ExpiresAt:      expiresAt,
		}
	}

	created, vas, qr, err := s.payments.CreateBundle(ctx, req, outVAs, qrCode)
	if err != nil {
		return nil, err
	}

	if s.audit != nil {
		_ = s.audit.Append(ctx, audit.Log{
			TenantID:     in.TenantID,
			ActorID:      in.ActorID,
			ActorRole:    "user",
			Action:       audit.ActionCreatePayment,
			ResourceType: "payment_request",
			ResourceID:   created.ID,
			Details: map[string]any{
				"request_number": created.RequestNumber,
				"amount":         created.Amount,
				"enabled_banks":  in.EnabledBanks,
				"enable_qris":    in.EnableQRIS,
				"expiry_minutes": in.ExpiryMinutes,
			},
			IPAddress: in.IPAddress,
		})
	}

	return &CreateResult{Request: created, VirtualAccounts: vas, QRIS: qr}, nil
}

// Get returns one payment request.
func (s *PaymentService) Get(ctx context.Context, tenantID, id string) (*CreateResult, error) {
	req, err := s.payments.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	vas, err := s.payments.ListVAsByRequest(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	qr, err := s.payments.ListQRISByRequest(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return &CreateResult{Request: req, VirtualAccounts: vas, QRIS: qr}, nil
}

// List returns payment requests.
func (s *PaymentService) List(ctx context.Context, tenantID, status string) ([]payment.Request, error) {
	if status != "" && !payment.Status(status).Valid() {
		return nil, fmt.Errorf("%w: invalid status filter %q", apperrors.ErrInvalidInput, status)
	}
	return s.payments.List(ctx, tenantID, status)
}

// Cancel transitions a PENDING request to CANCELLED.
func (s *PaymentService) Cancel(ctx context.Context, tenantID, id, actorID, ip string) (payment.Request, error) {
	req, err := s.payments.Cancel(ctx, tenantID, id)
	if err != nil {
		return payment.Request{}, err
	}
	if s.audit != nil {
		_ = s.audit.Append(ctx, audit.Log{
			TenantID:     tenantID,
			ActorID:      actorID,
			ActorRole:    "user",
			Action:       audit.ActionCancelPayment,
			ResourceType: "payment_request",
			ResourceID:   id,
			IPAddress:    ip,
		})
	}
	return req, nil
}

// GetByRequestNumber is used by the simulator + webhook lookups.
func (s *PaymentService) GetByRequestNumber(ctx context.Context, tenantID, number string) (*CreateResult, error) {
	req, err := s.payments.GetByRequestNumber(ctx, tenantID, number)
	if err != nil {
		return nil, err
	}
	vas, err := s.payments.ListVAsByRequest(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	qr, err := s.payments.ListQRISByRequest(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return &CreateResult{Request: req, VirtualAccounts: vas, QRIS: qr}, nil
}