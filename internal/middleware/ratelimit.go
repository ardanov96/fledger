// Package middleware — rate limit (Sprint 14 / Fase 2D).
//
// In-memory token-bucket rate limiter middleware. Suitable for single-instance
// deployments; for multi-instance replace with Redis-backed implementation.
//
// Behavior: per-key (IP / user / tenant) bucket that refills tokens at a
// steady rate. When the bucket is empty, requests return 429.
package middleware

import (
	"net/http"
	"sync"
	"time"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// RateLimiter is the per-key bucket state.
type RateLimiter struct {
	mu sync.Mutex

	// capacity is the maximum burst size.
	capacity float64

	// refillPerSecond is the steady-state refill rate.
	refillPerSecond float64

	// buckets maps key to current token count + last refill time.
	buckets map[string]*rlBucket
}

type rlBucket struct {
	tokens    float64
	lastRefil time.Time
}

// NewRateLimiter creates an in-memory token-bucket limiter.
//
//   capacity         - max burst tokens (e.g. 20)
//   refillPerSecond   - tokens added per second (e.g. 5 -> 5 req/s sustained, burst 20)
func NewRateLimiter(capacity, refillPerSecond float64) *RateLimiter {
	return &RateLimiter{
		capacity:       capacity,
		refillPerSecond: refillPerSecond,
		buckets:        make(map[string]*rlBucket),
	}
}

// Allow returns true if the request is allowed (token consumed).
func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		b = &rlBucket{tokens: l.capacity, lastRefil: now}
		l.buckets[key] = b
	}

	// Refill (continuous rate).
	elapsed := now.Sub(b.lastRefil).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.refillPerSecond
		if b.tokens > l.capacity {
			b.tokens = l.capacity
		}
		b.lastRefil = now
	}

	if b.tokens < 1.0 {
		return false
	}
	b.tokens -= 1.0
	return true
}

// RateLimitMiddleware returns an HTTP middleware that limits by key extractor.
// If rate limiter is nil, the middleware is a no-op (passthrough).
func RateLimitMiddleware(limiter *RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}
			key := keyFunc(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !limiter.Allow(key) {
				w.Header().Set("Retry-After", "1")
				httpx.Error(w, r, apperrors.ErrTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitByIP is a convenience extractor using X-Forwarded-For or RemoteAddr.
func RateLimitByIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}

// RateLimitConfig is the configurable knobs.
type RateLimitConfig struct {
	Enabled         bool
	Capacity        float64
	RefillPerSecond float64
}

// DefaultRateLimitConfig returns sensible defaults.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:         false, // off by default; enable via env var
		Capacity:        20,    // burst of 20 requests
		RefillPerSecond: 5,     // sustained 5 req/s
	}
}
