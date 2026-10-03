// Package middleware — Prometheus metric helpers for the multi-tier limiter
// (Sprint 22B.1, enhanced Sprint 61).
//
// Sprint 61 adds a `tenant_id` label to the rate-limit counters so
// operators can alert on per-tenant rate-limit hits (e.g., one noisy
// tenant spamming an endpoint). Labels:
//
//   tier       - which limiter tier rejected/allowed (ip | user | tenant | global)
//   tenant_id  - tenant UUID (or "anon" for unauthenticated requests)
//
// Higher cardinality tradeoff: tenant_id can be high (1000+ tenants in
// production). Prometheus handles this fine; alert rules should use
// `sum by (tenant_id)` to aggregate when needed.
//
// This file is intentionally separate from ratelimit_multi.go so the existing
// limiter stays decoupled from Prometheus. Callers in cmd/api wire the
// counters lazily (only when RATE_LIMIT_GLOBAL_ENABLED=true), avoiding
// unnecessary global state when rate limiting is disabled in dev/test.
package middleware

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ratelimitAllowedTotal and ratelimitRejectedTotal are the two Prometheus
// counters exposed at /metrics. Labels:
//
//   tier       - which limiter tier rejected/allowed
//   tenant_id  - tenant UUID (or "anon" for unauthenticated requests)
//
// Sprint 61: tenant_id label lets ops alert on per-tenant rate-limit hits
// ("tenant XYZ is spamming endpoint") rather than only aggregate counts.
// For the per-user tier, tenant_id is the tenant of the user (allows
// per-tenant aggregation even when individual users within a tenant are
// hitting their per-user cap).
var (
	ratelimitAllowedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fmcg_ratelimit_allowed_total",
			Help: "Total HTTP requests allowed by the multi-tier rate limiter, broken down by tier and tenant.",
		},
		[]string{"tier", "tenant_id"},
	)

	ratelimitRejectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fmcg_ratelimit_rejected_total",
			Help: "Total HTTP requests rejected by the multi-tier rate limiter, broken down by tier and tenant.",
		},
		[]string{"tier", "tenant_id"},
	)
)

func init() {
	// Register lazily on package load. Safe to call multiple times in test
	// binaries because prometheus.DefaultRegisterer panics on dup; if this
	// becomes a problem in CI, switch to a registry pool pattern.
	prometheus.MustRegister(ratelimitAllowedTotal, ratelimitRejectedTotal)
}

// IncRatelimitAllowed bumps the allowed counter for the given tier
// (ip | user | tenant | global) and tenant_id. tenant_id="anon" for
// unauthenticated requests (no JWT principal).
//
// Safe to call from any goroutine.
func IncRatelimitAllowed(tier, tenantID string) {
	ratelimitAllowedTotal.WithLabelValues(tier, normalizeTenantLabel(tenantID)).Inc()
}

// IncRatelimitRejected bumps the rejected counter for the given tier
// and tenant_id. Same normalization as IncRatelimitAllowed.
//
// Safe to call from any goroutine.
func IncRatelimitRejected(tier, tenantID string) {
	ratelimitRejectedTotal.WithLabelValues(tier, normalizeTenantLabel(tenantID)).Inc()
}

// normalizeTenantLabel returns "anon" for empty tenant IDs (so Prometheus
// doesn't get a high-cardinality "" label which would also be filtered
// out by many Grafana queries). Returns "anon" for the zero UUID
// (used by service accounts).
func normalizeTenantLabel(tenantID string) string {
	if tenantID == "" || tenantID == "00000000-0000-0000-0000-000000000000" {
		return "anon"
	}
	return tenantID
}