//go:build !windows
// +build !windows

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/auth/jwt"
)

func fakeVerifier(t *testing.T) (Verifier, *jwt.Signer) {
	t.Helper()
	provider := jwt.StaticSecret{Value: []byte("test-secret-do-not-use-in-prod-32bytes!")}
	signer := jwt.NewSigner(provider)
	verifier := jwt.NewVerifier(provider)
	return verifier, signer
}

func mintToken(t *testing.T, signer *jwt.Signer, overrides func(*jwt.Claims)) string {
	t.Helper()
	c := jwt.Claims{UserID: "user-123", Tenant: "tenant-a", Role: "hq_admin"}
	if overrides != nil {
		overrides(&c)
	}
	tok, err := signer.Sign(c, time.Hour)
	require.NoError(t, err)
	return tok
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	t.Parallel()
	v, _ := fakeVerifier(t)
	h := AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := serveReq(h, "GET", "/", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	t.Parallel()
	v, _ := fakeVerifier(t)
	h := AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	cases := []string{"TokenOnly", "Bearer", "Bearer ", "Basic dXNlcjpwYXNz", ""}
	for _, auth := range cases {
		rr := serveReqWithAuth(h, "GET", "/", nil, auth)
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "auth=%q", auth)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	t.Parallel()
	v, s := fakeVerifier(t)
	h := AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Mint an already-expired token.
	c := jwt.Claims{UserID: "u", Tenant: "t", Role: "r"}
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	c.IssuedAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))
	tok, err := s.Sign(c, 0)
	require.NoError(t, err)

	rr := serveReqWithAuth(h, "GET", "/", nil, "Bearer "+tok)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthMiddleware_InvalidSignature(t *testing.T) {
	t.Parallel()
	v, _ := fakeVerifier(t)

	otherSigner := jwt.NewSigner(jwt.StaticSecret{Value: []byte("different-secret-than-verifier!")})
	bad := mintToken(t, otherSigner, nil)

	h := AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := serveReqWithAuth(h, "GET", "/", nil, "Bearer "+bad)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	t.Parallel()
	v, s := fakeVerifier(t)
	var gotPrincipal *Principal
	h := AuthMiddleware(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrincipal = PrincipalFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	tok := mintToken(t, s, func(c *jwt.Claims) {
		c.UserID = "alice"
		c.Tenant = "tenant-1"
		c.Role = "hq_finance"
		c.Scopes = []string{"read"}
	})
	rr := serveReqWithAuth(h, "GET", "/", nil, "Bearer "+tok)
	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, gotPrincipal)
	assert.Equal(t, "alice", gotPrincipal.UserID)
	assert.Equal(t, "tenant-1", gotPrincipal.TenantID)
	assert.Equal(t, "hq_finance", gotPrincipal.Role)
}

func TestRequireAuth_PassesWhenAuthenticated(t *testing.T) {
	t.Parallel()
	v, s := fakeVerifier(t)
	called := false
	h := RequireAuth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		require.NotNil(t, PrincipalFromContext(r.Context()))
		w.WriteHeader(http.StatusOK)
	}))

	tok := mintToken(t, s, nil)
	rr := serveReqWithAuth(h, "GET", "/", nil, "Bearer "+tok)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called)
}

func TestRequirePermission_NoPrincipal_Returns401(t *testing.T) {
	t.Parallel()
	v, _ := fakeVerifier(t)
	h := RequirePermission(v, nil, "create", "invoice")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := serveReq(h, "GET", "/", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func serveReq(h http.Handler, method, path string, body *strings.Reader) *httptest.ResponseRecorder {
	return serveReqWithAuth(h, method, path, body, "")
}

func serveReqWithAuth(h http.Handler, method, path string, body *strings.Reader, auth string) *httptest.ResponseRecorder {
	bodyReader := body
	if bodyReader == nil {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}
