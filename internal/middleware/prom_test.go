// Package middleware - tests for the Sprint 61 per-tenant rate-limit
// metrics labels. Verifies:
//   - tenant_id label is set correctly
//   - "anon" is used for empty/zero tenant IDs (no leak to "anonymous" bucket)
//   - Both IncRatelimitAllowed and IncRatelimitRejected increment counters
package middleware

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// TestSprint61_IncRatelimitAllowed_TenantLabel verifies that the metric
// counter has the expected tenant_id label after Sprint 61.
func TestSprint61_IncRatelimitAllowed_TenantLabel(t *testing.T) {
	// Capture counter state before + after
	tenantID := "11111111-2222-3333-4444-555555555555"
	before := getCounterValue(t, "fmcg_ratelimit_allowed_total", "ip", tenantID)
	IncRatelimitAllowed("ip", tenantID)
	after := getCounterValue(t, "fmcg_ratelimit_allowed_total", "ip", tenantID)
	if after-before < 1 {
		t.Errorf("expected counter to increment for tenant %s, got %v -> %v",
			tenantID, before, after)
	}
}

func TestSprint61_IncRatelimitRejected_TenantLabel(t *testing.T) {
	tenantID := "66666666-7777-8888-9999-aaaaaaaaaaaa"
	before := getCounterValue(t, "fmcg_ratelimit_rejected_total", "user", tenantID)
	IncRatelimitRejected("user", tenantID)
	after := getCounterValue(t, "fmcg_ratelimit_rejected_total", "user", tenantID)
	if after-before < 1 {
		t.Errorf("expected rejected counter to increment for tenant %s", tenantID)
	}
}

func TestSprint61_NormalizeTenantLabel_Anon(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "anon"},
		{"00000000-0000-0000-0000-000000000000", "anon"},
		{"11111111-2222-3333-4444-555555555555", "11111111-2222-3333-4444-555555555555"},
		{"abc", "abc"}, // not a UUID but still passes through (no validation in middleware)
	}
	for _, c := range cases {
		got := normalizeTenantLabel(c.in)
		if got != c.want {
			t.Errorf("normalizeTenantLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSprint61_AnonLabelIncrement(t *testing.T) {
	before := getCounterValue(t, "fmcg_ratelimit_allowed_total", "ip", "anon")
	IncRatelimitAllowed("ip", "") // empty → "anon"
	IncRatelimitAllowed("ip", "00000000-0000-0000-0000-000000000000") // zero UUID → "anon"
	after := getCounterValue(t, "fmcg_ratelimit_allowed_total", "ip", "anon")
	if after-before < 2 {
		t.Errorf("expected anon counter to increment twice, got %v -> %v", before, after)
	}
}

// getCounterValue reads the current value of a Prometheus counter.
// Helper for tests; production code uses prometheus.MustRegister directly.
func getCounterValue(t *testing.T, name, tier, tenantID string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			tierOK, tenantOK := false, false
			for _, l := range m.GetLabel() {
				if l.GetName() == "tier" && l.GetValue() == tier {
					tierOK = true
				}
				if l.GetName() == "tenant_id" && l.GetValue() == tenantID {
					tenantOK = true
				}
			}
			if tierOK && tenantOK {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// Compile-time interface guard: verify the dto import doesn't go stale.
// (Removed if unused.)
var _ = (*dto.MetricFamily)(nil)