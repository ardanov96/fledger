// Package middleware contains HTTP middleware for the API.
//
// AuthMiddleware is a scaffold for Fase 2 — currently a stub that accepts
// any non-empty Bearer token and extracts a fake principal. Replace with
// real JWT validation in Fase 2 (golang-jwt/jwt/v5).
package middleware

import (
	"context"
	"net/http"
	"strings"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// Principal represents the authenticated user/service from the JWT.
// In Fase 2 this comes from a verified JWT token; for now it's a stub.
type Principal struct {
	UserID   string
	TenantID string
	Role     string
	Scopes   []string
}

type principalCtxKey struct{}

// WithPrincipal stores the principal in the context.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// PrincipalFromContext returns the principal from the context, or nil.
func PrincipalFromContext(ctx context.Context) *Principal {
	if p, ok := ctx.Value(principalCtxKey{}).(*Principal); ok {
		return p
	}
	return nil
}

// AuthMiddleware extracts the bearer token from the Authorization header
// and validates it. Currently a STUB that accepts any non-empty token.
//
// TODO Fase 2: replace stub with real JWT validation.
//  1. Parse token (jwt.Parse with HMAC + secret from config)
//  2. Verify signature, expiry, audience, issuer
//  3. Extract claims: user_id, tenant_id, role, scopes
//  4. Build Principal and attach to context
//  5. On failure, return 401 UNAUTHORIZED
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			httpx.Error(w, r, errUnauthorized("missing Authorization header"))
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			httpx.Error(w, r, errUnauthorized("invalid Authorization format; expected 'Bearer <token>'"))
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			httpx.Error(w, r, errUnauthorized("empty bearer token"))
			return
		}

		// STUB: in Fase 2, replace with real JWT validation
		principal := &Principal{
			UserID:   "user-from-token-" + truncate(token, 8),
			TenantID: "00000000-0000-0000-0000-000000000001",
			Role:     "user",
			Scopes:   []string{"read", "write"},
		}

		ctx := WithPrincipal(r.Context(), principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// errUnauthorized creates a typed AppError with HTTP 401.
func errUnauthorized(msg string) error {
	return apperrors.New("UNAUTHORIZED", msg, http.StatusUnauthorized)
}

// truncate returns first n chars of s for use in stub principal IDs.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// RequireAuth wraps AuthMiddleware + ensures a principal is present.
// Use on routes that REQUIRE auth (most business endpoints).
func RequireAuth(next http.Handler) http.Handler {
	return AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := PrincipalFromContext(r.Context())
		if p == nil {
			httpx.Error(w, r, errUnauthorized("no principal in context"))
			return
		}
		next.ServeHTTP(w, r)
	}))
}
