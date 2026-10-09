// Package middleware contains HTTP middleware for the Fledger Fleet API.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/fledger/fledger-fleet/internal/auth/jwt"
	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/platform/httpx"
)

// Principal represents the authenticated caller. Mirrors fledger-core.
type Principal struct {
	UserID   string
	TenantID string
	Role     string
	Scopes   []string
}

type principalCtxKey struct{}

// WithPrincipal stores the principal in the request context.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// PrincipalFromContext returns the principal, or nil if unauthenticated.
func PrincipalFromContext(ctx context.Context) *Principal {
	if p, ok := ctx.Value(principalCtxKey{}).(*Principal); ok {
		return p
	}
	return nil
}

// Verifier is the subset of jwt.Verifier needed here.
type Verifier interface {
	Verify(token string) (*jwt.Claims, error)
}

// RequireAuth validates the Bearer token and attaches a Principal.
func RequireAuth(v Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get("Authorization")
			if hdr == "" {
				httpx.Error(w, r, apperrors.ErrUnauthorizedMissing)
				return
			}
			parts := strings.SplitN(hdr, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				httpx.Error(w, r, apperrors.ErrUnauthorizedMissing.WithDetail(map[string]any{
					"reason": "expected 'Bearer <token>'",
				}))
				return
			}
			claims, err := v.Verify(strings.TrimSpace(parts[1]))
			if err != nil {
				switch {
				case errors.Is(err, jwt.ErrTokenExpired):
					httpx.Error(w, r, apperrors.ErrUnauthorizedMissing.WithDetail(map[string]any{"reason": "token expired"}))
				case errors.Is(err, jwt.ErrTokenInvalid), errors.Is(err, jwt.ErrTokenMalformed):
					httpx.Error(w, r, apperrors.ErrUnauthorizedMissing.WithDetail(map[string]any{"reason": "invalid token"}))
				default:
					httpx.Error(w, r, apperrors.ErrUnauthorizedMissing.WithDetail(map[string]any{"reason": "verification failed"}))
				}
				return
			}
			p := &Principal{
				UserID:   claims.UserID,
				TenantID: claims.Tenant,
				Role:     claims.Role,
				Scopes:   claims.Scopes,
			}
			ctx := WithPrincipal(r.Context(), p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope rejects callers missing any of the required scopes.
func RequireScope(required ...string) func(http.Handler) http.Handler {
	want := make(map[string]struct{}, len(required))
	for _, s := range required {
		want[s] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := PrincipalFromContext(r.Context())
			if p == nil {
				httpx.Error(w, r, apperrors.ErrUnauthorizedMissing)
				return
			}
			for _, s := range required {
				found := false
				for _, have := range p.Scopes {
					if have == s {
						found = true
						break
					}
				}
				if !found {
					_ = want // keep linter happy if want unused
					httpx.Error(w, r, apperrors.ErrForbiddenAction.WithDetail(map[string]any{
						"required_scope": s,
					}))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}