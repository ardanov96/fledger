// Package middleware — TenantContextMiddleware tests (Sprint 15).
//
//go:build !windows
// +build !windows

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/platform/tenantctx"
)

// helper: build a request with Principal already in context.
func reqWithPrincipal(p *Principal) *http.Request {
	ctx := WithPrincipal(context.Background(), p)
	return httptest.NewRequest("GET", "/", nil).WithContext(ctx)
}

func TestTenantContextMiddleware_AttachesInfo(t *testing.T) {
	tenantID := uuid.New().String()
	userID := uuid.New().String()
	p := &Principal{
		UserID:   userID,
		TenantID: tenantID,
		Role:     "hq_admin",
	}

	var captured *tenantctx.Info
	handler := TenantContextMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = tenantctx.InfoFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, reqWithPrincipal(p))

	require.NotNil(t, captured, "middleware must attach *tenantctx.Info to context")
	assert.Equal(t, tenantID, captured.TenantID.String())
	assert.Equal(t, userID, captured.UserID.String())
	assert.False(t, captured.IsSalesRep)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTenantContextMiddleware_SalesRepFlag(t *testing.T) {
	p := &Principal{
		UserID:   uuid.New().String(),
		TenantID: uuid.New().String(),
		Role:     "sales_rep",
	}

	var captured *tenantctx.Info
	handler := TenantContextMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = tenantctx.InfoFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, reqWithPrincipal(p))

	require.NotNil(t, captured)
	assert.True(t, captured.IsSalesRep, "sales_rep role must set IsSalesRep=true")
}

func TestTenantContextMiddleware_NoPrincipal_Returns500(t *testing.T) {
	handler := TenantContextMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil) // no Principal
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestTenantContextMiddleware_BadTenantUUID_Returns500(t *testing.T) {
	p := &Principal{
		UserID:   uuid.New().String(),
		TenantID: "not-a-uuid",
		Role:     "hq_admin",
	}

	handler := TenantContextMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, reqWithPrincipal(p))

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestTenantContextMiddleware_MissingTenant_Returns500(t *testing.T) {
	p := &Principal{
		UserID:   uuid.New().String(),
		// TenantID intentionally empty
		Role: "hq_admin",
	}

	handler := TenantContextMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, reqWithPrincipal(p))

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestTenantContextMiddleware_EmptyUserID_Allowed(t *testing.T) {
	// Service-account tokens may have empty user_id; middleware must accept.
	p := &Principal{
		UserID:   "",
		TenantID: uuid.New().String(),
		Role:     "hq_admin",
	}

	var captured *tenantctx.Info
	handler := TenantContextMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = tenantctx.InfoFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, reqWithPrincipal(p))

	require.NotNil(t, captured)
	assert.Equal(t, uuid.Nil, captured.UserID, "empty user_id should be zero UUID")
}

func TestTenantInfoFromContext_NilWhenAbsent(t *testing.T) {
	// Without middleware, InfoFromContext must return nil (not panic).
	got := tenantctx.InfoFromContext(context.Background())
	assert.Nil(t, got)
}

func TestInfoFromPrincipal_AllFields(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	info, err := infoFromPrincipal(&Principal{
		UserID:   userID.String(),
		TenantID: tenantID.String(),
		Role:     "outlet_manager",
	})
	require.NoError(t, err)
	assert.Equal(t, tenantID, info.TenantID)
	assert.Equal(t, userID, info.UserID)
	assert.False(t, info.IsSalesRep)
}
