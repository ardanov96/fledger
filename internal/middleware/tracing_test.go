// Package middleware - tests for SpanContext W3C propagation (Sprint 34).
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSpanContext_GenerateNewRoot(t *testing.T) {
	span := newRootSpan()
	if len(span.TraceID) != 32 {
		t.Errorf("TraceID length = %d, want 32", len(span.TraceID))
	}
	if len(span.SpanID) != 16 {
		t.Errorf("SpanID length = %d, want 16", len(span.SpanID))
	}
	if span.ParentSpanID != "" {
		t.Errorf("root span ParentSpanID = %q, want empty", span.ParentSpanID)
	}
	if span.Flags != "01" {
		t.Errorf("Flags = %q, want 01", span.Flags)
	}
}

func TestSpanContext_NewChild(t *testing.T) {
	parent := newRootSpan()
	child := parent.NewChild()
	if child.TraceID != parent.TraceID {
		t.Errorf("child.TraceID = %q, want %q (parent)", child.TraceID, parent.TraceID)
	}
	if child.ParentSpanID != parent.SpanID {
		t.Errorf("child.ParentSpanID = %q, want %q (parent.SpanID)", child.ParentSpanID, parent.SpanID)
	}
	if child.SpanID == parent.SpanID {
		t.Errorf("child.SpanID should differ from parent.SpanID")
	}
}

func TestSpanContext_NilChild(t *testing.T) {
	var nilSpan *SpanContext
	child := nilSpan.NewChild()
	if child == nil {
		t.Fatal("NewChild on nil should return a new root span")
	}
	if child.ParentSpanID != "" {
		t.Errorf("nil.NewChild().ParentSpanID = %q, want empty (root)", child.ParentSpanID)
	}
}

func TestSpanContext_TraceparentHeader(t *testing.T) {
	span := &SpanContext{
		TraceID: "0af7651916cd43dd8448eb211c80319c",
		SpanID:  "b7ad6b7169203331",
		Flags:   "01",
	}
	want := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	if got := span.TraceparentHeader(); got != want {
		t.Errorf("TraceparentHeader = %q, want %q", got, want)
	}
}

func TestTraceMiddleware_GeneratesSpanWhenNoHeader(t *testing.T) {
	called := false
	handler := TraceMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		span := SpanContextFromContext(r.Context())
		if span == nil {
			t.Fatal("expected SpanContext in ctx, got nil")
		}
		if len(span.TraceID) != 32 {
			t.Errorf("TraceID length = %d, want 32", len(span.TraceID))
		}
		if span.ParentSpanID != "" {
			t.Errorf("ParentSpanID = %q, want empty (no incoming header)", span.ParentSpanID)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler not invoked")
	}
	got := rec.Header().Get(TraceParentHeader)
	if !strings.HasPrefix(got, "00-") {
		t.Errorf("response traceparent = %q, want 00-prefix", got)
	}
	if len(got) < 55 {
		t.Errorf("response traceparent too short: %q (%d", got, len(got))
	}
}

func TestTraceMiddleware_PropagatesIncomingSpan(t *testing.T) {
	incoming := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

	var received *SpanContext
	handler := TraceMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = SpanContextFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(TraceParentHeader, incoming)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if received == nil {
		t.Fatal("expected SpanContext in ctx, got nil")
	}
	if received.TraceID != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("TraceID = %q, want propagated", received.TraceID)
	}
	if received.ParentSpanID != "b7ad6b7169203331" {
		t.Errorf("ParentSpanID = %q, want %q", received.ParentSpanID, "b7ad6b7169203331")
	}
	if received.SpanID == "b7ad6b7169203331" {
		t.Errorf("SpanID should differ from parent (new child span)")
	}
}

func TestTraceMiddleware_GeneratesNewOnInvalidHeader(t *testing.T) {
	// Malformed traceparent (less than 55)
	invalid := "invalid"

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(TraceParentHeader, invalid)
	rec := httptest.NewRecorder()

	var received *SpanContext
	handler := TraceMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = SpanContextFromContext(r.Context())
	}))
	handler.ServeHTTP(rec, req)

	if received == nil {
		t.Fatal("expected SpanContext in ctx, got nil")
	}
	if received.ParentSpanID != "" {
		t.Errorf("malformed header should produce root span (no parent), got ParentSpanID = %q", received.ParentSpanID)
	}
}

func TestWithSpanContext_StoresAndRetrieves(t *testing.T) {
	span := &SpanContext{TraceID: "abc", SpanID: "def", Flags: "01"}
	ctx := WithSpanContext(context.Background(), span)

	got := SpanContextFromContext(ctx)
	if got == nil {
		t.Fatal("expected SpanContext in ctx, got nil")
	}
	if got.TraceID != "abc" {
		t.Errorf("TraceID = %q, want abc", got.TraceID)
	}
}

func TestWithSpanContext_NilSpanReturnsSameCtx(t *testing.T) {
	ctx := context.Background()
	got := WithSpanContext(ctx, nil)
	if got != ctx {
		t.Errorf("WithSpanContext(ctx, nil) should return same ctx")
	}
}

func TestLogAttrs_FormatForSlog(t *testing.T) {
	span := &SpanContext{
		TraceID: "abc", SpanID: "def", ParentSpanID: "ghi",
	}
	attrs := LogAttrs(span)
	if len(attrs) != 6 {
		t.Errorf("LogAttrs length = %d, want 6 (3 keys + 3 values)", len(attrs))
	}
}

func TestLogAttrs_NilSpan(t *testing.T) {
	attrs := LogAttrs(nil)
	if attrs != nil {
		t.Errorf("LogAttrs(nil) = %v, want nil", attrs)
	}
}

func TestTraceIDFromContext_BackwardCompat(t *testing.T) {
	// Test that the old TraceIDFromContext API still works (Sprint 18 compat)
	span := &SpanContext{TraceID: "abc123", SpanID: "def456"}
	ctx := WithSpanContext(context.Background(), span)
	if got := TraceIDFromContext(ctx); got != "abc123" {
		t.Errorf("TraceIDFromContext = %q, want abc123", got)
	}
}

func TestTraceIDFromContext_EmptyCtx(t *testing.T) {
	if got := TraceIDFromContext(context.Background()); got != "" {
		t.Errorf("TraceIDFromContext(empty) = %q, want empty", got)
	}
}