// Package middleware — multi-tier rate limiter (Sprint 14 follow-up).
//
// Extends the single-tier `RateLimiter` (Sprint 14) with a chained
// multi-tier limiter that applies 3 independent buckets in sequence:
//
//  1. Per-IP     (defends against anonymous flooding)
//  2. Per-user   (defends against authenticated abuse; bypassed if no JWT)
//  3. Per-tenant (defends against one tenant exhausting shared resources)
//
// All 3 buckets are in-memory token-bucket implementations. For
// multi-instance deployments replace with a Redis-backed store that shares
// state across machines.
//
// Configuration (via env vars):
//
//	RATE_LIMIT_GLOBAL_ENABLED      - master switch (default false)
//	RATE_LIMIT_GLOBAL_BURST        - burst per key per tier (default 100)
//	RATE_LIMIT_GLOBAL_RPS          - sustained refill rate (default 50)
//	RATE_LIMIT_TRANSFER_BURST      - extra-tight burst for /v1/transfers (default 30)
//	RATE_LIMIT_TRANSFER_RPS        - sustained rps for transfers (default 10)
//
// Sprint 22B.1: per-tier allowed/rejected counts are exposed at /metrics via
// the Prometheus counters fmcg_ratelimit_allowed_total and
// fmcg_ratelimit_rejected_total (see prom.go). Backwards-compatible: callers
// that pass nil for the MultiTierLimiterMetrics argument continue to work.
package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// MultiTierLimiter applies N independent RateLimiters, each with its own
// key extractor. A request is allowed only if EVERY tier allows it; if any
// tier rejects, the request is denied and that tier's response is used.
type MultiTierLimiter struct {
	tiers []tier
}

type tier struct {
	name    string
	limiter *RateLimiter
	keyFunc KeyExtractor
}

// KeyExtractor returns the per-tier key for a request. Empty string means
// the tier should be bypassed for this request.
type KeyExtractor func(*http.Request) string

// NewMultiTierLimiter composes the given tiers into a chain. Order matters:
// the first tier that rejects short-circuits the chain.
func NewMultiTierLimiter(tiers ...tier) *MultiTierLimiter {
	return &MultiTierLimiter{tiers: tiers}
}

// Allow runs every tier; if any rejects, returns (false, tierName).
func (m *MultiTierLimiter) Allow(r *http.Request) (allowed bool, rejectedBy string) {
	for _, t := range m.tiers {
		key := t.keyFunc(r)
		if key == "" {
			continue // tier bypassed (e.g. no user_id)
		}
		if !t.limiter.Allow(key) {
			return false, t.name
		}
	}
	return true, ""
}

// MultiTierMiddleware wraps a MultiTierLimiter into an HTTP middleware.
//
// Observability (Sprint 22B.1): the Prometheus counters in prom.go are
// incremented alongside the in-memory MultiTierLimiterMetrics struct, so
// operators can scrape /metrics and dashboards alert on rejection rate per
// tier (ip / user / tenant). Per-tier counters fire only on the tier that
// triggered the decision (reject: the rejecting tier; allow: the lowest-tier
// that was actually evaluated — useful for distinguishing "anonymous user
// allowed by IP" from "authenticated user allowed by tenant").
func MultiTierMiddleware(m *MultiTierLimiter, metrics *MultiTierLimiterMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m == nil {
				next.ServeHTTP(w, r)
				return
			}
			allowed, tierName := m.Allow(r)
			if !allowed {
				IncRatelimitRejected(tierName)
				if metrics != nil {
					metrics.RecordRejected(tierName)
				}
				w.Header().Set("Retry-After", "1")
				w.Header().Set("X-RateLimit-Rejected-By", tierName)
				httpx.Error(w, r, apperrors.ErrTooManyRequests)
				return
			}
			IncRatelimitAllowed(tierName)
			if metrics != nil {
				metrics.RecordAllowed(tierName)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ----- Key extractors -----

// KeyByIP is an alias for RateLimitByIP (consistency with multi-tier naming).
func KeyByIP(r *http.Request) string {
	return RateLimitByIP(r)
}

// KeyByUser returns the user ID extracted from a JWT-style principal header.
// Returns empty string if header missing or unparseable (tier bypassed).
//
// The Principal header is set by RequireAuth middleware before this runs.
// Format: "user_id=<uuid>" (set by middleware/auth.go).
func KeyByUser(r *http.Request) string {
	userID := extractPrincipalField(r, "user_id")
	if userID == "" || userID == "00000000-0000-0000-0000-000000000000" {
		return "" // bypass for service accounts (zero UUID)
	}
	return "user:" + userID
}

// KeyByTenant returns the tenant ID from the Principal header.
// Returns empty string if header missing (tier bypassed).
func KeyByTenant(r *http.Request) string {
	tenantID := extractPrincipalField(r, "tenant_id")
	if tenantID == "" {
		return ""
	}
	return "tenant:" + tenantID
}

// extractPrincipalField reads a key=value pair from the X-Principal header.
// Used because Principal type is not exported (internal/auth middleware).
func extractPrincipalField(r *http.Request, field string) string {
	hdr := r.Header.Get("X-Principal")
	if hdr == "" {
		return ""
	}
	// Format: "user_id=<uuid>;tenant_id=<uuid>;role=<role>"
	for _, part := range strings.Split(hdr, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] == field {
			return kv[1]
		}
	}
	return ""
}

// KeyByPath limits per-endpoint (not per-user). Useful for protecting
// expensive endpoints like /v1/reconciler/run.
func KeyByPath(r *http.Request) string {
	return "path:" + r.URL.Path
}

// ----- Composite factories for common scenarios -----

// NewTransferLimiter returns a MultiTierLimiter configured for /v1/transfers:
//   - Per-IP: 30 burst, 10 rps
//   - Per-user: 30 burst, 10 rps (authenticated tighter cap)
//   - Per-tenant: 300 burst, 100 rps (shared pool, prevents one tenant dominating)
func NewTransferLimiter() *MultiTierLimiter {
	return NewTransferLimiterWithConfig(30, 10, 300, 100)
}

// NewTransferLimiterWithConfig is NewTransferLimiter with overridable per-tier params.
func NewTransferLimiterWithConfig(userBurst, userRps, tenantBurst, tenantRps float64) *MultiTierLimiter {
	ip := NewRateLimiter(30, 10)
	user := NewRateLimiter(userBurst, userRps)
	tenant := NewRateLimiter(tenantBurst, tenantRps)
	return NewMultiTierLimiter(
		tier{"ip", ip, KeyByIP},
		tier{"user", user, KeyByUser},
		tier{"tenant", tenant, KeyByTenant},
	)
}

// NewGlobalLimiter returns a MultiTierLimiter for the entire /v1/* tree:
//   - Per-IP: 100 burst, 50 rps (anonymous flood defense)
//   - Per-user: 100 burst, 50 rps (per-user cap)
//   - Per-tenant: 1000 burst, 500 rps (shared pool)
func NewGlobalLimiter() *MultiTierLimiter {
	return NewGlobalLimiterWithConfig(100, 50, 1000, 500)
}

// NewGlobalLimiterWithConfig is NewGlobalLimiter with overridable per-tier params.
func NewGlobalLimiterWithConfig(ipBurst, ipRps, tenantBurst, tenantRps float64) *MultiTierLimiter {
	ip := NewRateLimiter(ipBurst, ipRps)
	user := NewRateLimiter(ipBurst, ipRps)
	tenant := NewRateLimiter(tenantBurst, tenantRps)
	return NewMultiTierLimiter(
		tier{"ip", ip, KeyByIP},
		tier{"user", user, KeyByUser},
		tier{"tenant", tenant, KeyByTenant},
	)
}

// ----- Per-tier metrics (legacy in-memory; Sprint 22B.1 adds Prometheus) -----

// MultiTierLimiterMetrics provides per-tier counters for observability.
// Kept for backwards compatibility with the /internal/ratelimit-metrics
// JSON endpoint. Prometheus is the primary export path now (see prom.go).
type MultiTierLimiterMetrics struct {
	mu sync.Mutex

	AllowedByTier  map[string]uint64 `json:"allowed_by_tier"`
	RejectedByTier map[string]uint64 `json:"rejected_by_tier"`
}

// NewMultiTierLimiterMetrics returns an initialized metrics struct.
func NewMultiTierLimiterMetrics() *MultiTierLimiterMetrics {
	return &MultiTierLimiterMetrics{
		AllowedByTier:  make(map[string]uint64),
		RejectedByTier: make(map[string]uint64),
	}
}

// RecordAllowed increments the allowed counter for a tier.
func (m *MultiTierLimiterMetrics) RecordAllowed(tier string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AllowedByTier[tier]++
}

// RecordRejected increments the rejected counter for a tier.
func (m *MultiTierLimiterMetrics) RecordRejected(tier string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RejectedByTier[tier]++
}

// Snapshot returns a copy of current counters.
func (m *MultiTierLimiterMetrics) Snapshot() map[string]uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	all := make(map[string]uint64, len(m.AllowedByTier)+len(m.RejectedByTier))
	for k, v := range m.AllowedByTier {
		all["allowed_"+k] = v
	}
	for k, v := range m.RejectedByTier {
		all["rejected_"+k] = v
	}
	return all
}

// MetricsMiddleware exposes the metrics counters in Prometheus format
// (callable as a custom endpoint or appended to /metrics).
func MetricsMiddleware(metrics *MultiTierLimiterMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/internal/ratelimit-metrics" && metrics != nil {
				w.Header().Set("Content-Type", "application/json")
				snap := metrics.Snapshot()
				// simple JSON serialization
				_, _ = w.Write([]byte(formatMetricsJSON(snap)))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func formatMetricsJSON(m map[string]uint64) string {
	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for k, v := range m {
		if !first {
			sb.WriteString(",")
		}
		first = false
		sb.WriteString(`"`)
		sb.WriteString(k)
		sb.WriteString(`":`)
		// manual int formatting (avoid strconv import in hot path)
		sb.WriteString(itoa(v))
	}
	sb.WriteString("}")
	return sb.String()
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// Keep time import used even if unused in some refactors
var _ = time.Second
