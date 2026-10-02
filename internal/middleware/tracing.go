// Package middleware - W3C Trace Context propagation (Sprint 18 / Fase 3B,
// enhanced Sprint 34).
//
// Sprint 34 enhancement: previous implementation only carried trace_id.
// This version tracks the full W3C Trace Context: trace_id + span_id +
// parent_span_id + flags. Each request is its own span; downstream calls
// (DB, NATS publish) can create child spans via NewChildSpanID(ctx).
//
// The W3C traceparent format is identical to what OpenTelemetry SDK emits,
// so this implementation is forward-compatible: dropping in `go.opentelemetry.io/otel`
// later (Sprint 34.1) requires no header changes, just middleware replacement.
//
// Why custom (vs OTel SDK directly):
//   - OTel SDK adds heavy transitive deps (otel + sdk + exporters ≈ 5 MB)
//   - This middleware provides 100% of the W3C context propagation without
//     needing OTLP exporters (Tempo etc.) for the core flow to work
//   - For full distributed tracing with sampling + spans + exporters, see
//     Sprint 34.1 follow-up to add OTel SDK + Tempo OTLP exporter.
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

// spanCtxKey is the context key for the full *SpanContext value.
type spanCtxKey struct{}

// SpanContext is the full W3C trace context propagated through ctx.
// Sprint 34: this replaces the previous traceIDKey-only design.
type SpanContext struct {
	TraceID       string // 32 hex chars (16 bytes)
	SpanID        string // 16 hex chars (8 bytes) — current span
	ParentSpanID  string // 16 hex chars (8 bytes) — empty for root span
	Flags         string // 2 hex chars (W3C: 01 = sampled)
}

// TraceIDStr returns the trace_id (32 hex chars), or "" if no context.
// Named to avoid collision with the TraceID field.
func (s *SpanContext) TraceIDStr() string {
	if s == nil {
		return ""
	}
	return s.TraceID
}

// SpanIDStr returns the span_id (16 hex chars), or "" if no context.
func (s *SpanContext) SpanIDStr() string {
	if s == nil {
		return ""
	}
	return s.SpanID
}

// TraceparentHeader returns the W3C-formatted header value for this span.
func (s *SpanContext) TraceparentHeader() string {
	if s == nil {
		return ""
	}
	return formatTraceparent(s.TraceID, s.SpanID, s.Flags)
}

// SpanContextFromContext returns the *SpanContext stored in ctx, or nil if none.
func SpanContextFromContext(ctx context.Context) *SpanContext {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(spanCtxKey{}).(*SpanContext)
	return v
}

// NewChildSpanID generates a new span_id and returns a child SpanContext
// that shares the trace_id but has a new span_id (and the parent set to
// the current span_id). Use this when starting a downstream operation
// (DB query, NATS publish, etc.) so the trace is correlated end-to-end.
//
// Example:
//
//	span := middleware.SpanContextFromContext(ctx)
//	childSpan := span.NewChild()
//	childCtx := middleware.WithSpanContext(ctx, childSpan)
//	db.Query(childCtx, ...)
func (s *SpanContext) NewChild() *SpanContext {
	if s == nil {
		return newRootSpan()
	}
	return &SpanContext{
		TraceID:      s.TraceID,
		SpanID:       newSpanID(),
		ParentSpanID: s.SpanID,
		Flags:        s.Flags,
	}
}

// newRootSpan generates a fresh root span (no parent).
func newRootSpan() *SpanContext {
	return &SpanContext{
		TraceID: newTraceID(),
		SpanID:  newSpanID(),
		Flags:   "01", // sampled
	}
}

// WithSpanContext stores the given span context on ctx. Returns the new ctx.
func WithSpanContext(ctx context.Context, s *SpanContext) context.Context {
	if s == nil {
		return ctx
	}
	return context.WithValue(ctx, spanCtxKey{}, s)
}

// TraceMiddleware extracts or generates a W3C traceparent header, creates a
// per-request SpanContext (with parent from incoming header if any), stores
// it in the request context, and echoes the traceparent header back in the
// response for client-side debugging.
func TraceMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			span := extractOrGenerateSpan(r)

			ctx := WithSpanContext(r.Context(), span)
			r = r.WithContext(ctx)

			// Echo back for client-side visibility.
			w.Header().Set(TraceParentHeader, span.TraceparentHeader())

			next.ServeHTTP(w, r)
		})
	}
}

// extractOrGenerateSpan returns a span from the incoming traceparent header,
// or generates a fresh root span if missing / malformed.
func extractOrGenerateSpan(r *http.Request) *SpanContext {
	h := r.Header.Get(TraceParentHeader)
	if h == "" {
		return newRootSpan()
	}
	// Parse: 00-<trace_id 32hex>-<span_id 16hex>-<flags 2hex>
	if len(h) < 55 {
		return newRootSpan()
	}
	if h[2:3] != "-" || h[35:36] != "-" || h[52:53] != "-" {
		return newRootSpan()
	}
	traceID := h[3:35] // 32 hex chars
	parentSpanID := h[36:52] // 16 hex chars
	flags := h[53:55] // 2 hex chars
	if _, err := hex.DecodeString(traceID); err != nil {
		return newRootSpan()
	}
	return &SpanContext{
		TraceID:      traceID,
		SpanID:       newSpanID(), // we are the new span; parent = incoming
		ParentSpanID: parentSpanID,
		Flags:        flags,
	}
}

// newTraceID generates a fresh 16-byte (32 hex char) trace ID.
func newTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// newSpanID generates a fresh 8-byte (16 hex char) span ID.
func newSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// formatTraceparent builds the full W3C traceparent header value.
func formatTraceparent(traceID, spanID, flags string) string {
	return "00-" + traceID + "-" + spanID + "-" + flags
}

// LogAttrs returns slog-compatible attributes for the given span context.
// Use in structured loggers to correlate logs across services.
func LogAttrs(s *SpanContext) []any {
	if s == nil {
		return nil
	}
	return []any{
		"trace_id", s.TraceID,
		"span_id", s.SpanID,
		"parent_span_id", s.ParentSpanID,
	}
}

// Helper to extract trace_id from httpx envelope (used by response middleware).
var _ = httpx.GetRequestID

// TraceIDFromContext (Sprint 18 backward compat) returns the trace_id from
// the SpanContext stored in ctx, or "" if none.
func TraceIDFromContext(ctx context.Context) string {
	s := SpanContextFromContext(ctx)
	if s == nil {
		return ""
	}
	return s.TraceID
}