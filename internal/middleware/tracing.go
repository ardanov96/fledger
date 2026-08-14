// Package middleware - W3C Trace Context propagation (Sprint 18 / Fase 3B).
//
// Why this and not the full OpenTelemetry SDK:
//   - OTel SDK brings heavy transitive deps (otel + otlp + exporters)
//   - For Sprint 18 we only need W3C traceparent propagation across services
//   - The full OTel SDK + Tempo OTLP exporter will be added in Sprint 18+
//   - This minimal implementation:
//     1. Parses incoming `traceparent` header (W3C Trace Context spec)
//     2. Generates one if absent (so logs always have a trace_id)
//     3. Stores trace_id in request context for logs / downstream services
//     4. Adds `traceparent` to response (echo for debugging)
//
// When OTel SDK is added later, this middleware will be replaced by
// otelhttp.Middleware (chained) - the W3C format stays the same.

package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// W3C Trace Context header name (RFC).
const TraceParentHeader = "traceparent"

// W3C traceparent format: "00-{trace-id-32hex}-{parent-id-16hex}-{flags-2hex}".
// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01

// traceIDKey is the context key for trace_id (16 random bytes hex).
type traceIDKey struct{}

// TraceIDFromContext returns the trace_id stored in ctx, or "" if none.
func TraceIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(traceIDKey{}).(string)
	return v
}

// TraceMiddleware extracts or generates a W3C traceparent header, stores the
// trace_id in the request context for downstream loggers, and echoes the
// header back in the response for client-side debugging.
func TraceMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := extractOrGenerateTraceID(r)

			// Store in context for downstream handlers / loggers.
			ctx := context.WithValue(r.Context(), traceIDKey{}, traceID)
			r = r.WithContext(ctx)

			// Echo back for client-side visibility.
			w.Header().Set(TraceParentHeader, formatTraceparent(traceID, "00"))

			next.ServeHTTP(w, r)
		})
	}
}

// extractOrGenerateTraceID returns the trace_id from incoming traceparent header,
// or generates a fresh 16-byte (32 hex chars) ID if missing.
func extractOrGenerateTraceID(r *http.Request) string {
	h := r.Header.Get(TraceParentHeader)
	if h == "" {
		return newTraceID()
	}
	// Parse: 00-<trace_id 32hex>-<span_id 16hex>-<flags 2hex>
	if len(h) < 55 {
		return newTraceID() // malformed; use fresh
	}
	if h[2:3] != "-" || h[35:36] != "-" || h[52:53] != "-" {
		return newTraceID()
	}
	id := h[3:35] // 32 hex chars
	if _, err := hex.DecodeString(id); err != nil || len(id) != 32 {
		return newTraceID()
	}
	return id
}

// newTraceID generates a fresh 16-byte (32 hex char) trace ID.
func newTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// formatTraceparent builds the full W3C traceparent header value.
func formatTraceparent(traceID, flags string) string {
	// span-id 16 hex chars (zero-padded) - placeholder
	return "00-" + traceID + "-0000000000000001-" + flags
}

// Helper to extract trace_id from httpx envelope (used by response middleware).
var _ = httpx.GetRequestID
