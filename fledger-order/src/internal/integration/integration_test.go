package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fledger/fledger-order/internal/domain/pricing"
	"github.com/fledger/fledger-order/internal/integration/coreclient"
	"github.com/fledger/fledger-order/internal/integration/fleetclient"
	"github.com/fledger/fledger-order/internal/repository/postgres"
	"github.com/fledger/fledger-order/internal/usecase"

	"strings"
)

const testTenant = "00000000-0000-0000-0000-000000000001"

// rig bundles the wired-up services and a stub for Fledger Core + Fleet.
type rig struct {
	Pool      *pgxpool.Pool
	Services  *usecase.Services
	CoreSrv   *httptest.Server
	FleetSrv  *httptest.Server
	mu        sync.Mutex
	arSummary coreclient.ARSummary
	fleetDO   fleetclient.DeliveryOrderResponse
}

func setup(t *testing.T) *rig {
	t.Helper()
	if v := os.Getenv("FLEDGER_ORDER_TEST_DB"); v != "" {
		os.Setenv("DATABASE_URL", v)
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_order_test?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Skipf("test DB unavailable: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB ping failed: %v", err)
	}
	for _, table := range []string{
		"order_audit_logs", "order_outbox", "order_items", "order_orders",
		"order_inventory_stocks", "order_price_tiers", "order_products",
	} {
		_, _ = pool.Exec(context.Background(), "DELETE FROM "+table)
	}
	r := &rig{Pool: pool}

	// Stub Fledger Core: returns canned AR summary based on ?customer_id=
	r.arSummary = coreclient.ARSummary{
		CustomerID:         "",
		OutstandingARMinor: 8_500_000,
		CreditLimitMinor:   20_000_000,
		OverdueBucket:      coreclient.OverdueBucket{HasOverdue30d: false},
		OverdueDays:        0,
	}
	r.CoreSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Connection", "close")
		switch req.URL.Path {
		case "/v1/ping":
			w.WriteHeader(http.StatusOK)
			return
		}
		// /v1/customers/{id}/ar-summary
		if strings.HasPrefix(req.URL.Path, "/v1/customers/") && strings.HasSuffix(req.URL.Path, "/ar-summary") {
			custID := req.URL.Path[26 : len(req.URL.Path)-12]
			out := r.arSummary
			out.CustomerID = custID
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		// /v1/transfers (not used by order flow; kept for future parity)
		if req.Method == "POST" && req.URL.Path == "/v1/transfers" {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"transaction_id": uuid.NewString(), "status": "POSTED"})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Stub Fledger Fleet: returns a canned DO id.
	r.fleetDO = fleetclient.DeliveryOrderResponse{
		ID: uuid.NewString(), DoNumber: "DO-202610-00081", Status: "DISPATCHED", DispatchedAt: time.Now().UTC().Format(time.RFC3339),
	}
	r.FleetSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Connection", "close")
		if req.Method == "POST" && req.URL.Path == "/v1/fleet/delivery-orders" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(r.fleetDO)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	core := coreclient.NewClient(coreclient.Config{
		BaseURL: r.CoreSrv.URL, TenantID: testTenant, Timeout: 3 * time.Second,
	})
	fleet := fleetclient.NewClient(fleetclient.Config{
		BaseURL: r.FleetSrv.URL, TenantID: testTenant, Timeout: 3 * time.Second,
	})
	r.Services = usecase.NewServices(usecase.Deps{
		Pool: pool,
		Products: postgres.NewProductRepo(pool), Prices: postgres.NewPricingRepo(pool),
		Inventory: postgres.NewInventoryRepo(pool), Orders: postgres.NewOrderRepo(pool),
		Outbox: postgres.NewOutboxRepo(pool), Audit: postgres.NewAuditRepo(pool),
		Core: core, Fleet: fleet, OverridePIN: "992144",
	})
	t.Cleanup(func() { pool.Close(); r.CoreSrv.Close(); r.FleetSrv.Close() })
	return r
}

func mustProduct(t *testing.T, r *rig, sku, name string, weightGrams int) (string, *usecase.CreateResult) {
	t.Helper()
	p, err := r.Services.Product.Create(context.Background(), usecase.CreateProductInput{
		TenantID: testTenant, SKU: sku, Name: name, Category: "SEMBAKO", Unit: "DUS", WeightGrams: weightGrams,
	})
	require.NoError(t, err)
	return p.ID, nil
}

func mustPricing(t *testing.T, r *rig, productID, tier string, minQty int, price int64) {
	t.Helper()
	_, err := r.Services.Pricing.Create(context.Background(), usecase.CreatePricingInput{
		TenantID: testTenant, ProductID: productID, Tier: tierAsT(t, tier), MinQuantity: minQty, UnitPrice: price,
	})
	require.NoError(t, err)
}

func tierAsT(t *testing.T, s string) pricing.Tier {
	t.Helper()
	return pricing.Tier(s)
}

func mustStock(t *testing.T, r *rig, productID string, qty int) {
	t.Helper()
	_, err := r.Services.Inv.Adjust(context.Background(), usecase.AdjustStockInput{
		TenantID: testTenant, ProductID: productID, WarehouseID: "WH-CENTRAL-01", Delta: qty,
	})
	require.NoError(t, err)
}

func TestHappyPath_CreateOrder_EvaluatePass_DispatchFleet(t *testing.T) {
	r := setup(t)
	prodID, _ := mustProduct(t, r, "SKU-OIL-001", "Minyak Goreng Bimoli 2L", 12000)
	mustPricing(t, r, prodID, "GROSIR", 1, 120_000)
	mustPricing(t, r, prodID, "GROSIR", 10, 115_000) // volume discount
	mustStock(t, r, prodID, 100)

	custID := "cccccccc-0001-0000-0000-000000000001"
	created, err := r.Services.Order.Create(context.Background(), usecase.CreateInput{
		TenantID:           testTenant,
		CustomerID:         custID,
		CustomerName:       "Toko Sumber Rezeki",
		CustomerTier:       pricing.Tier("GROSIR"),
		CustomerPhone:      "081298765432",
		DestinationAddress: "Jl. Raya Daan Mogot No. 45",
		Items:              []usecase.CreateInputItem{{ProductID: prodID, Quantity: 10}},
		ActorID:            "test", IPAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	assert.Equal(t, "PENDING_CREDIT_CHECK", string(created.Order.Status))
	assert.EqualValues(t, 1_150_000, created.Order.TotalAmount, "volume discount should apply")
	assert.Equal(t, 120, created.Order.TotalWeightKg, "10 * 12000g = 120kg")

	// Evaluate credit (Core stub returns 8.5jt AR, limit 20jt, no overdue ??? PASS).
	res, err := r.Services.Order.EvaluateCredit(context.Background(), usecase.EvaluateInput{
		TenantID: testTenant, OrderID: created.Order.ID, ActorID: "test", IPAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	assert.Equal(t, "PASSED", string(res.CreditGateStatus))
	assert.Equal(t, "APPROVED", string(res.OrderStatus))
	assert.True(t, res.Evaluation.Pass)

	// Outbox should have 1 row.
	counts, _ := r.Services.Order.OutboxCounts(context.Background(), testTenant)
	require.Equal(t, 1, counts.Pending)

	// Drain outbox ??? stub Fleet receives the DO.
	sent, failed, err := r.Services.Order.DrainOutbox(context.Background(), 16, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Equal(t, 0, failed)

	// Order should now be DISPATCHED_TO_FLEET with fledger_fleet_do_id set.
	final, _, err := r.Services.Order.Get(context.Background(), testTenant, created.Order.ID)
	require.NoError(t, err)
	assert.Equal(t, "DISPATCHED_TO_FLEET", string(final.Status))
	assert.Equal(t, r.fleetDO.ID, final.FledgerFleetDOID)
}

func TestCreditGate_Overdue_Blocks(t *testing.T) {
	r := setup(t)
	// Make Core return overdue.
	r.arSummary.OverdueBucket = coreclient.OverdueBucket{HasOverdue30d: true}
	r.arSummary.OverdueDays = 47
	r.arSummary.OverdueInvoiceID = "INV-OLD-0012"

	prodID, _ := mustProduct(t, r, "SKU-X", "X", 5000)
	mustPricing(t, r, prodID, "RETAIL", 1, 50_000)
	mustStock(t, r, prodID, 100)

	custID := "cccccccc-0001-0000-0000-000000000001"
	created, err := r.Services.Order.Create(context.Background(), usecase.CreateInput{
		TenantID:           testTenant,
		CustomerID:         custID,
		CustomerName:       "Toko Macet",
		CustomerTier:       pricing.Tier("RETAIL"),
		DestinationAddress: "Jl. A",
		Items:              []usecase.CreateInputItem{{ProductID: prodID, Quantity: 2}},
		ActorID:            "test",
	})
	require.NoError(t, err)
	res, err := r.Services.Order.EvaluateCredit(context.Background(), usecase.EvaluateInput{
		TenantID: testTenant, OrderID: created.Order.ID, ActorID: "test",
	})
	require.NoError(t, err)
	assert.False(t, res.Evaluation.Pass)
	assert.Equal(t, "CREDIT_BLOCKED", string(res.OrderStatus))
	assert.Equal(t, "OVERDUE_BLOCKED", string(res.CreditGateStatus))
}

func TestCreditGate_LimitExceeded_Blocks(t *testing.T) {
	r := setup(t)
	// Set AR so projected exceeds limit.
	r.arSummary.OutstandingARMinor = 18_000_000
	r.arSummary.CreditLimitMinor = 20_000_000
	r.arSummary.OverdueBucket = coreclient.OverdueBucket{HasOverdue30d: false}

	prodID, _ := mustProduct(t, r, "SKU-Y", "Y", 1000)
	mustPricing(t, r, prodID, "GROSIR", 1, 50_000)
	mustStock(t, r, prodID, 100)

	custID := "cccccccc-0001-0000-0000-000000000001"
	created, err := r.Services.Order.Create(context.Background(), usecase.CreateInput{
		TenantID:           testTenant,
		CustomerID:         custID,
		CustomerName:       "Toko Limit",
		CustomerTier:       pricing.Tier("GROSIR"),
		DestinationAddress: "Jl. B",
		Items:              []usecase.CreateInputItem{{ProductID: prodID, Quantity: 50}}, // 50 * 50k = 2.5jt; AR+2.5jt = 20.5jt > 20jt
		ActorID:            "test",
	})
	require.NoError(t, err)
	res, err := r.Services.Order.EvaluateCredit(context.Background(), usecase.EvaluateInput{
		TenantID: testTenant, OrderID: created.Order.ID, ActorID: "test",
	})
	require.NoError(t, err)
	assert.False(t, res.Evaluation.Pass)
	assert.Equal(t, "CREDIT_BLOCKED", string(res.OrderStatus))
	assert.Equal(t, "LIMIT_EXCEEDED", string(res.CreditGateStatus))
}

func TestOverrideCredit_WithValidPin(t *testing.T) {
	r := setup(t)
	r.arSummary.OverdueBucket = coreclient.OverdueBucket{HasOverdue30d: true}
	r.arSummary.OverdueDays = 47

	prodID, _ := mustProduct(t, r, "SKU-Z", "Z", 2000)
	mustPricing(t, r, prodID, "GROSIR", 1, 80_000)
	mustStock(t, r, prodID, 100)

	custID := "cccccccc-0001-0000-0000-000000000001"
	created, err := r.Services.Order.Create(context.Background(), usecase.CreateInput{
		TenantID: testTenant, CustomerID: custID, CustomerName: "Toko Macet",
		CustomerTier: pricing.Tier("GROSIR"), DestinationAddress: "Jl. C",
		Items: []usecase.CreateInputItem{{ProductID: prodID, Quantity: 3}},
		ActorID: "test",
	})
	require.NoError(t, err)
	// Evaluate -> blocked
	_, err = r.Services.Order.EvaluateCredit(context.Background(), usecase.EvaluateInput{
		TenantID: testTenant, OrderID: created.Order.ID, ActorID: "test",
	})
	require.NoError(t, err)

	// Override with valid PIN.
	overridden, err := r.Services.Order.OverrideCredit(context.Background(), usecase.OverrideInput{
		TenantID: testTenant, OrderID: created.Order.ID, OverridePIN: "992144",
		Reason: "Owner berjanji transfer hari ini", ActorID: "finance-director-01",
	})
	require.NoError(t, err)
	assert.Equal(t, "APPROVED", string(overridden.Status))
	assert.Equal(t, "OVERRIDDEN", string(overridden.CreditGateStatus))
	assert.Equal(t, "finance-director-01", overridden.OverrideBy)

	// Outbox should now have 1 row (the dispatch).
	counts, _ := r.Services.Order.OutboxCounts(context.Background(), testTenant)
	assert.Equal(t, 1, counts.Pending)
}

func TestOverrideCredit_BadPinRejected(t *testing.T) {
	r := setup(t)
	prodID, _ := mustProduct(t, r, "SKU-W", "W", 1000)
	mustPricing(t, r, prodID, "RETAIL", 1, 10_000)
	mustStock(t, r, prodID, 10)
	created, err := r.Services.Order.Create(context.Background(), usecase.CreateInput{
		TenantID: testTenant, CustomerID: "cccccccc-0001-0000-0000-000000000001", CustomerName: "X",
		CustomerTier: pricing.Tier("RETAIL"), DestinationAddress: "Y",
		Items: []usecase.CreateInputItem{{ProductID: prodID, Quantity: 1}}, ActorID: "test",
	})
	require.NoError(t, err)
	_, err = r.Services.Order.OverrideCredit(context.Background(), usecase.OverrideInput{
		TenantID: testTenant, OrderID: created.Order.ID, OverridePIN: "WRONG",
		Reason: "x", ActorID: "test",
	})
	require.Error(t, err)
}

func TestStockReservation_InsufficientRejected(t *testing.T) {
	r := setup(t)
	prodID, _ := mustProduct(t, r, "SKU-Q", "Q", 1000)
	mustPricing(t, r, prodID, "RETAIL", 1, 5_000)
	mustStock(t, r, prodID, 1)
	_, err := r.Services.Order.Create(context.Background(), usecase.CreateInput{
		TenantID: testTenant, CustomerID: "cccccccc-0001-0000-0000-000000000001", CustomerName: "X",
		CustomerTier: pricing.Tier("RETAIL"), DestinationAddress: "Y",
		Items: []usecase.CreateInputItem{{ProductID: prodID, Quantity: 100}}, ActorID: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stock")
}

func TestGenerateOrderNumberFormat(t *testing.T) {
	n := usecase.GenerateOrderNumber(time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC))
	if len(n) < 14 || n[:4] != "ORD-" {
		t.Fatalf("bad format: %s", n)
	}
}

// _ = fmt to keep import in scope
var _ = fmt.Sprintf