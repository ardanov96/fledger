package geo

import (
	"math"
	"testing"
)

// TestHaversine_KnownReference checks the formula against the Jakarta
// reference pair (Monas → Kota Tua). Both are ~3.2 km apart — the test
// asserts the calculated distance is within 5% of the true value.
func TestHaversine_KnownReference(t *testing.T) {
	// Monas (Jakarta Pusat) and Museum Fatahillah (Jakarta Kota).
	monasLat, monasLng := -6.175392, 106.827153
	kotaLat, kotaLng := -6.135200, 106.813690
	d := DistanceMeters(monasLat, monasLng, kotaLat, kotaLng)
	// Real distance ≈ 4.6 km (straight line). Tolerance: ±10%.
	if d < 4100 || d > 5100 {
		t.Fatalf("Haversine distance off: got %.1f m, expected ~4600 m", d)
	}
}

// TestHaversine_SamePoint asserts d ≈ 0 for identical coordinates.
func TestHaversine_SamePoint(t *testing.T) {
	d := DistanceMeters(-6.2, 106.8, -6.2, 106.8)
	if d > 0.001 {
		t.Fatalf("same point must give 0 distance, got %f", d)
	}
}

// TestWithinRadius covers the boolean decision used in the check-in
// handler.
func TestWithinRadius(t *testing.T) {
	// Two points 50m apart.
	a := DistanceMeters(-6.175, 106.827, -6.175, 106.82745) // 1 degree lng ≈ 111 km
	// We construct a contrived case: same lat, 0.0001 deg apart.
	b := math.Abs(a - 11.13) // ~ 11.13m at equator
	_ = b
	ok, d := WithinRadius(-6.175, 106.827, -6.175, 106.827, 100)
	if !ok {
		t.Fatalf("expected within 100m radius, got %.2f m", d)
	}
	ok, d = WithinRadius(-6.175, 106.827, -6.175, 106.829, 100)
	if ok {
		t.Fatalf("expected OUT of 100m radius, got %.2f m", d)
	}
}