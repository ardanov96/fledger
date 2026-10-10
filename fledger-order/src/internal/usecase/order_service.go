// Package usecase — order service (Sprint 3 + 4).
//
// Lifecycle:
//   1. CreateOrder: validate items, look up tier price, reserve stock, insert
//      order + items in one DB tx, return order in PENDING_CREDIT_CHECK.
//   2. EvaluateCredit: call Fledger Core AR + aging. If both rules pass, set
//      status=APPROVED + enqueue the dispatch outbox event. Otherwise
//      status=CREDIT_BLOCKED.
//   3. DispatchFleet: outbox worker drains the row -> POST to Fledger Fleet ->
//      store fleet_do_id and flip status=DISPATCHED_TO_FLEET.
//   4. OverrideCredit: Finance Manager unlocks with the configured PIN.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/fledger/fledger-order/internal/platform/errors"
	"github.com/fledger/fledger-order/internal/domain/audit"
	"github.com/fledger/fledger-order/internal/domain/order"
	"github.com/fledger/fledger-order/internal/domain/outbox"
	"github.com/fledger/fledger-order/internal/domain/pricing"
	"github.com/fledger/fledger-order/internal/integration/coreclient"
	"github.com/fledger/fledger-order/internal/integration/fleetclient"
	"github.com/fledger/fledger-order/internal/platform/credit"
	"github.com/fledger/fledger-order/internal/repository/postgres"
)

// OrderService is the central orchestrator.
type OrderService struct {
	orders     *postgres.OrderRepo
	products   *postgres.ProductRepo
	prices     *postgres.PricingRepo
	inventory  *postgres.InventoryRepo
	outbox     *postgres.OutboxRepo
	audit      *postgres.AuditRepo
	core       *coreclient.Client
	fleet      *fleetclient.Client
	evaluator  *credit.Evaluator
	overridePIN string
	log        *slog.Logger
}

func NewOrderService(
	orders *postgres.OrderRepo, products *postgres.ProductRepo, prices *postgres.PricingRepo,
	inv *postgres.InventoryRepo, ob *postgres.OutboxRepo, audit *postgres.AuditRepo,
	core *coreclient.Client, fleet *fleetclient.Client, evaluator *credit.Evaluator,
	overridePIN string,
) *OrderService {
	return &OrderService{
		orders: orders, products: products, prices: prices, inventory: inv, outbox: ob, audit: audit,
		core: core, fleet: fleet, evaluator: evaluator, overridePIN: overridePIN,
		log: slog.Default(),
	}
}

func (s *OrderService) SetLogger(l *slog.Logger) { if l != nil { s.log = l } }

// CreateInput bundles the body of POST /v1/order/orders.
type CreateInput struct {
	TenantID          string
	CustomerID        string
	CustomerName      string
	CustomerTier      pricing.Tier
	CustomerPhone     string
	DestinationAddress string
	Items             []CreateInputItem
	Notes             string
	ActorID           string
	IPAddress         string
}

type CreateInputItem struct {
	ProductID string
	Quantity  int
}

type CreateResult struct {
	Order order.Order   `json:"order"`
	Items []order.Item  `json:"items"`
}

// Create reserves stock, prices items via the tier table, and inserts the
// order header + items in one DB transaction. The order is left in
// PENDING_CREDIT_CHECK — only EvaluateCredit() decides APPROVED.
func (s *OrderService) Create(ctx context.Context, in CreateInput) (*CreateResult, error) {
	if _, err := uuid.Parse(in.CustomerID); err != nil {
		return nil, fmt.Errorf("%w: customer_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if strings.TrimSpace(in.CustomerName) == "" {
		return nil, fmt.Errorf("%w: customer_name required", apperrors.ErrInvalidInput)
	}
	if !in.CustomerTier.Valid() {
		return nil, fmt.Errorf("%w: invalid customer_tier %q", apperrors.ErrInvalidInput, in.CustomerTier)
	}
	if strings.TrimSpace(in.DestinationAddress) == "" {
		return nil, fmt.Errorf("%w: destination_address required", apperrors.ErrInvalidInput)
	}
	if len(in.Items) == 0 {
		return nil, fmt.Errorf("%w: at least one item required", apperrors.ErrInvalidInput)
	}
	orderNumber := GenerateOrderNumber(time.Now().UTC())

	// Resolve prices and reserve stock per item.
	now := time.Now().UTC()
	items := make([]order.Item, 0, len(in.Items))
	for _, it := range in.Items {
		if _, err := uuid.Parse(it.ProductID); err != nil {
			return nil, fmt.Errorf("%w: items.product_id must be a valid UUID", apperrors.ErrInvalidInput)
		}
		if it.Quantity <= 0 {
			return nil, fmt.Errorf("%w: items.quantity must be > 0", apperrors.ErrInvalidInput)
		}
		prod, err := s.products.Get(ctx, in.TenantID, it.ProductID)
		if err != nil {
			return nil, err
		}
		// Find the best (highest min_qty <= requested) price tier.
		tier, err := s.prices.FindBestPrice(ctx, in.TenantID, it.ProductID, in.CustomerTier, it.Quantity)
		if err != nil {
			return nil, fmt.Errorf("price for product %s / tier %s: %w", it.ProductID, in.CustomerTier, err)
		}
		line := order.Item{
			TenantID:    in.TenantID,
			ProductID:   prod.ID,
			SKU:         prod.SKU,
			Name:        prod.Name,
			Quantity:    it.Quantity,
			UnitPrice:   tier.UnitPrice,
			LineTotal:   tier.UnitPrice * int64(it.Quantity),
			WeightGrams: prod.WeightGrams * it.Quantity,
		}
		items = append(items, line)
	}

	// Reserve stock per item. We use a single warehouse "WH-CENTRAL-01".
	const warehouse = "WH-CENTRAL-01"
	// Run the reservations first; if any fails, release the already-reserved
	// ones to keep the system idempotent.
	reserved := make([]order.Item, 0, len(items))
	released := false
	deferFn := func() {
		if released {
			return
		}
		for _, r := range reserved {
			_, _ = s.inventory.Release(context.Background(), in.TenantID, r.ProductID, warehouse, r.Quantity)
		}
	}
	for _, line := range items {
		if _, err := s.inventory.Reserve(ctx, in.TenantID, line.ProductID, warehouse, line.Quantity); err != nil {
			deferFn()
			return nil, err
		}
		reserved = append(reserved, line)
	}
	released = true // success — leave reservations in place

	var subtotal int64
	var totalWeightGrams int
	for _, it := range items {
		subtotal += it.LineTotal
		totalWeightGrams += it.WeightGrams
	}

	o := order.Order{
		TenantID:           in.TenantID,
		OrderNumber:        orderNumber,
		CustomerID:         in.CustomerID,
		CustomerName:       strings.TrimSpace(in.CustomerName),
		CustomerTier:       string(in.CustomerTier),
		CustomerPhone:      strings.TrimSpace(in.CustomerPhone),
		DestinationAddress: strings.TrimSpace(in.DestinationAddress),
		TotalWeightKg:      totalWeightGrams / 1000,
		Subtotal:           subtotal,
		Discount:           0,
		TotalAmount:        subtotal,
		Status:             order.StatusPendingCreditCheck,
		CreditGateStatus:   order.CreditUnevaluated,
		CreditCheckDetails: map[string]any{},
		Notes:              in.Notes,
	}
	// Persist the order header + items in one tx.
	created, persisted, err := s.orders.CreateWithItems(ctx, o, items)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Append(ctx, audit.Log{
		TenantID:     in.TenantID,
		ActorID:      in.ActorID,
		ActorRole:    "salesman",
		Action:       audit.ActionCreateOrder,
		ResourceType: "order",
		ResourceID:   created.ID,
		Details: map[string]any{
			"order_number":  created.OrderNumber,
			"subtotal":      created.Subtotal,
			"items":         len(persisted),
			"total_weight_kg": created.TotalWeightKg,
		},
		IPAddress: in.IPAddress,
	})
	_ = now
	return &CreateResult{Order: created, Items: persisted}, nil
}

// EvaluateInput bundles the body of POST /v1/order/orders/:id/evaluate-credit.
type EvaluateInput struct {
	TenantID  string
	OrderID   string
	ActorID   string
	IPAddress string
}

// EvaluateResult is the response shape.
type EvaluateResult struct {
	OrderID         string                 `json:"order_id"`
	OrderStatus     order.Status           `json:"order_status"`
	CreditGateStatus order.CreditGateStatus `json:"credit_gate_status"`
	Evaluation     credit.Decision        `json:"evaluation"`
}

// EvaluateCredit runs the Hard Credit Gate and either APPROVES (and enqueues
// the dispatch outbox event) or BLOCKS the order.
func (s *OrderService) EvaluateCredit(ctx context.Context, in EvaluateInput) (*EvaluateResult, error) {
	if _, err := uuid.Parse(in.OrderID); err != nil {
		return nil, fmt.Errorf("%w: invalid order id", apperrors.ErrInvalidInput)
	}
	o, err := s.orders.Get(ctx, in.TenantID, in.OrderID)
	if err != nil {
		return nil, err
	}
	if o.Status != order.StatusPendingCreditCheck {
		return nil, fmt.Errorf("%w: order is not in PENDING_CREDIT_CHECK (current=%s)",
			apperrors.ErrConflict, o.Status)
	}

	decision, err := s.evaluator.Evaluate(ctx, o.CustomerID, o.TotalAmount)
	if err != nil {
		return nil, fmt.Errorf("evaluate credit: %w", err)
	}

	details := map[string]any{
		"decision":    decision,
		"evaluated_at": time.Now().UTC().Format(time.RFC3339),
		"actor_id":     in.ActorID,
	}
	var newStatus order.Status
	var cgs order.CreditGateStatus
	if decision.Pass {
		newStatus = order.StatusApproved
		cgs = order.CreditPassed
	} else {
		newStatus = order.StatusCreditBlocked
		cgs = creditGateFromDecision(decision)
	}

	_, err = s.orders.UpdateStatus(ctx, in.TenantID, in.OrderID, newStatus, map[string]any{
		"credit_gate_status": string(cgs),
		"evaluation":         decision,
		"evaluated_at":       time.Now().UTC().Format(time.RFC3339),
		"actor_id":           in.ActorID,
	})
	if err != nil {
		return nil, err
	}

	action := audit.ActionCreditBlocked
	if decision.Pass {
		action = audit.ActionCreditPassed
	}
	_ = s.audit.Append(ctx, audit.Log{
		TenantID:     in.TenantID,
		ActorID:      in.ActorID,
		ActorRole:    "system",
		Action:       action,
		ResourceType: "order",
		ResourceID:   o.ID,
		Details:      details,
		IPAddress:    in.IPAddress,
	})

	// If approved, enqueue the dispatch outbox event (Sprint 4).
	if decision.Pass {
		items, err := s.orders.ListItems(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		ob, err := s.outbox.Append(ctx, outbox.Event{
			TenantID:    in.TenantID,
			EventType:   outbox.EventDispatchedFleet,
			AggregateID: o.ID,
			Payload: map[string]any{
				"order_id":             o.ID,
				"order_number":         o.OrderNumber,
				"customer_id":          o.CustomerID,
				"customer_name":        o.CustomerName,
				"destination_address":  o.DestinationAddress,
				"total_nominal":        o.TotalAmount,
				"total_weight_kg":      o.TotalWeightKg,
				"items":                orderItemsForFleet(items),
				"idempotency_key":      o.ID,
			},
		})
		if err != nil {
			return nil, err
		}
		_ = ob
	}

	return &EvaluateResult{
		OrderID:         o.ID,
		OrderStatus:     newStatus,
		CreditGateStatus: cgs,
		Evaluation:     decision,
	}, nil
}

func creditGateFromDecision(d credit.Decision) order.CreditGateStatus {
	if d.HasOverdue30d {
		return order.CreditOverdueBlocked
	}
	return order.CreditLimitExceeded
}

// OverrideInput bundles the body of POST /v1/order/orders/:id/override-credit.
type OverrideInput struct {
	TenantID  string
	OrderID   string
	OverridePIN string
	Reason     string
	ActorID    string
	IPAddress  string
}

// OverrideCredit unlocks a CREDIT_BLOCKED order using the configured PIN.
func (s *OrderService) OverrideCredit(ctx context.Context, in OverrideInput) (*order.Order, error) {
	if _, err := uuid.Parse(in.OrderID); err != nil {
		return nil, fmt.Errorf("%w: invalid order id", apperrors.ErrInvalidInput)
	}
	if s.overridePIN == "" || in.OverridePIN != s.overridePIN {
		return nil, apperrors.ErrOverridePinInvalid
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, fmt.Errorf("%w: reason required for override", apperrors.ErrInvalidInput)
	}
	o, err := s.orders.Get(ctx, in.TenantID, in.OrderID)
	if err != nil {
		return nil, err
	}
	if o.Status != order.StatusCreditBlocked {
		return nil, fmt.Errorf("%w: order is not CREDIT_BLOCKED (current=%s)",
			apperrors.ErrConflict, o.Status)
	}
	updated, err := s.orders.ApplyOverride(ctx, in.TenantID, in.OrderID, in.ActorID, in.Reason, map[string]any{
		"override_reason": in.Reason,
		"override_at":     time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}
	_ = s.audit.Append(ctx, audit.Log{
		TenantID:     in.TenantID,
		ActorID:      in.ActorID,
		ActorRole:    "finance_manager",
		Action:       audit.ActionOverrideApproved,
		ResourceType: "order",
		ResourceID:   o.ID,
		Details: map[string]any{
			"override_reason":  in.Reason,
			"previous_status":  string(o.Status),
			"new_status":       string(updated.Status),
		},
		IPAddress: in.IPAddress,
	})

	// Enqueue dispatch after override.
	items, err := s.orders.ListItems(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	_, err = s.outbox.Append(ctx, outbox.Event{
		TenantID:    in.TenantID,
		EventType:   outbox.EventDispatchedFleet,
		AggregateID: o.ID,
		Payload: map[string]any{
			"order_id":            o.ID,
			"order_number":        o.OrderNumber,
			"customer_id":         o.CustomerID,
			"customer_name":       o.CustomerName,
			"destination_address": o.DestinationAddress,
			"total_nominal":       o.TotalAmount,
			"total_weight_kg":     o.TotalWeightKg,
			"items":               orderItemsForFleet(items),
			"idempotency_key":     o.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// Get returns one order with its items.
func (s *OrderService) Get(ctx context.Context, tenantID, id string) (order.Order, []order.Item, error) {
	o, err := s.orders.Get(ctx, tenantID, id)
	if err != nil {
		return order.Order{}, nil, err
	}
	items, err := s.orders.ListItems(ctx, o.ID)
	if err != nil {
		return order.Order{}, nil, err
	}
	return o, items, nil
}

func (s *OrderService) List(ctx context.Context, tenantID, status string) ([]order.Order, error) {
	return s.orders.List(ctx, tenantID, status)
}

// CancelInput cancels a PENDING / BLOCKED order and releases the reservations.
type CancelInput struct {
	TenantID  string
	OrderID   string
	ActorID   string
	IPAddress string
}

func (s *OrderService) Cancel(ctx context.Context, in CancelInput) (*order.Order, error) {
	if _, err := uuid.Parse(in.OrderID); err != nil {
		return nil, fmt.Errorf("%w: invalid order id", apperrors.ErrInvalidInput)
	}
	o, items, err := s.Get(ctx, in.TenantID, in.OrderID)
	if err != nil {
		return nil, err
	}
	if o.Status == order.StatusDispatchedToFleet || o.Status == order.StatusCompleted {
		return nil, fmt.Errorf("%w: order already %s, cannot cancel", apperrors.ErrConflict, o.Status)
	}
	const warehouse = "WH-CENTRAL-01"
	for _, it := range items {
		_, _ = s.inventory.Release(ctx, in.TenantID, it.ProductID, warehouse, it.Quantity)
	}
	updated, err := s.orders.UpdateStatus(ctx, in.TenantID, in.OrderID, order.StatusCancelled, nil)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Append(ctx, audit.Log{
		TenantID:     in.TenantID,
		ActorID:      in.ActorID,
		ActorRole:    "salesman",
		Action:       audit.ActionCreateOrder, // reuse CREATE_ORDER for cancel audit
		ResourceType: "order",
		ResourceID:   o.ID,
		Details:      map[string]any{"event": "CANCELLED", "previous_status": string(o.Status)},
		IPAddress:    in.IPAddress,
	})
	return &updated, nil
}

// orderItemsForFleet converts order.Item into the fleet payload shape.
func orderItemsForFleet(items []order.Item) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"sku_id":    it.SKU,
			"name":      it.Name,
			"quantity":  it.Quantity,
			"unit_price": it.UnitPrice,
		})
	}
	return out
}

// =============================================================================
// Outbox drain (Sprint 4).
// =============================================================================

// OutboxCounts returns the dashboard view.
func (s *OrderService) OutboxCounts(ctx context.Context, tenantID string) (postgres.OutboxCounts, error) {
	return s.outbox.Count(ctx, tenantID)
}

// DrainOutbox runs one drain cycle.
func (s *OrderService) DrainOutbox(ctx context.Context, batch, maxAttempts int) (sent, failed int, err error) {
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
		if err := s.dispatchOne(ctx, e, maxAttempts); err != nil {
			failed++
			continue
		}
		sent++
	}
	return sent, failed, nil
}

// dispatchOne posts the DO to Fledger Fleet and updates the order row.
func (s *OrderService) dispatchOne(ctx context.Context, e outbox.Event, maxAttempts int) error {
	orderID, _ := e.Payload["order_id"].(string)
	if orderID == "" {
		_ = s.outbox.MarkFailed(ctx, e.ID, "missing order_id in payload", 24*time.Hour)
		return errors.New("missing order_id")
	}
	idem, _ := e.Payload["idempotency_key"].(string)
	customerName, _ := e.Payload["customer_name"].(string)
	destAddr, _ := e.Payload["destination_address"].(string)
	totalNom, _ := numericField(e.Payload, "total_nominal")

	itemsRaw, _ := e.Payload["items"].([]any)
	items := make([]fleetclient.DeliveryItem, 0, len(itemsRaw))
	for _, raw := range itemsRaw {
		m, _ := raw.(map[string]any)
		if m == nil {
			continue
		}
		qty, _ := numericField(m, "quantity")
		up, _ := numericField(m, "unit_price")
		items = append(items, fleetclient.DeliveryItem{
			SkuID:     stringField(m, "sku_id"),
			Name:      stringField(m, "name"),
			Quantity:  int(qty),
			UnitPrice: up,
		})
	}
	weightKg, _ := numericField(e.Payload, "total_weight_kg")

	doIn := fleetclient.DeliveryOrderInput{
		DoNumber:           stringField(e.Payload, "order_number"),
		CustomerName:       customerName,
		DestinationAddress: destAddr,
		TotalNominal:       totalNom,
		TotalWeightKg:      int(weightKg),
		Items:              items,
	}
	resp, err := s.fleet.CreateDeliveryOrder(ctx, doIn, idem)
	if err != nil {
		// Distinguish permanent vs transient: if upstream returns 4xx we
		// permanently fail; 5xx/network → retry.
		if errors.Is(err, apperrors.ErrInvalidInput) {
			_ = s.outbox.MarkFailed(ctx, e.ID, err.Error(), 30*24*time.Hour)
		} else {
			_ = s.outbox.MarkFailed(ctx, e.ID, err.Error(), backoffForOutbox(e.RetryCount))
		}
		return err
	}
	if _, err := s.orders.SetFleetDOID(ctx, e.TenantID, orderID, resp.ID); err != nil {
		s.log.Warn("set fleet DOID failed", "err", err.Error(), "order_id", orderID)
	}
	if err := s.outbox.MarkSent(ctx, e.ID); err != nil {
		return err
	}
	_ = s.audit.Append(ctx, audit.Log{
		TenantID:     e.TenantID,
		ActorID:      "outbox-worker",
		ActorRole:    "system",
		Action:       audit.ActionDispatchedFleet,
		ResourceType: "order",
		ResourceID:   uuidFromString(orderID),
		Details: map[string]any{
			"fleet_do_id":    resp.ID,
			"fleet_do_number": resp.DoNumber,
		},
	})
	return nil
}

// DrainLoop runs DrainOutbox on a ticker.
func (s *OrderService) DrainLoop(ctx context.Context, interval time.Duration, batch, maxAttempts int) {
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

func backoffForOutbox(attempts int) time.Duration {
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

func stringField(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func numericField(m map[string]any, k string) (int64, error) {
	v, ok := m[k]
	if !ok {
		return 0, fmt.Errorf("missing %s", k)
	}
	switch x := v.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case float64:
		return int64(x), nil
	}
	return 0, fmt.Errorf("%s not numeric", k)
}

func uuidFromString(s string) string {
	u, _ := uuid.Parse(s)
	if u == uuid.Nil {
		return s
	}
	return u.String()
}