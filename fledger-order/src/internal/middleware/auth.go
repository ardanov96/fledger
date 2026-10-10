// Package middleware contains HTTP middleware for Fledger Order.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/fledger/fledger-order/internal/auth/jwt"
	apperrors "github.com/fledger/fledger-order/internal/platform/errors"
	"github.com/fledger/fledger-order/internal/platform/httpx"
)

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