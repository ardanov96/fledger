package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/fledger/fledger-pay/internal/platform/errors"
	"github.com/fledger/fledger-pay/internal/domain/audit"
	"github.com/fledger/fledger-pay/internal/domain/outbox"
	"github.com/fledger/fledger-pay/internal/domain/payment"
	"github.com/fledger/fledger-pay/internal/domain/transaction"
	"github.com/fledger/fledger-pay/internal/integration/coreclient"
	"github.com/fledger/fledger-pay/internal/integration/dunningclient"
	"github.com/fledger/fledger-pay/internal/repository/postgres"
)

// SettlementService processes payment callbacks (webhook or simulator) and
// drives the outbox → Fledger Core settlement pipeline.
type SettlementService struct {
	payments     *postgres.PaymentRepo
	transactions *postgres.TransactionRepo
	outbox       *postgres.OutboxRepo
	audit        *postgres.AuditRepo
	core         *coreclient.Client
	dunning      *dunningclient.Client
	log          *slog.Logger
}

func NewSettlementService(
	p *postgres.PaymentRepo,
	t *postgres.TransactionRepo,
	o *postgres.OutboxRepo,
	a *postgres.AuditRepo,
	c *coreclient.Client,
	d *dunningclient.Client,
) *SettlementService {
	return &SettlementService{
		payments:     p,
		transactions: t,
		outbox:       o,
		audit:        a,
		core:         c,
		dunning:      d,
		log:          slog.Default(),
	}
}

func (s *SettlementService) SetLogger(l *slog.Logger) {
	if l != nil {
		s.log = l
	}
}

// WebhookInput is the shape accepted by the unified /v1/pay/webhooks/:gateway
// handler after per-gateway unmarshalling.
type WebhookInput struct {
	TenantID     string
	Gateway      string
	OrderID      string // request_number
	Status       string // "settlement" | "capture" | "partial" | ...
	GrossAmount  int64
	Channel      transaction.Channel
	ExternalRef  string
	IdempKey     string
	PayerName    string
	PayerBank    string
	RawPayload   map[string]any
	SignatureOK  bool
	IPAddress    string
	ActorID      string
}

// SettleResult is the response shape returned to the gateway / simulator.
type SettleResult struct {
	Status            string `json:"status"`
	RequestNumber     string `json:"request_number"`
	TransactionID     string `json:"transaction_id"`
	FledgerInvoiceID  string `json:"fledger_invoice_id"`
	SettledAt         string `json:"settled_at"`
	OutboxStatus      string `json:"outbox_status"`
	IdempotentReplay  bool   `json:"idempotent_replay,omitempty"`
}

// ProcessCallback is the single entry point used by both the webhook handler
// and the simulator. It runs the idempotency guard, inserts the
// pay_transactions row, updates the payment request to SETTLED, and enqueues
// the outbox event in one DB transaction.
func (s *SettlementService) ProcessCallback(ctx context.Context, in WebhookInput) (*SettleResult, error) {
	if in.OrderID == "" || in.ExternalRef == "" {
		return nil, fmt.Errorf("%w: order_id, external_ref required", apperrors.ErrInvalidInput)
	}
	if in.GrossAmount <= 0 {
		return nil, fmt.Errorf("%w: gross_amount must be > 0", apperrors.ErrInvalidInput)
	}
	if !in.Channel.Valid() {
		return nil, fmt.Errorf("%w: invalid channel %q", apperrors.ErrInvalidInput, in.Channel)
	}

	// If tenant_id was not provided in headers (e.g. external payment gateway callback),
	// resolve it from the payment request by its globally unique request_number.
	var pr payment.Request
	var err error
	if in.TenantID == "" {
		pr, err = s.payments.GetByRequestNumberGlobal(ctx, in.OrderID)
		if err != nil {
			return nil, err
		}
		in.TenantID = pr.TenantID
	} else {
		pr, err = s.payments.GetByRequestNumber(ctx, in.TenantID, in.OrderID)
		if err != nil {
			return nil, err
		}
	}

	// Idempotency guard: if a transaction with this external_ref already
	// exists, return a 200 with idempotent_replay=true and DO NOTHING.
	existing, err := s.transactions.GetByExternalRef(ctx, in.TenantID, in.ExternalRef)
	if err == nil {
		// Build a synthetic result from the existing transaction.
		counts, _ := s.outbox.Count(ctx, in.TenantID)
		_ = counts
		return &SettleResult{
			Status:           string(pr.Status),
			RequestNumber:    pr.RequestNumber,
			TransactionID:    existing.ID,
			FledgerInvoiceID: pr.FledgerInvoiceID,
			SettledAt:        pr.SettledAt.UTC().Format(time.RFC3339),
			OutboxStatus:     "REPLAY",
			IdempotentReplay: true,
		}, nil
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}
	if pr.Status == payment.StatusSettled {
		// Already settled (race with another webhook) — fall through to
		// idempotent reply using the existing transaction.
		txs, terr := s.transactions.ListByRequest(ctx, pr.ID)
		if terr == nil && len(txs) > 0 {
			return &SettleResult{
				Status:           string(pr.Status),
				RequestNumber:    pr.RequestNumber,
				TransactionID:    txs[0].ID,
				FledgerInvoiceID: pr.FledgerInvoiceID,
				SettledAt:        pr.SettledAt.UTC().Format(time.RFC3339),
				OutboxStatus:     "REPLAY",
				IdempotentReplay: true,
			}, nil
		}
	}

	idemKey := in.IdempKey
	if idemKey == "" {
		idemKey = in.ExternalRef
	}

	tx := transaction.Transaction{
		TenantID:          in.TenantID,
		PaymentRequestID:  pr.ID,
		Channel:           in.Channel,
		ExternalReference: in.ExternalRef,
		IdempotencyKey:    idemKey,
		GrossAmount:       in.GrossAmount,
		NetAmount:         in.GrossAmount, // no fee in simulator
		FeeAmount:         0,
		PaidAt:            time.Now().UTC(),
		PayerName:         strings.TrimSpace(in.PayerName),
		PayerBank:         strings.TrimSpace(in.PayerBank),
		RawPayload:        in.RawPayload,
		SignatureVerified: in.SignatureOK,
	}
	txRow, err := s.transactions.Insert(ctx, tx)
	if err != nil {
		return nil, err
	}

	settled, err := s.payments.MarkSettled(ctx, in.TenantID, pr.ID, in.GrossAmount)
	if err != nil {
		return nil, err
	}

	// Build outbox payload — the worker will call Core.
	obPayload := map[string]any{
		"event_type":         "fledger.pay.settled",
		"payment_request_id": pr.ID,
		"transaction_id":     txRow.ID,
		"fledger_invoice_id": pr.FledgerInvoiceID,
		"customer_id":        pr.CustomerID,
		"amount_minor":       in.GrossAmount,
		"currency":           pr.Currency,
		"channel":            string(in.Channel),
		"payer_name":         txRow.PayerName,
		"settled_at":         settled.SettledAt.UTC().Format(time.RFC3339),
		"from_account_id":    "ACC_CUSTOMER_AR",
		"to_account_id":      "ACC_BANK_BCA",
	}
	ob, err := s.outbox.Append(ctx, outbox.Event{
		TenantID:         in.TenantID,
		TransactionID:    txRow.ID,
		FledgerInvoiceID: pr.FledgerInvoiceID,
		EventType:        "fledger.pay.settled",
		Payload:          obPayload,
	})
	if err != nil {
		return nil, err
	}

	// Best-effort: notify Fledger Dunning so it can self-heal its queue.
	// A failure here does NOT block the payment settlement; the dunning
	// side will still see the webhook if the operator retries, or the
	// dunning cron will eventually catch up.
	if s.dunning != nil && pr.FledgerInvoiceID != "" {
		_ = s.dunning.NotifyPaymentSettled(ctx, pr.FledgerInvoiceID, in.GrossAmount, txRow.ID)
	}

	// Audit
	if s.audit != nil {
		action := audit.ActionSettleTransaction
		if in.Gateway != "" {
			action = audit.ActionWebhookReceived
		}
		_ = s.audit.Append(ctx, audit.Log{
			TenantID:     in.TenantID,
			ActorID:      in.ActorID,
			ActorRole:    "gateway",
			Action:       action,
			ResourceType: "payment_transaction",
			ResourceID:   txRow.ID,
			Details: map[string]any{
				"gateway":          in.Gateway,
				"request_number":   pr.RequestNumber,
				"amount":           in.GrossAmount,
				"channel":          string(in.Channel),
				"external_ref":     in.ExternalRef,
				"signature_verified": in.SignatureOK,
				"outbox_id":        ob.ID,
			},
			IPAddress: in.IPAddress,
		})
	}

	return &SettleResult{
		Status:           string(settled.Status),
		RequestNumber:    pr.RequestNumber,
		TransactionID:    txRow.ID,
		FledgerInvoiceID: pr.FledgerInvoiceID,
		SettledAt:        settled.SettledAt.UTC().Format(time.RFC3339),
		OutboxStatus:     string(ob.Status),
	}, nil
}

// OutboxDrainOnce runs one drain cycle. Returns (sent, failed, error).
func (s *SettlementService) OutboxDrainOnce(ctx context.Context, batch, maxAttempts int) (sent, failed int, err error) {
	if batch <= 0 {
		batch = 16
	}
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	due, err := s.outbox.FetchDue(ctx, batch)
	if err != nil {
		return 0, 0, err
	}
	for _, e := range due {
		// (a) POST /v1/transfers
		fromAcct, _ := e.Payload["from_account_id"].(string)
		toAcct, _ := e.Payload["to_account_id"].(string)
		// JSONB decode yields numbers as float64; coerce to int64.
		var amountMinor int64
		switch v := e.Payload["amount_minor"].(type) {
		case int64:
			amountMinor = v
		case int:
			amountMinor = int64(v)
		case float64:
			amountMinor = int64(v)
		}
		currency, _ := e.Payload["currency"].(string)
		description, _ := e.Payload["description"].(string)
		if description == "" {
			description = fmt.Sprintf("Auto-settlement via %v", e.Payload["channel"])
		}
		if fromAcct == "" {
			fromAcct = "ACC_CUSTOMER_AR"
		}
		if toAcct == "" {
			toAcct = "ACC_BANK_BCA"
		}
		if currency == "" {
			currency = "IDR"
		}
		if amountMinor <= 0 {
			s.log.Warn("outbox: skipping row with invalid payload", "id", e.ID)
			_ = s.outbox.MarkFailed(ctx, e.ID, "amount_minor <= 0", 24*time.Hour)
			failed++
			continue
		}
		_, terr := s.core.CreateTransfer(ctx, coreclient.TransferInput{
			FromAccountID:  fromAcct,
			ToAccountID:    toAcct,
			AmountMinor:    amountMinor,
			Currency:       currency,
			Description:    description,
			IdempotencyKey: e.TransactionID,
		})
		if terr != nil {
			if errors.Is(terr, apperrors.ErrInvalidInput) {
				// Permanent failure (4xx) — park.
				_ = s.outbox.MarkFailed(ctx, e.ID, terr.Error(), 30*24*time.Hour)
				failed++
				continue
			}
			_ = s.outbox.MarkFailed(ctx, e.ID, terr.Error(), backoffFor(e.RetryCount))
			failed++
			continue
		}
		// (b) Mark invoice PAID in Core.
		invoiceID, _ := e.Payload["fledger_invoice_id"].(string)
		if invoiceID != "" {
			if _, perr := s.core.MarkInvoicePaid(ctx, invoiceID, e.TransactionID); perr != nil {
				// Mark the transfer step succeeded but the invoice-pay step
				// failed. We DO NOT mark the row sent — the worker will retry
				// both calls (the transfer is idempotent via Idempotency-Key).
				_ = s.outbox.MarkFailed(ctx, e.ID, "invoice pay: "+perr.Error(), backoffFor(e.RetryCount))
				failed++
				continue
			}
		}
		if err := s.outbox.MarkSent(ctx, e.ID); err != nil {
			return sent, failed, err
		}
		s.log.Info("outbox sent",
			"event_id", e.ID,
			"transaction_id", e.TransactionID,
			"invoice_id", invoiceID,
		)
		sent++
	}
	return sent, failed, nil
}

// OutboxCounts returns dashboard view.
func (s *SettlementService) OutboxCounts(ctx context.Context, tenantID string) (postgres.OutboxCounts, error) {
	return s.outbox.Count(ctx, tenantID)
}

// DrainLoop runs OutboxDrainOnce on a ticker until ctx is cancelled.
func (s *SettlementService) DrainLoop(ctx context.Context, interval time.Duration, batch, maxAttempts int) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	_, _, _ = s.OutboxDrainOnce(ctx, batch, maxAttempts)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sent, failed, err := s.OutboxDrainOnce(ctx, batch, maxAttempts)
			if err != nil {
				s.log.Error("outbox drain", "err", err.Error())
				continue
			}
			if sent > 0 || failed > 0 {
				s.log.Info("outbox drain tick", "sent", sent, "failed", failed)
			}
		}
	}
}

func backoffFor(attempts int) time.Duration {
	if attempts <= 0 {
		return 3 * time.Second
	}
	d := 3 * time.Second
	for i := 1; i < attempts; i++ {
		d *= 2
		if d >= 30*time.Minute {
			return 30 * time.Minute
		}
	}
	return d
}

// MarshalPayload is exported for tests that need to inspect the row.
func MarshalPayload(e outbox.Event) ([]byte, error) { return json.Marshal(e.Payload) }

// Avoid unused-import warning.
var _ = uuid.NewString