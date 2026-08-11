//go:build !windows
// +build !windows

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	t.Parallel()
	h := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := serveReq(h, "GET", "/", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assertErrorCode(t, rr, http.StatusUnauthorized)
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	t.Parallel()
	h := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []string{
		"TokenOnly",                  // no Bearer prefix
		"Bearer",                     // no token
		"Bearer ",                    // empty token
		"Basic dXNlcjpwYXNz",         // wrong scheme
		"",                           // empty
	}

	for _, auth := range tests {
		rr := serveReqWithAuth(h, "GET", "/", nil, auth)
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "auth=%q", auth)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	t.Parallel()
	var gotPrincipal *Principal
	h := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrincipal = PrincipalFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := serveReqWithAuth(h, "GET", "/", nil, "Bearer test-token-12345")
	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, gotPrincipal, "principal should be set")
	assert.NotEmpty(t, gotPrincipal.UserID)
	assert.NotEmpty(t, gotPrincipal.TenantID)
	assert.Equal(t, "user", gotPrincipal.Role)
}

func TestRequireAuth_PassesWhenAuthenticated(t *testing.T) {
	t.Parallel()
	called := false
	h := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		p := PrincipalFromContext(r.Context())
		require.NotNil(t, p)
		assert.Equal(t, "user", p.Role)
		w.WriteHeader(http.StatusOK)
	}))

	rr := serveReqWithAuth(h, "GET", "/", nil, "Bearer valid-token")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called)
}

// helpers

func serveReq(h http.Handler, method, path string, body *strings.Reader) *httptest.ResponseRecorder {
	return serveReqWithAuth(h, method, path, body, "")
}

func serveReqWithAuth(h http.Handler, method, path string, body *strings.Reader, auth string) *httptest.ResponseRecorder {
	var bodyReader *strings.Reader
	if body != nil {
		bodyReader = body
	} else {
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

func assertErrorCode(t *testing.T, rr *httptest.ResponseRecorder, expected int) {
	t.Helper()
	var env struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env), "body: %s", rr.Body.String())
	require.NotNil(t, env.Error, "expected error envelope")
	// For now, just verify the status matches and there's a code.
	assert.Equal(t, expected, rr.Code)
}
