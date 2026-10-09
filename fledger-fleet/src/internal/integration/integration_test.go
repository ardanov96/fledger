// Package integration_test exercises the full POD → invoice-sync pipeline
// against a real PostgreSQL instance + a stubbed Fledger Core HTTP server.
//
// Uses a dedicated test database (default: fledger_fleet_test). The helper
// script scripts/setup-test-db.cmd creates it and runs the migrations before
// `go test ./...` runs.
package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fledger/fledger-fleet/internal/domain/delivery_order"
	"github.com/fledger/fledger-fleet/internal/domain/driver"
	"github.com/fledger/fledger-fleet/internal/domain/pod"
	"github.com/fledger/fledger-fleet/internal/domain/vehicle"
	"github.com/fledger/fledger-fleet/internal/integration/coreclient"
	"github.com/fledger/fledger-fleet/internal/repository/postgres"
	"github.com/fledger/fledger-fleet/internal/usecase"
)

const testTenant = "00000000-0000-0000-0000-000000000001"

func dbURL() string {
	if v := os.Getenv("FLEDGER_FLEET_TEST_DB"); v != "" {
		return v
	}
	return "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_fleet_test?sslmode=disable"
}

// rig is the wired-up test harness.
type rig struct {
	Pool     *pgxpool.Pool
	Services *usecase.Services
	CoreSrv  *httptest.Server
	// received captures every invoice the stub Core saw (in arrival order).
	mu       sync.Mutex
	received []coreclient.InvoiceInput
}

func setup(t *testing.T) *rig {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbURL())
	if err != nil {
		t.Skipf("test DB unavailable: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB ping failed: %v", err)
	}
	// Clean slate per test.
	for _, table := range []string{
		"fleet_do_status_history", "fleet_proof_of_deliveries", "fleet_do_items",
		"fleet_delivery_orders", "fleet_trips", "fleet_outbox",
		"fleet_drivers", "fleet_vehicles",
	} {
		_, _ = pool.Exec(context.Background(), "DELETE FROM "+table)
	}

	r := &rig{Pool: pool}
	r.CoreSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/v1/invoices" {
			var in coreclient.InvoiceInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			r.mu.Lock()
			r.received = append(r.received, in)
			idx := len(r.received)
			r.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(coreclient.InvoiceResponse{
				ID:          uuid.NewString(),
				Code:        in.Code,
				AmountMinor: in.AmountMinor,
				CustomerID:  in.CustomerID,
				Status:      "open",
			})
			_ = idx
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	client := coreclient.NewClient(coreclient.Config{
		BaseURL:  r.CoreSrv.URL,
		TenantID: testTenant,
	})
	r.Services = usecase.NewServices(usecase.Deps{
		Pool:       pool,
		Vehicles:   postgres.NewVehicleRepo(pool),
		Drivers:    postgres.NewDriverRepo(pool),
		Trips:      postgres.NewTripRepo(pool),
		DOs:        postgres.NewDORepo(pool),
		PODs:       postgres.NewPODRepo(pool),
		Outbox:     postgres.NewOutboxRepo(pool),
		Audit:      postgres.NewAuditRepo(pool),
		CoreClient: client,
	})
	t.Cleanup(func() {
		pool.Close()
		r.CoreSrv.Close()
	})
	return r
}

func mustVehicle(t *testing.T, r *rig, plate string) vehicle.Vehicle {
	t.Helper()
	v, err := r.Services.Vehicle.Create(context.Background(), vehicle.Vehicle{
		TenantID:    testTenant,
		PlateNumber: plate,
		VehicleType: "CDE_BOX",
		BrandModel:  "Test",
		CapacityKg:  1500,
	})
	require.NoError(t, err)
	return v
}

func mustDriver(t *testing.T, r *rig, name, phone string) driver.Driver {
	t.Helper()
	d, err := r.Services.Driver.Create(context.Background(), driver.Driver{
		TenantID:      testTenant,
		FullName:      name,
		PhoneNumber:   phone,
		LicenseNumber: "SIM-TEST",
	})
	require.NoError(t, err)
	return d
}

func mustDO(t *testing.T, r *rig, number string) (delivery_order.DeliveryOrder, []delivery_order.DOItem) {
	t.Helper()
	do, items, err := r.Services.DO.Create(context.Background(), usecase.CreateInput{
		TenantID:           testTenant,
		DoNumber:           number,
		CustomerID:         "cccccccc-0001-0000-0000-000000000001",
		CustomerName:       "Toko Test",
		DestinationAddress: "Jl Test 1",
		Items: []delivery_order.DOItem{
			{ProductSKU: "SKU-OIL-01", ProductName: "Minyak Goreng", QtyOrdered: 10, UnitPriceCents: 500_000},
		},
	})
	require.NoError(t, err)
	return do, items
}

// TestPOD_PartialDelivery_SyncsInvoice is the Sprint 5 end-to-end happy path.
func TestPOD_PartialDelivery_SyncsInvoice(t *testing.T) {
	r := setup(t)
	v := mustVehicle(t, r, "B TST-001")
	d := mustDriver(t, r, "Tester A", "081200000001")
	do, _ := mustDO(t, r, "DO-T-001")

	trip, err := r.Services.Trip.Create(context.Background(), usecase.CreateTripInput{
		TenantID: testTenant, TripNumber: "TRIP-T-001",
		VehicleID: v.ID, DriverID: d.ID, DoIDs: []string{do.ID},
	})
	require.NoError(t, err)
	_, err = r.Services.Trip.Dispatch(context.Background(), testTenant, trip.ID)
	require.NoError(t, err)

	res, err := r.Services.POD.Submit(context.Background(), usecase.SubmitInput{
		TenantID:          testTenant,
		DOID:              do.ID,
		RecipientName:     "Ibu Test",
		SignatureDataURL:  "data:image/png;base64,TEST",
		PhotoEvidenceURLs: []string{"https://storage.example/p1.jpg"},
		DeliveredLat:      -6.1,
		DeliveredLng:      106.7,
		Items: []pod.ItemResult{{
			ProductSKU: "SKU-OIL-01", QtyDelivered: 8, QtyRejected: 2,
			RejectionReason: "DAMAGED_LEAK",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, "DELIVERED_PARTIAL", res.Status)
	assert.EqualValues(t, 4_000_000, res.NominalDeliveredCents)
	assert.False(t, res.Queued)
	require.Len(t, r.received, 1, "stub Core must have seen exactly one invoice")
	assert.Equal(t, "cccccccc-0001-0000-0000-000000000001", r.received[0].CustomerID)
	assert.Equal(t, "INV-DO-DO-T-001", r.received[0].Code)
	assert.EqualValues(t, 4_000_000, r.received[0].AmountMinor)
}

// TestPOD_RejectsMissingPhotoEvidence covers the Sprint 3 guard.
func TestPOD_RejectsMissingPhotoEvidence(t *testing.T) {
	r := setup(t)
	v := mustVehicle(t, r, "B TST-002")
	d := mustDriver(t, r, "Tester B", "081200000002")
	do, _ := mustDO(t, r, "DO-T-002")
	trip, err := r.Services.Trip.Create(context.Background(), usecase.CreateTripInput{
		TenantID: testTenant, TripNumber: "TRIP-T-002",
		VehicleID: v.ID, DriverID: d.ID, DoIDs: []string{do.ID},
	})
	require.NoError(t, err)
	_, err = r.Services.Trip.Dispatch(context.Background(), testTenant, trip.ID)
	require.NoError(t, err)

	_, err = r.Services.POD.Submit(context.Background(), usecase.SubmitInput{
		TenantID:         testTenant,
		DOID:             do.ID,
		RecipientName:    "X",
		SignatureDataURL: "data:image/png;base64,TEST",
		// photo_evidence_urls is intentionally omitted
		DeliveredLat: -6.1,
		DeliveredLng: 106.7,
		Items: []pod.ItemResult{{
			ProductSKU: "SKU-OIL-01", QtyDelivered: 8, QtyRejected: 2,
			RejectionReason: "DAMAGED_LEAK",
		}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "photo_evidence_urls")
	assert.Empty(t, r.received, "no invoice must have been sent to Core")
}

// TestTrip_RejectsUnavailableVehicle proves Sprint 2's invariant: a vehicle
// already in ON_TRIP cannot be assigned to a new trip.
func TestTrip_RejectsUnavailableVehicle(t *testing.T) {
	r := setup(t)
	v := mustVehicle(t, r, "B TST-003")
	d := mustDriver(t, r, "Tester C", "081200000003")
	do, _ := mustDO(t, r, "DO-T-003")

	// First trip goes live — vehicle becomes ON_TRIP after dispatch.
	_, err := r.Services.Trip.Create(context.Background(), usecase.CreateTripInput{
		TenantID: testTenant, TripNumber: "TRIP-T-003-A",
		VehicleID: v.ID, DriverID: d.ID, DoIDs: []string{do.ID},
	})
	require.NoError(t, err)
	_, err = r.Services.Trip.Dispatch(context.Background(), testTenant, "TRIP-T-003-A")
	// Trip lookup is by UUID not number — query it back.
	all, err := r.Services.Trip.List(context.Background(), testTenant, "")
	require.NoError(t, err)
	require.Len(t, all, 1)
	_, err = r.Services.Trip.Dispatch(context.Background(), testTenant, all[0].ID)
	require.NoError(t, err)

	// Now try to create a second trip with the same vehicle — must fail.
	_, err = r.Services.Trip.Create(context.Background(), usecase.CreateTripInput{
		TenantID: testTenant, TripNumber: "TRIP-T-003-B",
		VehicleID: v.ID, DriverID: d.ID, DoIDs: []string{do.ID},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ON_TRIP")
}

// TestOutbox_QueuesWhenCoreOffline simulates Core being down on the first call.
func TestOutbox_QueuesWhenCoreOffline(t *testing.T) {
	r := setup(t)
	r.CoreSrv.Close() // take Core offline

	v := mustVehicle(t, r, "B TST-004")
	d := mustDriver(t, r, "Tester D", "081200000004")
	do, _ := mustDO(t, r, "DO-T-004")
	trip, err := r.Services.Trip.Create(context.Background(), usecase.CreateTripInput{
		TenantID: testTenant, TripNumber: "TRIP-T-004",
		VehicleID: v.ID, DriverID: d.ID, DoIDs: []string{do.ID},
	})
	require.NoError(t, err)
	_, err = r.Services.Trip.Dispatch(context.Background(), testTenant, trip.ID)
	require.NoError(t, err)

	res, err := r.Services.POD.Submit(context.Background(), usecase.SubmitInput{
		TenantID:         testTenant,
		DOID:             do.ID,
		RecipientName:    "X",
		SignatureDataURL: "data:image/png;base64,TEST",
		DeliveredLat:     -6.1,
		DeliveredLng:     106.7,
		Items: []pod.ItemResult{{
			ProductSKU: "SKU-OIL-01", QtyDelivered: 10, QtyRejected: 0,
		}},
	})
	require.NoError(t, err)
	assert.True(t, res.Queued, "must be queued when Core is unreachable")
	assert.Empty(t, res.FledgerInvoiceID)

	counts, err := r.Services.Outbox.Counts(context.Background(), testTenant)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, counts.Pending, 1)
}