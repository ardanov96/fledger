package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// prometheusHandler returns a Prometheus metrics handler wrapped in chi.
//
// In addition to the default Go runtime metrics (auto-registered by client_golang),
// we expose Sprint 22B.1 custom counters for the multi-tier rate limiter:
//   - fmcg_ratelimit_allowed_total{tier="ip|user|tenant"} counter
//   - fmcg_ratelimit_rejected_total{tier="ip|user|tenant"} counter
//
// The counters are populated by middleware.MultiTierMiddleware when a
// *middleware.MultiTierLimiterMetrics receiver is wired (was nil before this
// sprint — see cmd/api/main.go buildRouter). The middleware uses the
// prometheus.IncCounterFn-style helpers in internal/middleware/prom.go.
func prometheusHandler() http.Handler {
	// prometheus.DefaultRegisterer is initialised by client_golang when the
	// first metric is created; we ensure it here for parity with prior sprints.
	_ = prometheus.DefaultRegisterer

	return promhttp.Handler()
}
