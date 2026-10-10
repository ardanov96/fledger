// Package middleware contains HTTP middleware for Fledger Dunning.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/fledger/fledger-dunning/internal/auth/jwt"
	apperrors "github.com/fledger/fledger-dunning/internal/platform/errors"
	"github.com/fledger/fledger-dunning/internal/platform/httpx"
)

// DefaultTenantID is the fallback tenant id used when the request omits
// X-Tenant-ID (handy for local UI smoke tests and the embedded portal).
const DefaultTenantID = "a0000000-0000-0000-0000-000000000001"

type Principal struct {
	UserID   string
	TenantID string
	Role     string
	Scopes   []string
}

type principalCtxKey struct{}

func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

func PrincipalFromContext(ctx context.Context) *Principal {
	if p, ok := ctx.Value(principalCtxKey{}).(*Principal); ok {
		return p
	}
	return nil
}

type Verifier interface {
	Verify(token string) (*jwt.Claims, error)
}

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

// TenantFromContext returns the tenant id from the authenticated principal,
// or the DefaultTenantID fallback (e.g. for the embedded web portal which
// runs unauthenticated).
func TenantFromContext(ctx context.Context) string {
	if p := PrincipalFromContext(ctx); p != nil && p.TenantID != "" {
		return p.TenantID
	}
	return DefaultTenantID
}

// TenantFromRequest reads the X-Tenant-ID header with the default fallback.
func TenantFromRequest(r *http.Request) string {
	if h := r.Header.Get("X-Tenant-ID"); h != "" {
		return h
	}
	return DefaultTenantID
}