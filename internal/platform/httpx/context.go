package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// Context key types (unexported to prevent collisions).
type (
	requestIDKey struct{}
	traceIDKey   struct{}
	userIDKey    struct{}
	tenantIDKey  struct{}
)

// =============================================================================
// Context accessors
// =============================================================================

// WithRequestID stores a request ID in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// GetRequestID returns the request ID from the context, or "" if none.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// WithTraceID stores a trace ID in the context.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, id)
}

// GetTraceID returns the trace ID from the context, or "" if none.
func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey{}).(string); ok {
		return v
	}
	return ""
}

// WithUserID stores a user ID in the context (set by auth middleware).
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

// GetUserID returns the user ID from the context, or "" if none.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey{}).(string); ok {
		return v
	}
	return ""
}

// WithTenantID stores a tenant ID in the context (set by tenant middleware).
func WithTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, tenantIDKey{}, id)
}

// GetTenantID returns the tenant ID from the context, or "" if none.
func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(tenantIDKey{}).(string); ok {
		return v
	}
	return ""
}

// =============================================================================
// Middleware
// =============================================================================

// RequestIDMiddleware extracts or generates a request ID, attaches it to the
// context and response headers, and propagates it to all downstream handlers.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = GenerateID()
		}
		w.Header().Set("X-Request-ID", id)

		ctx := WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GenerateID returns a 16-character hex string backed by crypto/rand.
// Suitable for request IDs, audit entry IDs, etc. Collision-resistant for
// the volume a single API instance handles.
func GenerateID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should never fail in practice.
		return "fallback-err"
	}
	return hex.EncodeToString(b[:])
}
