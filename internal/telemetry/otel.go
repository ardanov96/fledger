// Package telemetry - OpenTelemetry SDK setup (Sprint 39).
//
// Initializes a TracerProvider with OTLP/HTTP exporter when OTEL_ENABLED=true.
// Spans are emitted to the configured OTLP endpoint (default: http://localhost:4318).
//
// Usage in cmd/api/main.go:
//
//	if cfg.Telemetry.OTELEnabled {
//	    shutdown, err := telemetry.InitTracer(ctx, cfg.Telemetry)
//	    if err != nil {
//	        log.Warn("OTel init failed", "error", err)
//	    } else {
//	        defer shutdown(context.Background())
//	    }
//	}
//
// W3C trace context is propagated automatically — the existing TraceMiddleware
// (Sprint 34) and the new otelhttp.Middleware can coexist. Sprint 39 wires
// both: the custom middleware provides trace_id in logs even when OTel is
// off, and otelhttp provides full span creation when OTel is on.
package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// TelemetryConfig holds the OTel configuration.
type TelemetryConfig struct {
	Enabled       bool
	OTLPEndpoint  string // e.g. http://tempo:4318
	ServiceName   string // e.g. fmcg-wallet-api
	SamplerRatio  float64
}

// ShutdownFunc gracefully shuts down the TracerProvider, flushing any
// buffered spans to the OTLP collector. Always call via defer.
type ShutdownFunc func(context.Context) error

// InitTracer sets up the OTel TracerProvider with OTLP/HTTP exporter.
// Returns a ShutdownFunc that should be called on process exit.
//
// When cfg.Enabled is false, returns a no-op ShutdownFunc — no global
// state is changed.
func InitTracer(ctx context.Context, cfg TelemetryConfig) (ShutdownFunc, error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	if cfg.OTLPEndpoint == "" {
		cfg.OTLPEndpoint = "http://localhost:4318"
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "fmcg-wallet"
	}
	if cfg.SamplerRatio <= 0 || cfg.SamplerRatio > 1 {
		cfg.SamplerRatio = 1.0 // default: sample everything
	}

	// Resource describes the service that produces spans.
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
		resource.WithProcess(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, fmt.Errorf("otel: create resource: %w", err)
	}

	// OTLP/HTTP exporter
	exporter, err := otlptrace.New(ctx,
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(stripScheme(cfg.OTLPEndpoint)),
			otlptracehttp.WithInsecure(), // local Tempo — no TLS
			otlptracehttp.WithTimeout(5*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel: create exporter: %w", err)
	}

	// TracerProvider with batch span processor + ratio-based sampler
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SamplerRatio)),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
	)
	otel.SetTracerProvider(tp)

	// W3C trace context + baggage propagators (standard for HTTP services)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(ctx context.Context) error {
		// Use a fresh context with timeout for the shutdown — the parent
		// ctx may already be cancelled by the time we shut down.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ctx
		return tp.Shutdown(shutdownCtx)
	}
	return shutdown, nil
}

// stripScheme removes http:// or https:// from an endpoint since
// otlptracehttp.WithEndpoint expects host:port only.
func stripScheme(endpoint string) string {
	const (
		httpScheme  = "http://"
		httpsScheme = "https://"
	)
	if strings.HasPrefix(endpoint, httpScheme) {
		return endpoint[len(httpScheme):]
	}
	if strings.HasPrefix(endpoint, httpsScheme) {
		return endpoint[len(httpsScheme):]
	}
	return endpoint
}

// HTTPMiddleware returns an otelhttp middleware that creates spans for each
// incoming request. Use after InitTracer (no-op if OTel is disabled).
//
// Apply BEFORE the existing TraceMiddleware (Sprint 34) so:
//   - otelhttp creates the parent span + extracts W3C context
//   - TraceMiddleware reads the trace_id from ctx for logs
func HTTPMiddleware(next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, "http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
}