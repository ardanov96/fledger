package domain

import (
	"testing"

	"github.com/fledger/fledger-fleet/internal/domain/delivery_order"
	"github.com/fledger/fledger-fleet/internal/domain/pod"
	"github.com/fledger/fledger-fleet/internal/domain/vehicle"
)

// TestVehicleStatusValid guards the Status.Valid() method.
func TestVehicleStatusValid(t *testing.T) {
	for _, s := range []vehicle.Status{
		vehicle.StatusAvailable,
		vehicle.StatusOnTrip,
		vehicle.StatusMaintenance,
	} {
		if !s.Valid() {
			t.Fatalf("expected %q to be valid", s)
		}
	}
	if vehicle.Status("UNKNOWN").Valid() {
		t.Fatalf("UNKNOWN should not be valid")
	}
}

// TestComputeNominalOrdered verifies the SKU price math used for DO totals and
// for the settled invoice payload.
func TestComputeNominalOrdered(t *testing.T) {
	items := []delivery_order.DOItem{
		{ProductSKU: "SKU-A", QtyOrdered: 10, UnitPriceCents: 500_000}, // 5_000_000
		{ProductSKU: "SKU-B", QtyOrdered: 5, UnitPriceCents: 120_000},  //   600_000
		{ProductSKU: "SKU-C", QtyOrdered: 0, UnitPriceCents: 999_999},   //   skipped
	}
	got := delivery_order.ComputeNominalOrdered(items)
	if got != 5_600_000 {
		t.Fatalf("want 5600000 got %d", got)
	}
}

// TestComputeNominalDelivered exercises the partial-delivery math that drives
// the invoice sync.
func TestComputeNominalDelivered(t *testing.T) {
	items := []delivery_order.DOItem{
		{ProductSKU: "SKU-A", QtyDelivered: 8, UnitPriceCents: 500_000}, // 4_000_000
		{ProductSKU: "SKU-B", QtyDelivered: 5, UnitPriceCents: 120_000}, //   600_000
	}
	got := delivery_order.ComputeNominalDelivered(items)
	if got != 4_600_000 {
		t.Fatalf("want 4600000 got %d", got)
	}
}

// TestComputeDOStatus covers the state-machine branching for full, partial,
// and failed deliveries.
func TestComputeDOStatus(t *testing.T) {
	cases := []struct {
		ordered, delivered, rejected int
		want                         delivery_order.Status
	}{
		{10, 10, 0, delivery_order.StatusDeliveredFull},
		{10, 8, 2, delivery_order.StatusDeliveredPartial},
		{10, 6, 4, delivery_order.StatusDeliveredPartial},
		{10, 5, 5, delivery_order.StatusDeliveredPartial},
		{10, 0, 0, delivery_order.StatusDeliveryFailed},
		{10, 0, 10, delivery_order.StatusDeliveryFailed},
		{10, 4, 0, delivery_order.StatusDeliveryFailed},
	}
	for _, c := range cases {
		got := pod.ComputeDOStatus(c.ordered, c.delivered, c.rejected)
		if got != c.want {
			t.Errorf("ordered=%d delivered=%d rejected=%d: want %s got %s",
				c.ordered, c.delivered, c.rejected, c.want, got)
		}
	}
}

// TestValidRejectionReasons guards the closed set of rejection_reason codes
// (Sprint 3 invariant — anything outside the set is a 400).
func TestValidRejectionReasons(t *testing.T) {
	known := pod.ValidRejectionReasons()
	for _, k := range []string{"DAMAGED_LEAK", "EXPIRED", "WRONG_ITEM", "STORE_REJECTED"} {
		if _, ok := known[k]; !ok {
			t.Fatalf("%s should be a known rejection reason", k)
		}
	}
	if _, ok := known["BAD_CODE"]; ok {
		t.Fatalf("BAD_CODE should not be a known rejection reason")
	}
}