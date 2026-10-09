// Package usecase — collection + settlement service (Sprints 3 + 4).
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/fledger/fledger-force/internal/platform/errors"
	"github.com/fledger/fledger-force/internal/domain/audit"
	"github.com/fledger/fledger-force/internal/domain/collection"
	"github.com/fledger/fledger-force/internal/domain/outbox"
	"github.com/fledger/fledger-force/internal/domain/salesrep"
	"github.com/fledger/fledger-force/internal/domain/settlement"
	"github.com/fledger/fledger-force/internal/integration/coreclient"
	"github.com/fledger/fledger-force/internal/repository/postgres"
)

// CollectionService handles cash collections + EOD settlements.
type CollectionService struct {
	collections *postgres.CollectionRepo
	settlements *postgres.SettlementRepo
	reps        *postgres.RepRepo
	stores      *postgres.StoreRepo
	visits      *postgres.VisitRepo
	outbox      *postgres.OutboxRepo
	audit       *postgres.AuditRepo
	core        *coreclient.Client
	log         *slog.Logger
}

func NewCollectionService(
	colls *postgres.CollectionRepo, settles *postgres.SettlementRepo,
	reps *postgres.RepRepo, stores *postgres.StoreRepo, visits *postgres.VisitRepo,
	ob *postgres.OutboxRepo, a *postgres.AuditRepo, c *coreclient.Client,
) *CollectionService {
	return &CollectionService{
		collections: colls, settlements: settles,
		reps: reps, stores: stores, visits: visits,
		outbox: ob, audit: a, core: c,
		log: slog.Default(),
	}
}

func (s *CollectionService) SetLogger(l *slog.Logger) {
	if l != nil {
		s.log = l
	}
}

// CollectInput is the body of POST /v1/force/collections.
type CollectInput struct {
	TenantID         string
	VisitID          string
	SalesRepID       string
	StoreID          string
	FledgerInvoiceID string
	Amount           int64
	PayerName        string
	PayerPhone       string
	ActorID          string
	IPAddress        string
}

// CollectResult is the response shape.
type CollectResult struct {
	Collection         collection.Collection         `json:"collection"`
	SalesRepCurrentCash int64                       `json:"sales_rep_current_cash_held"`
	WAResceipt         WAResceiptPayload            `json:"wa_receipt_payload"`
}

// Collect validates the inputs, enforces max_cash_limit, persists the cash
// collection, increments the rep's current_cash_held, and enqueues the
// outbox event for Fledger Core (in one DB transaction).
func (s *CollectionService) Collect(ctx context.Context, in CollectInput) (*CollectResult, error) {
	if _, err := uuid.Parse(in.SalesRepID); err != nil {
		return nil, fmt.Errorf("%w: sales_rep_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(in.StoreID); err != nil {
		return nil, fmt.Errorf("%w: store_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(in.FledgerInvoiceID); err != nil {
		return nil, fmt.Errorf("%w: fledger_invoice_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if in.Amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be > 0", apperrors.ErrInvalidInput)
	}
	if strings.TrimSpace(in.PayerName) == "" {
		return nil, fmt.Errorf("%w: payer_name required", apperrors.ErrInvalidInput)
	}

	rep, err := s.reps.Get(ctx, in.TenantID, in.SalesRepID)
	if err != nil {
		return nil, err
	}
	if rep.Status == salesrep.StatusLocked {
		return nil, fmt.Errorf("%w: salesman terkunci (SETTLEMENT_LOCKED); selesaikan setoran terlebih dahulu",
			apperrors.ErrForbidden)
	}
	// Guard: if the new amount would push the rep past the max cash limit.
	if rep.CurrentCashHeld+in.Amount > rep.MaxCashLimit {
		return nil, apperrors.CashLimitExceeded(map[string]any{
			"current_cash_held": rep.CurrentCashHeld,
			"max_cash_limit":    rep.MaxCashLimit,
			"attempted_amount":  in.Amount,
		})
	}
	store, err := s.stores.Get(ctx, in.TenantID, in.StoreID)
	if err != nil {
		return nil, err
	}

	// Generate receipt.
	receiptNumber := GenerateReceiptNumber(time.Now().UTC())

	col := collection.Collection{
		TenantID:         in.TenantID,
		SalesRepID:       in.SalesRepID,
		StoreID:          in.StoreID,
		FledgerInvoiceID: in.FledgerInvoiceID,
		ReceiptNumber:    receiptNumber,
		Amount:           in.Amount,
		CollectedAt:      time.Now().UTC(),
		PayerName:        strings.TrimSpace(in.PayerName),
		PayerPhone:       strings.TrimSpace(in.PayerPhone),
		WAReceiptSent:    true, // we "send" the WA payload to the client which posts it
		Status:           collection.StatusHeldBySales,
	}
	if in.VisitID != "" {
		if _, err := uuid.Parse(in.VisitID); err == nil {
			col.VisitID = &in.VisitID
		}
	}

	tx, err := s.collections.Insert(ctx, col)
	if err != nil {
		return nil, err
	}

	updated, err := s.reps.IncrementCashHeld(ctx, in.TenantID, rep.ID, in.Amount)
	if err != nil {
		return nil, err
	}

	// Enqueue outbox.
	ob, err := s.outbox.Append(ctx, outbox.Event{
		TenantID:    in.TenantID,
		EventType:   outbox.EventCashCollected,
		AggregateID: tx.ID,
		Payload: map[string]any{
			"collection_id":     tx.ID,
			"receipt_number":    tx.ReceiptNumber,
			"fledger_invoice_id": tx.FledgerInvoiceID,
			"customer_id":       store.ID, // store id is treated as the customer AR account
			"from_account_id":   "ACC_CUSTOMER_AR",
			"to_account_id":     rep.FledgerWalletAccountID,
			"amount_minor":      tx.Amount,
			"currency":          "IDR",
			"channel":           "CASH_COLLECTION",
			"collected_at":      tx.CollectedAt.UTC().Format(time.RFC3339),
			"description":       fmt.Sprintf("Pembayaran kas lapangan %s (RCP-%s)", store.Name, receiptNumber),
		},
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Append(ctx, audit.Log{
		TenantID:     in.TenantID,
		ActorID:      in.ActorID,
		ActorRole:    "salesman",
		Action:       audit.ActionCashCollected,
		ResourceType: "cash_collection",
		ResourceID:   tx.ID,
		Details: map[string]any{
			"receipt_number": tx.ReceiptNumber,
			"amount":         tx.Amount,
			"outbox_id":      ob.ID,
		},
		IPAddress: in.IPAddress,
	})

	wa := BuildWAResceiptPayload(
		store.Name, in.PayerName, tx.ReceiptNumber, tx.FledgerInvoiceID,
		rep.EmployeeCode, rep.Name, tx.Amount,
	)
	if store.Phone != "" {
		wa.To = store.Phone
	}

	return &CollectResult{
		Collection:          tx,
		SalesRepCurrentCash: updated.CurrentCashHeld,
		WAResceipt:          wa,
	}, nil
}

// ListByRep returns the rep's collections (optionally filtered by status).
func (s *CollectionService) ListByRep(ctx context.Context, repID, status string) ([]collection.Collection, error) {
	return s.collections.ListByRep(ctx, repID, status)
}

// InquiryInput is the body of GET /v1/force/settlements/reps/:id/inquiry.
type InquiryResult struct {
	SalesRepID      string                  `json:"sales_rep_id"`
	SalesRepName    string                  `json:"sales_rep_name"`
	EmployeeCode    string                  `json:"employee_code"`
	TotalCashHeld   int64                   `json:"total_cash_held"`
	CollectionsCount int                    `json:"collections_count"`
	Collections     []collection.Collection `json:"collections"`
}

// Inquiry returns the rep's outstanding cash + the list of unsettled
// collections.
func (s *CollectionService) Inquiry(ctx context.Context, tenantID, repID string) (*InquiryResult, error) {
	rep, err := s.reps.Get(ctx, tenantID, repID)
	if err != nil {
		return nil, err
	}
	sum, count, err := s.collections.SumUnsettledByRep(ctx, repID)
	if err != nil {
		return nil, err
	}
	_ = sum
	colls, err := s.collections.ListByRep(ctx, repID, string(collection.StatusHeldBySales))
	if err != nil {
		return nil, err
	}
	return &InquiryResult{
		SalesRepID:       rep.ID,
		SalesRepName:     rep.Name,
		EmployeeCode:     rep.EmployeeCode,
		TotalCashHeld:    rep.CurrentCashHeld,
		CollectionsCount: count,
		Collections:      colls,
	}, nil
}

// SettleInput is the body of POST /v1/force/settlements.
type SettleInput struct {
	TenantID           string
	SalesRepID         string
	PhysicalCashReceived int64
	CashierNotes       string
	ActorID            string
	IPAddress          string
}

// SettleResult is the response shape.
type SettleResult struct {
	Settlement          settlement.Settlement `json:"settlement"`
	DiscrepancyAmount  int64                 `json:"discrepancy_amount"`
	SalesRepNewCashHeld int64                 `json:"sales_rep_new_cash_held"`
	SalesRepStatus      string                `json:"sales_rep_status"`
	OutboxStatus        string                `json:"outbox_status"`
}

// Settle processes the EOD cash deposit: creates the settlement row, marks
// the rep's collections as SETTLED_TO_HQ, resets the rep's cash to 0,
// restores their status to ACTIVE (or locks them if there's a discrepancy),
// and enqueues the outbox event for Fledger Core — all in one DB
// transaction (via the outbox).
func (s *CollectionService) Settle(ctx context.Context, in SettleInput) (*SettleResult, error) {
	if _, err := uuid.Parse(in.SalesRepID); err != nil {
		return nil, fmt.Errorf("%w: sales_rep_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if in.PhysicalCashReceived < 0 {
		return nil, fmt.Errorf("%w: physical_cash_received must be >= 0", apperrors.ErrInvalidInput)
	}

	rep, err := s.reps.Get(ctx, in.TenantID, in.SalesRepID)
	if err != nil {
		return nil, err
	}
	if rep.CurrentCashHeld == 0 {
		return nil, fmt.Errorf("%w: salesman tidak memiliki kas yang harus disetor", apperrors.ErrConflict)
	}

	systemCash := rep.CurrentCashHeld
	discrepancy := in.PhysicalCashReceived - systemCash

	status := settlement.StatusSettled
	repNewStatus := salesrep.StatusActive
	if discrepancy != 0 {
		status = settlement.StatusDiscrepancy
		repNewStatus = salesrep.StatusLocked
	}

	settleNumber := GenerateEODSettlementNumber(time.Now().UTC())
	set := settlement.Settlement{
		TenantID:           in.TenantID,
		SettlementNumber:   settleNumber,
		SalesRepID:         in.SalesRepID,
		CashierUserID:      in.ActorID,
		TotalSystemCash:    systemCash,
		TotalPhysicalCash:  in.PhysicalCashReceived,
		DiscrepancyAmount:  discrepancy,
		Status:             status,
		CashierNotes:       in.CashierNotes,
		SettledAt:          time.Now().UTC(),
	}
	saved, err := s.settlements.Insert(ctx, set)
	if err != nil {
		return nil, err
	}

	// Mark all HELD_BY_SALES collections as SETTLED_TO_HQ and reset rep cash.
	held, err := s.collections.ListByRep(ctx, rep.ID, string(collection.StatusHeldBySales))
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(held))
	for _, c := range held {
		ids = append(ids, c.ID)
	}
	if err := s.collections.MarkSettledToHQ(ctx, saved.ID, ids); err != nil {
		return nil, err
	}

	// Reset rep cash to 0 (the discrepancy still hangs as personal debt).
	if _, err := s.reps.IncrementCashHeld(ctx, in.TenantID, rep.ID, -systemCash); err != nil {
		return nil, err
	}
	updated, err := s.reps.SetStatus(ctx, in.TenantID, rep.ID, repNewStatus)
	if err != nil {
		return nil, err
	}

	// Outbox event (HQ cash receives the physical cash; salesman wallet settles
	// the system balance; on discrepancy the salesman keeps the delta as
	// remaining wallet debt).
	obPayload := map[string]any{
		"settlement_id":       saved.ID,
		"fledger_invoice_id":  nil,
		"from_account_id":     rep.FledgerWalletAccountID,
		"to_account_id":       "ACC_HQ_PHYSICAL_CASH",
		"amount_minor":        in.PhysicalCashReceived,
		"currency":            "IDR",
		"channel":             "EOD_SETTLEMENT",
		"system_cash":         systemCash,
		"physical_cash":       in.PhysicalCashReceived,
		"discrepancy_amount":  discrepancy,
		"description":         fmt.Sprintf("Setoran kas fisik EOD oleh %s (EOD-%s)", rep.Name, settleNumber),
	}
	ob, err := s.outbox.Append(ctx, outbox.Event{
		TenantID:    in.TenantID,
		EventType:   outbox.EventEODSettled,
		AggregateID: saved.ID,
		Payload:     obPayload,
	})
	if err != nil {
		return nil, err
	}

	action := audit.ActionEODSettled
	if status == settlement.StatusDiscrepancy {
		action = audit.ActionDiscrepancyFlagged
	}
	_ = s.audit.Append(ctx, audit.Log{
		TenantID:     in.TenantID,
		ActorID:      in.ActorID,
		ActorRole:    "cashier",
		Action:       action,
		ResourceType: "eod_settlement",
		ResourceID:   saved.ID,
		Details: map[string]any{
			"settlement_number": saved.SettlementNumber,
			"system_cash":       systemCash,
			"physical_cash":     in.PhysicalCashReceived,
			"discrepancy":       discrepancy,
			"outbox_id":         ob.ID,
		},
		IPAddress: in.IPAddress,
	})

	return &SettleResult{
		Settlement:          saved,
		DiscrepancyAmount:  discrepancy,
		SalesRepNewCashHeld: updated.CurrentCashHeld,
		SalesRepStatus:      string(updated.Status),
		OutboxStatus:        string(ob.Status),
	}, nil
}

// OutboxCounts returns the dashboard view.
func (s *CollectionService) OutboxCounts(ctx context.Context, tenantID string) (postgres.OutboxCounts, error) {
	return s.outbox.Count(ctx, tenantID)
}

// DrainOutbox runs one outbox drain cycle.
func (s *CollectionService) DrainOutbox(ctx context.Context, batch, maxAttempts int) (sent, failed int, err error) {
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
		fromAcct, _ := e.Payload["from_account_id"].(string)
		toAcct, _ := e.Payload["to_account_id"].(string)
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
		if currency == "" {
			currency = "IDR"
		}
		if fromAcct == "" || toAcct == "" {
			fromAcct = "ACC_CUSTOMER_AR"
			toAcct = repWalletFromPayload(e.Payload)
		}
		_, terr := s.core.CreateTransfer(ctx, coreclient.TransferInput{
			FromAccountID:  fromAcct,
			ToAccountID:    toAcct,
			AmountMinor:    amountMinor,
			Currency:       currency,
			Description:    description,
			IdempotencyKey: e.AggregateID,
		})
		if terr != nil {
			if errors.Is(terr, apperrors.ErrInvalidInput) {
				_ = s.outbox.MarkFailed(ctx, e.ID, terr.Error(), e.RetryCount, maxAttempts, 30*24*time.Hour)
				failed++
				continue
			}
			_ = s.outbox.MarkFailed(ctx, e.ID, terr.Error(), e.RetryCount, maxAttempts, backoffFor(e.RetryCount))
			failed++
			continue
		}
		if err := s.outbox.MarkSent(ctx, e.ID); err != nil {
			return sent, failed, err
		}
		s.log.Info("outbox sent",
			"event_id", e.ID,
			"event_type", e.EventType,
			"amount", amountMinor,
		)
		sent++
	}
	return sent, failed, nil
}

// DrainLoop runs DrainOutbox on a ticker.
func (s *CollectionService) DrainLoop(ctx context.Context, interval time.Duration, batch, maxAttempts int) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	_, _, _ = s.DrainOutbox(ctx, batch, maxAttempts)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sent, failed, err := s.DrainOutbox(ctx, batch, maxAttempts)
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

func repWalletFromPayload(p map[string]any) string {
	if v, ok := p["to_account_id"].(string); ok {
		return v
	}
	return "ACC_SALES_WALLET_UNKNOWN"
}
