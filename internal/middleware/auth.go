// Package middleware contains HTTP middleware for the API.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"

	"github.com/runut/fmcg-wallet/internal/auth/jwt"
	"github.com/runut/fmcg-wallet/internal/auth/rbac"
)

// Principal represents the authenticated user/service from the JWT.
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

// Verifier abstracts the JWT verifier so tests can inject a fake.
type Verifier interface {
	Verify(token string) (*jwt.Claims, error)
}

// AuthMiddleware extracts the Bearer token, verifies it, and attaches a
// Principal to the request context. On any failure it returns 401.
func AuthMiddleware(verifier Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
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

			claims, err := verifier.Verify(token)
			if err != nil {
				switch {
				case errors.Is(err, jwt.ErrTokenExpired):
					httpx.Error(w, r, errUnauthorized("token expired"))
				case errors.Is(err, jwt.ErrTokenInvalid), errors.Is(err, jwt.ErrTokenMalformed):
					httpx.Error(w, r, errUnauthorized("invalid token"))
				default:
					httpx.Error(w, r, errUnauthorized("authentication failed"))
				}
				return
			}

			principal := &Principal{
				UserID:   claims.UserID,
				TenantID: claims.Tenant,
				Role:     claims.Role,
				Scopes:   claims.Scopes,
			}
			ctx := WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth wraps AuthMiddleware + ensures a principal is present.
func RequireAuth(verifier Verifier) func(http.Handler) http.Handler {
	auth := AuthMiddleware(verifier)
	return func(next http.Handler) http.Handler {
		return auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if PrincipalFromContext(r.Context()) == nil {
				httpx.Error(w, r, errUnauthorized("no principal in context"))
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

// RequirePermission enforces a Casbin RBAC check after authentication.
func RequirePermission(verifier Verifier, enforcer *rbac.Enforcer, action, object string) func(http.Handler) http.Handler {
	auth := AuthMiddleware(verifier)
	return func(next http.Handler) http.Handler {
		return auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := PrincipalFromContext(r.Context())
			if p == nil {
				httpx.Error(w, r, errUnauthorized("no principal in context"))
				return
			}
			ok, err := enforcer.Check(p.UserID, p.Role, p.TenantID, action, object)
			if err != nil {
				httpx.Error(w, r, errInternal("rbac check failed"))
				return
			}
			if !ok {
				httpx.Error(w, r, errForbidden(action, object))
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

func errUnauthorized(msg string) error {
	return apperrors.New("UNAUTHORIZED", msg, http.StatusUnauthorized)
}

func errForbidden(action, object string) error {
	return apperrors.New("FORBIDDEN", "permission denied: "+action+" on "+object, http.StatusForbidden)
}

func errInternal(msg string) error {
	return apperrors.New("INTERNAL", msg, http.StatusInternalServerError)
}
