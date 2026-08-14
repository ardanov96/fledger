// Package middleware — rate limit tests (Sprint 14).
//
//go:build !windows
// +build !windows

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_BurstAllowance(t *testing.T) {
	l := NewRateLimiter(5, 1) // capacity 5, refill 1/s
	for i := 0; i < 5; i++ {
		assert.True(t, l.Allow("key1"), "request %d within burst should be allowed", i+1)
	}
	assert.False(t, l.Allow("key1"), "6th request should be denied (bucket empty)")
}

func TestRateLimiter_Refill(t *testing.T) {
	l := NewRateLimiter(2, 10) // capacity 2, refill 10/s (fast)
	assert.True(t, l.Allow("k"))
	assert.True(t, l.Allow("k"))
	assert.False(t, l.Allow("k"))

	// Sleep 200ms → 2 tokens added (10/s × 0.2s = 2).
	time.Sleep(200 * time.Millisecond)

	assert.True(t, l.Allow("k"), "after refill, should allow again")
	assert.True(t, l.Allow("k"), "after refill, second token should allow")
	assert.False(t, l.Allow("k"), "third request after refill should be denied")
}

func TestRateLimiter_KeyIsolation(t *testing.T) {
	l := NewRateLimiter(2, 0) // capacity 2, no refill
	assert.True(t, l.Allow("alice"))
	assert.True(t, l.Allow("alice"))
	assert.False(t, l.Allow("alice"))

	// bob has a separate bucket — should still work.
	assert.True(t, l.Allow("bob"))
	assert.True(t, l.Allow("bob"))
	assert.False(t, l.Allow("bob"))
}

func TestRateLimitMiddleware_PassThrough(t *testing.T) {
	// Limiter nil → middleware is no-op.
	mw := RateLimitMiddleware(nil, RateLimitByIP)
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimitMiddleware_BlocksAtLimit(t *testing.T) {
	l := NewRateLimiter(1, 0) // 1 burst, no refill
	mw := RateLimitMiddleware(l, RateLimitByIP)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request: allowed.
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "1.2.3.4:5000"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Second request from same IP: denied.
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "1.2.3.4:5001"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
	assert.Equal(t, "1", rr2.Header().Get("Retry-After"))
}

func TestRateLimitMiddleware_DifferentKeysIndependent(t *testing.T) {
	l := NewRateLimiter(1, 0)
	mw := RateLimitMiddleware(l, RateLimitByIP)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, ip := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip + ":5000"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "IP %s should be allowed (separate bucket)", ip)
	}
}

func TestRateLimiter_NoRefillBelowCapacity(t *testing.T) {
	l := NewRateLimiter(10, 0)
	for i := 0; i < 5; i++ {
		l.Allow("k")
	}
	time.Sleep(100 * time.Millisecond)
	// Without refill, after using 5 we should still have 5 left.
	assert.True(t, l.Allow("k"))
	assert.True(t, l.Allow("k"))
}
