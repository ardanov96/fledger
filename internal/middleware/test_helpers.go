//go:build !windows
// +build !windows

package middleware

import (
	"testing"

	"github.com/runut/fmcg-wallet/internal/auth/jwt"
)

// stubVerifier returns a Verifier that accepts any non-empty token and
// returns a fixed Principal. Used by tests that only need AuthMiddleware
// to set the principal — they don't care about JWT validity.
type stubVerifier struct{}

// Verify satisfies the Verifier interface.
func (stubVerifier) Verify(token string) (*jwt.Claims, error) {
	if token == "" {
		return nil, jwt.ErrTokenInvalid
	}
	return &jwt.Claims{
		UserID: "test-user",
		Tenant: "00000000-0000-0000-0000-000000000001",
		Role:   "hq_admin",
		Scopes: []string{"read", "write"},
	}, nil
}

// stubVerifierFor returns a stub Verifier for tests.
func stubVerifierFor(t *testing.T) Verifier {
	t.Helper()
	return stubVerifier{}
}
</content><task_progress>- [x] All Sprint 9A files (incl. test_helpers.go)
- [ ] Wire to main.go (cmd/api/main.go + add JWT verifier + Casbin enforcer setup)
- [ ] Verify build
- [ ] Commit Sprint 9A + push</task_progress></task_progress>