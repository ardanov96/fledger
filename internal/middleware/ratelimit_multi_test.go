// Package middleware - MultiTierLimiter tests.
//
//go:build !windows
// +build !windows

package middleware

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestMultiTierLimiter_AllowAllTiersPass verifies that a request passes
// when all tier buckets have tokens.
func TestMultiTierLimiter_AllowAllTiersPass(t *testing.T) {
	m := NewMultiTierLimiter(
		tier{"ip", NewRateLimiter(10, 5), KeyByIP},
		tier{"user", NewRateLimiter(10, 5), KeyByUser},
		tier{"tenant", NewRateLimiter(10, 5), KeyByTenant},
	)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		req.Header.Set("X-Principal", "user_id=u-1;tenant_id=t-1;role=hq_admin")
		allowed, rejectedBy := m.Allow(req)
		if !allowed {
			t.Errorf("iter %d: expected allowed, got rejectedBy=%s", i, rejectedBy)
		}
	}
}

// TestMultiTierLimiter_IPTierRejects verifies that the IP tier rejects
// once its token bucket is empty, even though user+tenant still have tokens.
func TestMultiTierLimiter_IPTierRejects(t *testing.T) {
	ip := NewRateLimiter(2, 0.5) // tiny bucket, slow refill
	user := NewRateLimiter(100, 100)
	tenant := NewRateLimiter(100, 100)
	m := NewMultiTierLimiter(
		tier{"ip", ip, KeyByIP},
		tier{"user", user, KeyByUser},
		tier{"tenant", tenant, KeyByTenant},
	)

	// Drain IP bucket: 3 attempts should all fail at the IP tier.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.5:9999"
		req.Header.Set("X-Principal", "user_id=u-1;tenant_id=t-1")
		allowed, rejectedBy := m.Allow(req)
		if allowed {
			t.Errorf("iter %d: expected IP rejected, got allowed", i)
		}
		if rejectedBy != "ip" {
			t.Errorf("iter %d: expected rejectedBy=ip, got %s", i, rejectedBy)
		}
	}
}

// TestMultiTierLimiter_EmptyKeyBypassesTier verifies that tiers with
// empty key (e.g. anonymous request with no X-Principal) are skipped
// (so user/tenant tiers don't block public endpoints).
func TestMultiTierLimiter_EmptyKeyBypassesTier(t *testing.T) {
	ip := NewRateLimiter(5, 1)
	user := NewRateLimiter(1, 0.1) // would reject if not bypassed
	tenant := NewRateLimiter(5, 1)
	m := NewMultiTierLimiter(
		tier{"ip", ip, KeyByIP},
		tier{"user", user, KeyByUser},
		tier{"tenant", tenant, KeyByTenant},
	)

	// Anonymous request: no X-Principal header → user tier key is "" → bypass.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.10:9999"
		// no X-Principal header
		allowed, _ := m.Allow(req)
		if !allowed {
			t.Errorf("iter %d: anonymous request should bypass user tier and pass", i)
		}
	}
}

// TestMultiTierLimiter_DifferentKeysIndependent verifies that IP tier
// tracks per-IP-key buckets independently (one IP doesn't affect another).
func TestMultiTierLimiter_DifferentKeysIndependent(t *testing.T) {
	m := NewMultiTierLimiter(
		tier{"ip", NewRateLimiter(2, 0.01), KeyByIP}, // tiny burst, almost no refill
		tier{"user", NewRateLimiter(100, 100), KeyByUser},
		tier{"tenant", NewRateLimiter(100, 100), KeyByTenant},
	)

	// Drain IP1.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Set("X-Principal", "user_id=u-1;tenant_id=t-1")
		_, _ = m.Allow(req)
	}

	// IP2 should still have its own full bucket.
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("X-Principal", "user_id=u-1;tenant_id=t-1")
	allowed, _ := m.Allow(req)
	if !allowed {
		t.Error("IP2 should still have full bucket, independent of IP1 exhaustion")
	}
}

// TestMultiTierLimiter_Metrics verifies per-tier counters via middleware.
func TestMultiTierLimiter_Metrics(t *testing.T) {
	m := NewMultiTierLimiter(
		tier{"ip", NewRateLimiter(100, 100), KeyByIP},
		tier{"user", NewRateLimiter(100, 100), KeyByUser},
		tier{"tenant", NewRateLimiter(100, 100), KeyByTenant},
	)
	metrics := NewMultiTierLimiterMetrics()
	mw := MultiTierMiddleware(m, metrics)

	handler := mw(noopHandler())

	// Fire 3 allowed requests.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.0.1:1234"
		req.Header.Set("X-Principal", "user_id=u-1;tenant_id=t-1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("iter %d: expected 200 OK, got %d", i, rec.Code)
		}
	}

	// Now drain IP tier by spamming same IP.
	ip := NewRateLimiter(2, 0.001)
	ipDrain := NewMultiTierLimiter(
		tier{"ip", ip, KeyByIP},
		tier{"user", NewRateLimiter(100, 100), KeyByUser},
		tier{"tenant", NewRateLimiter(100, 100), KeyByTenant},
	)
	drainMetrics := NewMultiTierLimiterMetrics()
	drainMw := MultiTierMiddleware(ipDrain, drainMetrics)
	drainHandler := drainMw(noopHandler())

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.99.99.99:1234"
		req.Header.Set("X-Principal", "user_id=u-1;tenant_id=t-1")
		rec := httptest.NewRecorder()
		drainHandler.ServeHTTP(rec, req)
	}

	snap := drainMetrics.Snapshot()
	if snap["allowed_ip"]+snap["allowed_user"]+snap["allowed_tenant"] == 0 {
		t.Errorf("expected some allowed counters, got %v", snap)
	}
	if snap["rejected_ip"] == 0 {
		t.Errorf("expected rejected_ip counter > 0, got %v", snap)
	}
	if snap["rejected_ip"] >= 5 {
		t.Errorf("rejected_ip should be ≤5 (all attempts), got %d", snap["rejected_ip"])
	}
}

// noopHandler is a passthrough http.Handler for tests.
func noopHandler() interface {
	ServeHTTP(http.ResponseWriter, *httptest.ResponseRecorder)
} {
	return nil
}

// helper to silence "declared and not used" for `sync` import in some build configs.
var _ = sync.Mutex{}
var _ = time.Second
