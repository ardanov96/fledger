// Package usecase_test holds unit tests for the application services that
// do NOT need a live PostgreSQL connection (e.g. validation rules,
// pure-function behaviour). Integration tests that need a DB live in
// internal/integration.
package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fledger/fledger-fleet/internal/domain/delivery_order"
	"github.com/fledger/fledger-fleet/internal/domain/vehicle"
	"github.com/fledger/fledger-fleet/internal/usecase"
)

// TestCapacityValidation_Pass proves that within-capacity totals succeed using real usecase logic.
func TestCapacityValidation_Pass(t *testing.T) {
	v := vehicle.Vehicle{CapacityKg: 1500}
	// 50 boxes × 10kg = 500kg — well under 1500kg.
	boxes := 50
	assert.Equal(t, 500.0, usecase.EstimateWeightKg(boxes))
	assert.NoError(t, usecase.CheckCapacity(v.CapacityKg, boxes))
}

// TestCapacityValidation_Fail covers the failure path the trip service uses
// to reject a trip that would overload the assigned vehicle.
func TestCapacityValidation_Fail(t *testing.T) {
	v := vehicle.Vehicle{CapacityKg: 400}
	// 50 boxes × 10kg = 500kg — exceeds 400kg capacity.
	boxes := 50
	err := usecase.CheckCapacity(v.CapacityKg, boxes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds vehicle capacity")
}

// TestDeliveryOrder_ComputeNominal_AllCases exercises the money math used by
// the DO service to compute the ordered & settled nominals.
func TestDeliveryOrder_ComputeNominal_AllCases(t *testing.T) {
	items := []delivery_order.DOItem{
		{ProductSKU: "A", QtyOrdered: 5, QtyDelivered: 5, UnitPriceCents: 100_000},  // 500_000
		{ProductSKU: "B", QtyOrdered: 10, QtyDelivered: 7, UnitPriceCents: 200_000}, // 1_400_000
	}
	ordered := delivery_order.ComputeNominalOrdered(items)
	delivered := delivery_order.ComputeNominalDelivered(items)
	require.EqualValues(t, 2_500_000, ordered)
	require.EqualValues(t, 1_900_000, delivered)
}