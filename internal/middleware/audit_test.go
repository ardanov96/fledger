//go:build !windows
// +build !windows

package middleware
import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/runut/fmcg-wallet/internal/domain/audit"
)
func TestAuditMiddleware_RecordsRequest(t *testing.T) {
	t.Parallel()
	repo := audit.NewMemoryRepository()
	mw := AuditMiddleware(repo, slog.New(slog.NewTextHandler(discardWriter{}, nil)))
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("POST", "/v1/transfers", strings.NewReader("{}"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	got, err := repo.List(req.Context(), "00000000-0000-0000-0000-000000000001", 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	e := got[0]
	assert.Equal(t, audit.Action("transfers.create"), e.Action)
	assert.Equal(t, "transfers", e.ResourceType)
	assert.Equal(t, audit.OutcomeSuccess, e.Outcome)
	assert.Equal(t, "POST", e.Metadata["method"])
	assert.Equal(t, "/v1/transfers", e.Metadata["path"])
}
func TestAuditMiddleware_RecordsFailureOn5xx(t *testing.T) {
	t.Parallel()
	repo := audit.NewMemoryRepository()
	mw := AuditMiddleware(repo, slog.New(slog.NewTextHandler(discardWriter{}, nil)))
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	req := httptest.NewRequest("GET", "/v1/accounts", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	got, _ := repo.List(req.Context(), "any", 10)
	require.Len(t, got, 1)
	assert.Equal(t, audit.OutcomeFailure, got[0].Outcome)
}
func TestAuditMiddleware_SkipsHealthPaths(t *testing.T) {
	t.Parallel()
	repo := audit.NewMemoryRepository()
	mw := AuditMiddleware(repo, slog.New(slog.NewTextHandler(discardWriter{}, nil)))
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, path := range []string{"/healthz", "/readyz", "/version", "/metrics"} {
		req := httptest.NewRequest("GET", path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
	got, _ := repo.List(nil, "any", 100)
	assert.Empty(t, got, "health/metrics paths should not be audited")
}
func TestAuditMiddleware_AttachesPrincipal(t *testing.T) {
	t.Parallel()
	repo := audit.NewMemoryRepository()
	mw := AuditMiddleware(repo, slog.New(slog.NewTextHandler(discardWriter{}, nil)))
	handler := mw(AuthMiddleware(stubVerifierFor(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
<task_progress>- [x] All Sprint 9A files (incl. test_helpers.go)
- [ ] Wire to main.go (cmd/api/main.go + add JWT verifier + Casbin enforcer setup)
- [ ] Verify build
		w.WriteHeader(http.StatusOK)
	})))
	req := httptest.NewRequest("GET", "/v1/accounts", nil)
	req.Header.Set("Authorization", "Bearer anything-here-stubVerifier-accepts")
<task_progress>- [x] All Sprint 9A files
- [ ] Fix audit_test.go signature + add stubVerifier helper
- [ ] Wire to main.go (cmd/api/main.go + add JWT verifier + Casbin enforcer setup)
- [ ] Verify build
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	got, _ := repo.List(req.Context(), "any", 10)
	require.Len(t, got, 1)
	assert.NotEmpty(t, got[0].ActorID, "actor_id should be set from principal")
	assert.Equal(t, audit.ActorUser, got[0].ActorType)
}
func TestDeriveAction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		method, path, want string
	}{
		{"POST", "/v1/transfers", "transfers.create"},
		{"GET", "/v1/transfers", "transfers.list"},
		{"GET", "/v1/transfers/abc", "transfers.get"},
		{"POST", "/v1/accounts", "accounts.create"},
		{"GET", "/v1/accounts", "accounts.list"},
		{"GET", "/v1/accounts/abc/entries", "accounts.get"},
		{"DELETE", "/v1/accounts/abc", "accounts.delete"},
		{"GET", "/", "root.get"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, deriveAction(tt.method, tt.path))
		})
	}
}
func TestDeriveResource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path      string
		wantType  string
		wantID    string
	}{
		{"/v1/accounts", "accounts", ""},
		{"/v1/accounts/abc-123", "accounts", "abc-123"},
		{"/v1/accounts/abc-123/entries", "accounts", "abc-123"},
		{"/v1/transfers/tx-1", "transfers", "tx-1"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			gotType, gotID := deriveResource(tt.path)
			assert.Equal(t, tt.wantType, gotType)
			assert.Equal(t, tt.wantID, gotID)
		})
	}
}
type discardWriter struct{}
func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
