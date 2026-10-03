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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/runut/fmcg-wallet/internal/platform/httpx"
	"github.com/runut/fmcg-wallet/internal/ratelimit"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
)

// MultiTierLimiter applies N independent RateLimiters, each with its own
// key extractor. A request is allowed only if EVERY tier allows it; if any
// tier rejects, the request is denied and that tier's response is used.
//
// Sprint 62: optional tierResolver. When non-nil, the "tenant" tier
// dynamically resolves its bucket size + RPS from the resolver (which
// the caller wires to the per-tenant tier service). When nil, falls
// back to the static tenant bucket created at construction time
// (Sprint 14 behavior).
type MultiTierLimiter struct {
	tiers           []tier
	tierResolver    TierResolver // optional, Sprint 62
	tierLimiterMu   sync.Mutex
	tenantLimiters  map[string]*tenantLimiterEntry
}

// Tier is one stage in the MultiTierLimiter chain. Exported (Sprint 62) so
// callers outside the middleware package (e.g. cmd/api/main.go) can compose
// tiers without exposing the unexported struct.
type Tier struct {
	Name    string
	Limiter *RateLimiter
	KeyFunc KeyExtractor
}

// alias for backwards compatibility with internal callers
type tier = Tier

// KeyExtractor returns the per-tier key for a request. Empty string means
// the tier should be bypassed for this request.
type KeyExtractor func(*http.Request) string

// NewMultiTierLimiter composes the given tiers into a chain. Order matters:
// the first tier that rejects short-circuits the chain.
func NewMultiTierLimiter(tiers ...tier) *MultiTierLimiter {
	return &MultiTierLimiter{tiers: tiers}
}

// NewMultiTierLimiterWithTierResolver is NewMultiTierLimiter + per-tenant
// dynamic limits (Sprint 62). When tierResolver is non-nil, the limiter
// uses TierLimits from the resolver (cached 60s per tenant in the caller's
// service). When nil, behaves as NewMultiTierLimiter (uses static buckets).
func NewMultiTierLimiterWithTierResolver(resolver TierResolver, tiers ...tier) *MultiTierLimiter {
	return &MultiTierLimiter{tiers: tiers, tierResolver: resolver}
}

// Allow runs every tier; if any rejects, returns (false, tierName).
//
// Sprint 62: when tierResolver is configured, the "tenant" tier looks up
// per-tenant limits dynamically. Other tiers use their static buckets.
func (m *MultiTierLimiter) Allow(r *http.Request) (allowed bool, rejectedBy string) {
	tenantID := extractPrincipalField(r, "tenant_id")
	var limits TierLimits
	var limitsOK bool
	if m.tierResolver != nil && tenantID != "" {
		var err error
		limits, limitsOK, err = m.tierResolver(r.Context(), tenantID)
		if err != nil {
			// Resolver error → fall back to static buckets for this request.
			limitsOK = false
		}
	}

	for _, t := range m.tiers {
		key := t.KeyFunc(r)
		if key == "" {
			continue // tier bypassed (e.g. no user_id)
		}
		// Sprint 62: for the tenant tier, dynamically resolve the
		// limiter if resolver is configured. Re-bucket when tier config
		// changes (i.e., burst or RPS changes).
		if m.tierResolver != nil && t.Name == "tenant" && limitsOK {
			tierLimiter := m.tenantLimiterFor(tenantID, limits)
			if !tierLimiter.Allow(key) {
				return false, t.Name
			}
			continue
		}
		if !t.Limiter.Allow(key) {
			return false, t.Name
		}
	}
	return true, ""
}

// tenantLimiterFor returns (and caches) a per-tenant RateLimiter sized
// according to the resolved TierLimits. Sprint 62.
func (m *MultiTierLimiter) tenantLimiterFor(tenantID string, limits TierLimits) *RateLimiter {
	// Simple inline cache: rate limiters keyed by tenant.
	// For typical 100-1000 tenants a mutex+map is fine.
	m.tierLimiterMu.Lock()
	defer m.tierLimiterMu.Unlock()
	if m.tenantLimiters == nil {
		m.tenantLimiters = make(map[string]*tenantLimiterEntry)
	}
	if entry, ok := m.tenantLimiters[tenantID]; ok &&
		entry.burst == float64(limits.TenantBurst) &&
		entry.rps == float64(limits.TenantRPS) {
		return entry.limiter
	}
	rl := NewRateLimiter(float64(limits.TenantBurst), float64(limits.TenantRPS))
	entry := tenantLimiterEntry{
		limiter: rl,
		burst:   float64(limits.TenantBurst),
		rps:     float64(limits.TenantRPS),
	}
	m.tenantLimiters[tenantID] = &entry
	return rl
}

// tenantLimiterEntry caches the bucket + its dimensions so we can
// detect config changes (and rebuild the bucket) when limits change.
type tenantLimiterEntry struct {
	limiter *RateLimiter
	burst   float64
	rps     float64
}

// MultiTierMiddleware wraps a MultiTierLimiter into an HTTP middleware.
//
// Observability (Sprint 22B.1 + Sprint 61): the Prometheus counters in
// prom.go are incremented alongside the in-memory MultiTierLimiterMetrics
// struct. Sprint 61 adds tenant_id label so ops can alert per-tenant.
//
// Per-tier counters fire only on the tier that triggered the decision
// (reject: the rejecting tier; allow: the lowest-tier that was actually
// evaluated — useful for distinguishing "anonymous user allowed by IP"
// from "authenticated user allowed by tenant").
func MultiTierMiddleware(m *MultiTierLimiter, metrics *MultiTierLimiterMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m == nil {
				next.ServeHTTP(w, r)
				return
			}
			allowed, tierName := m.Allow(r)
			tenantID := extractPrincipalField(r, "tenant_id") // Sprint 61: per-tenant metric label
			if !allowed {
				IncRatelimitRejected(tierName, tenantID)
				if metrics != nil {
					metrics.RecordRejected(tierName)
				}
				w.Header().Set("Retry-After", "1")
				w.Header().Set("X-RateLimit-Rejected-By", tierName)
				httpx.Error(w, r, apperrors.ErrTooManyRequests)
				return
			}
			IncRatelimitAllowed(tierName, tenantID)
			if metrics != nil {
				metrics.RecordAllowed(tierName)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TierLimits and TierResolver are defined in internal/ratelimit/types.go
// (Sprint 62) so the usecase package can produce them and the middleware
// package can consume them without a circular import. Re-aliased here for
// readability.
type TierLimits = ratelimit.TierLimits
type TierResolver = ratelimit.TierResolver

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

// NewLoginLimiter returns a MultiTierLimiter configured for /v1/auth/login
// (Sprint 61: tighter than global; per-IP + per-tenant to prevent
// credential stuffing from one tenant blocking others).
//   - Per-IP:    5 burst, 0.5 rps (Sprint 14 legacy)
//   - Per-tenant: 20 burst, 2 rps (Sprint 61 new — shared pool)
//
// The login limiter is intentionally stricter than the global limiter
// because account-enumeration attacks target /v1/auth/login specifically.
func NewLoginLimiter() *MultiTierLimiter {
	return NewLoginLimiterWithConfig(5, 0.5, 20, 2)
}

// NewLoginLimiterWithConfig is NewLoginLimiter with overridable params.
func NewLoginLimiterWithConfig(ipBurst, ipRps, tenantBurst, tenantRps float64) *MultiTierLimiter {
	ip := NewRateLimiter(ipBurst, ipRps)
	tenant := NewRateLimiter(tenantBurst, tenantRps)
	return NewMultiTierLimiter(
		tier{"ip", ip, KeyByIP},
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
		sb.WriteString(strconv.FormatUint(v, 10))
	}
	sb.WriteString("}")
	return sb.String()
}

// Keep time import used even if unused in some refactors
var _ = time.Second
