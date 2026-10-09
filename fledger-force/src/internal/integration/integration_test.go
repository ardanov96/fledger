package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fledger/fledger-force/internal/domain/salesrep"
	"github.com/fledger/fledger-force/internal/domain/store"
	"github.com/fledger/fledger-force/internal/integration/coreclient"
	"github.com/fledger/fledger-force/internal/repository/postgres"
	"github.com/fledger/fledger-force/internal/usecase"
)

const testTenant = "00000000-0000-0000-0000-000000000001"

type rig struct {
	Pool      *pgxpool.Pool
	Services  *usecase.Services
	CoreSrv   *httptest.Server
	transfers []coreclient.TransferInput
}

func setup(t *testing.T) *rig {
	t.Helper()
	if v := os.Getenv("FLEDGER_FORCE_TEST_DB"); v != "" {
		os.Setenv("DATABASE_URL", v)
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_force_test?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Skipf("test DB unavailable: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB ping failed: %v", err)
	}
	for _, table := range []string{
		"force_audit_logs", "force_settlement_outbox", "force_eod_settlements",
		"force_cash_collections", "force_visits", "force_beat_plans",
		"force_stores", "force_sales_reps",
	} {
		_, _ = pool.Exec(context.Background(), "DELETE FROM "+table)
	}
	r := &rig{Pool: pool}
	r.CoreSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == "POST" && req.URL.Path == "/v1/transfers" {
			w.Header().Set("Connection", "close")
			body, _ := io.ReadAll(req.Body)
			var in coreclient.TransferInput
			_ = json.Unmarshal(body, &in)
			// Sequential appends are safe in httptest's single-threaded loop.
			r.transfers = append(r.transfers, in)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(coreclient.TransferResponse{
				TransactionID: uuid.NewString(),
				Status:        "POSTED",
				AmountMinor:   in.AmountMinor,
			})
			return
		}
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
	}))
	client := coreclient.NewClient(coreclient.Config{
		BaseURL: r.CoreSrv.URL, TenantID: testTenant, Timeout: 10 * time.Second,
	})
	r.Services = usecase.NewServices(usecase.Deps{
		Pool: pool,
		Reps: postgres.NewRepRepo(pool), Stores: postgres.NewStoreRepo(pool),
		Visits: postgres.NewVisitRepo(pool), Collections: postgres.NewCollectionRepo(pool),
		Settlements: postgres.NewSettlementRepo(pool), Outbox: postgres.NewOutboxRepo(pool),
		Audit: postgres.NewAuditRepo(pool), Core: client,
	})
	t.Cleanup(func() { pool.Close(); r.CoreSrv.Close() })
	return r
}

func mustRep(t *testing.T, r *rig, code, name, wallet string) *salesrep.Rep {
	t.Helper()
	out, err := r.Services.Rep.Create(context.Background(), usecase.CreateRepInput{
		TenantID: testTenant, EmployeeCode: code, Name: name, Phone: "081234567890",
		Role: "CANVASSER", FledgerWalletAccountID: wallet, MaxCashLimit: 50_000_000,
	})
	if err != nil {
		t.Fatalf("create rep: %v", err)
	}
	return &out
}

func mustStore(t *testing.T, r *rig, code, name string, lat, lng float64) *store.Store {
	t.Helper()
	out, err := r.Services.Store.Create(context.Background(), usecase.CreateStoreInput{
		TenantID: testTenant, StoreCode: code, Name: name, Address: "Jl. Test " + code,
		Latitude: lat, Longitude: lng, GeofenceRadiusMeters: 100, Tier: "RETAIL",
	})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return &out
}

func TestHappyPath_CheckIn_Collect_Settle(t *testing.T) {
	r := setup(t)
	rep := mustRep(t, r, "SLS-001", "Budi", "ACC_SALES_WALLET_BUDI")
	store := mustStore(t, r, "TKO-001", "Toko Sumber Rezeki", -6.175392, 106.827153)
	invoiceID := uuid.NewString()

	// Create beat plan for today.
	plan, err := r.Services.Visit.CreateBeatPlan(context.Background(), usecase.CreateBeatPlanInput{
		TenantID: testTenant, SalesRepID: rep.ID, PlanDate: time.Now().UTC(),
		Territory: "Jakarta Pusat", TargetStoresCount: 1,
	})
	require.NoError(t, err)

	// 1) Check-in within geofence (same coords as the store).
	checkIn, err := r.Services.Visit.CheckIn(context.Background(), usecase.CheckInInput{
		TenantID: testTenant, BeatPlanID: plan.ID, StoreID: store.ID,
		SalesRepID: rep.ID, Latitude: -6.175392, Longitude: 106.827153,
		VisitType: "TAKING_ORDER_AND_COLLECTION",
		ActorID:   "test", IPAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	assert.True(t, checkIn.GeofenceVerified, "should be within 100m")
	assert.LessOrEqual(t, checkIn.DistanceMeters, 100)

	// 2) Collect Rp 2.500.000 cash.
	col, err := r.Services.Collection.Collect(context.Background(), usecase.CollectInput{
		TenantID: testTenant, VisitID: checkIn.ID, SalesRepID: rep.ID,
		StoreID: store.ID, FledgerInvoiceID: invoiceID, Amount: 2_500_000,
		PayerName: "Ibu Siti", PayerPhone: "081234567890",
		ActorID: "test", IPAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 2_500_000, col.SalesRepCurrentCash)
	assert.NotEmpty(t, col.Collection.ReceiptNumber)
	assert.NotEmpty(t, col.WAResceipt.Message)

	// 3) Outbox should have 1 pending row.
	counts, _ := r.Services.Collection.OutboxCounts(context.Background(), testTenant)
	require.Equal(t, 1, counts.Pending)

	// 4) Drain outbox — Core stub sees the transfer.
	sent, failed, err := r.Services.Collection.DrainOutbox(context.Background(), 16, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Equal(t, 0, failed)
	require.Len(t, r.transfers, 1)
	assert.EqualValues(t, 2_500_000, r.transfers[0].AmountMinor)
	assert.Equal(t, "ACC_SALES_WALLET_BUDI", r.transfers[0].ToAccountID)
	assert.Equal(t, "ACC_CUSTOMER_AR", r.transfers[0].FromAccountID)

	// 5) Inquiry before EOD shows balance.
	inq, err := r.Services.Collection.Inquiry(context.Background(), testTenant, rep.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 2_500_000, inq.TotalCashHeld)

	// 6) EOD settlement — exact match.
	settle, err := r.Services.Collection.Settle(context.Background(), usecase.SettleInput{
		TenantID: testTenant, SalesRepID: rep.ID, PhysicalCashReceived: 2_500_000,
		CashierNotes: "Uang pas", ActorID: "cashier-1", IPAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 0, settle.DiscrepancyAmount)
	assert.Equal(t, "SETTLED", string(settle.Settlement.Status))
	assert.Equal(t, "ACTIVE", settle.SalesRepStatus)
	assert.EqualValues(t, 0, settle.SalesRepNewCashHeld)

	// 7) Outbox now has 1 EOD event.
	sent2, _, _ := r.Services.Collection.DrainOutbox(context.Background(), 16, 10)
	assert.Equal(t, 1, sent2)
	assert.Len(t, r.transfers, 2)
	// Second transfer = EOD settlement from salesman wallet to HQ cash.
	assert.Equal(t, "ACC_SALES_WALLET_BUDI", r.transfers[1].FromAccountID)
	assert.Equal(t, "ACC_HQ_PHYSICAL_CASH", r.transfers[1].ToAccountID)
}

func TestGeofence_OutOfRadius_StillRecords(t *testing.T) {
	r := setup(t)
	rep := mustRep(t, r, "SLS-002", "Ani", "ACC_SALES_WALLET_ANI")
	store := mustStore(t, r, "TKO-002", "Toko Jauh", -6.175392, 106.827153)
	plan, err := r.Services.Visit.CreateBeatPlan(context.Background(), usecase.CreateBeatPlanInput{
		TenantID: testTenant, SalesRepID: rep.ID, PlanDate: time.Now().UTC(),
		Territory: "X", TargetStoresCount: 1,
	})
	require.NoError(t, err)
	// 1.5 km away from the store — outside 100m radius.
	out, err := r.Services.Visit.CheckIn(context.Background(), usecase.CheckInInput{
		TenantID: testTenant, BeatPlanID: plan.ID, StoreID: store.ID,
		SalesRepID: rep.ID, Latitude: -6.189, Longitude: 106.840,
		ActorID: "test", IPAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	assert.False(t, out.GeofenceVerified, "should be flagged out-of-radius")
	assert.Greater(t, out.DistanceMeters, 1000)
}

func TestCashCollection_RejectsOverMaxLimit(t *testing.T) {
	r := setup(t)
	rep := mustRep(t, r, "SLS-003", "Cecep", "ACC_SALES_WALLET_CECEP")
	store := mustStore(t, r, "TKO-003", "Toko Cecep", -6.175, 106.827)
	// Set a tight max limit via the DB.
	_, err := r.Pool.Exec(context.Background(),
		`UPDATE force_sales_reps SET max_cash_limit = 1000000 WHERE id = $1`, rep.ID)
	require.NoError(t, err)

	_, err = r.Services.Collection.Collect(context.Background(), usecase.CollectInput{
		TenantID: testTenant, SalesRepID: rep.ID, StoreID: store.ID,
		FledgerInvoiceID: uuid.NewString(), Amount: 2_000_000,
		PayerName: "Pak X",
		ActorID: "test", IPAddress: "127.0.0.1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Batas kas fisik")
}

func TestEODSettlement_Discrepancy_LocksRep(t *testing.T) {
	r := setup(t)
	rep := mustRep(t, r, "SLS-004", "Dodi", "ACC_SALES_WALLET_DODI")
	store := mustStore(t, r, "TKO-004", "Toko Dodi", -6.175, 106.827)
	// First a 3.000.000 collection.
	_, err := r.Services.Collection.Collect(context.Background(), usecase.CollectInput{
		TenantID: testTenant, SalesRepID: rep.ID, StoreID: store.ID,
		FledgerInvoiceID: uuid.NewString(), Amount: 3_000_000,
		PayerName: "Pak Dodi",
		ActorID: "test", IPAddress: "127.0.0.1",
	})
	require.NoError(t, err)

	// Then settle with a 500.000 shortfall.
	settle, err := r.Services.Collection.Settle(context.Background(), usecase.SettleInput{
		TenantID: testTenant, SalesRepID: rep.ID, PhysicalCashReceived: 2_500_000,
		CashierNotes: "kurang 500rb", ActorID: "cashier-1", IPAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	assert.EqualValues(t, -500_000, settle.DiscrepancyAmount)
	assert.Equal(t, "DISCREPANCY_FLAGGED", string(settle.Settlement.Status))
	assert.Equal(t, "SETTLEMENT_LOCKED", settle.SalesRepStatus)

	// Salesman is now locked — collection must be rejected.
	_, err = r.Services.Collection.Collect(context.Background(), usecase.CollectInput{
		TenantID: testTenant, SalesRepID: rep.ID, StoreID: store.ID,
		FledgerInvoiceID: uuid.NewString(), Amount: 100_000,
		PayerName: "Pak Dodi",
		ActorID: "test", IPAddress: "127.0.0.1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SETTLEMENT_LOCKED")
}

func TestReceiptNumberFormat(t *testing.T) {
	n := usecase.GenerateReceiptNumber(time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC))
	if len(n) < 14 || n[:4] != "RCP-" {
		t.Fatalf("bad receipt format: %s", n)
	}
	// Year-month section.
	if !(n[4:10] == "202610") {
		t.Fatalf("bad ym: %s", n)
	}
}

func TestEODNumberFormat(t *testing.T) {
	n := usecase.GenerateEODSettlementNumber(time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC))
	if len(n) < 14 || n[:4] != "EOD-" {
		t.Fatalf("bad eod format: %s", n)
	}
}

func TestWAResceiptPayload(t *testing.T) {
	p := usecase.BuildWAResceiptPayload(
		"Toko Sumber Rezeki", "Ibu Siti", "RCP-202610-ABCDE",
		"a1000000-0000-0000-0000-000000000001", "SLS-JKT-004", "Budi", 2500000)
	if p.Message == "" {
		t.Fatalf("empty message")
	}
	if !contains(p.Message, "RCP-202610-ABCDE") {
		t.Fatalf("missing receipt in message: %s", p.Message)
	}
	if !contains(p.Message, "Budi") {
		t.Fatalf("missing sales name: %s", p.Message)
	}
}

func contains(s, sub string) bool { return fmt.Sprintf("%s", s) != "" && (len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}