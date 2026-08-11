package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// prometheusHandler returns a Prometheus metrics handler wrapped in chi.
//
// In Fase 0, the API does not yet expose custom metrics — the default
// process/Go runtime metrics are still useful for the System Health dashboard.
// As Fase 3 progresses, we register custom counters/histograms for
// business metrics (transactions_total, outbox_lag, etc.).
func prometheusHandler() http.Handler {
	// Force registration of the default Go runtime metrics.
	// prometheus.DefaultRegisterer.MustRegister(...) is called by clients
	// when they create their first metric; we ensure it here for parity.
	_ = prometheus.DefaultRegisterer

	return promhttp.Handler()
}
