// Package middleware — Prometheus metric helpers for the multi-tier limiter
// (Sprint 22B.1).
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
// counters exposed at /metrics. The label `tier` tells operators which tier
// (per-ip / per-user / per-tenant) is rejecting traffic — useful for
// differentiating anonymous floods vs authenticated abuse vs noisy tenants.
//
// Both counters are registered against prometheus.DefaultRegisterer. Any other
// metrics package that uses the same default registry will see them in the
// same scrape output (without having to thread a *prometheus.Registry pointer
// through main.go).
var (
	ratelimitAllowedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fmcg_ratelimit_allowed_total",
			Help: "Total number of HTTP requests allowed by the multi-tier rate limiter, broken down by tier.",
		},
		[]string{"tier"},
	)

	ratelimitRejectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fmcg_ratelimit_rejected_total",
			Help: "Total number of HTTP requests rejected by the multi-tier rate limiter, broken down by tier.",
		},
		[]string{"tier"},
	)
)

func init() {
	// Register lazily on package load. Safe to call multiple times in test
	// binaries because prometheus.DefaultRegisterer panics on dup; if this
	// becomes a problem in CI, switch to a registry pool pattern.
	prometheus.MustRegister(ratelimitAllowedTotal, ratelimitRejectedTotal)
}

// IncRatelimitAllowed bumps the allowed counter for the given tier
// (ip | user | tenant | global). Safe to call from any goroutine.
func IncRatelimitAllowed(tier string) {
	ratelimitAllowedTotal.WithLabelValues(tier).Inc()
}

// IncRatelimitRejected bumps the rejected counter for the given tier.
// Safe to call from any goroutine.
func IncRatelimitRejected(tier string) {
	ratelimitRejectedTotal.WithLabelValues(tier).Inc()
}
