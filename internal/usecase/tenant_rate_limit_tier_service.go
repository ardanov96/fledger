// Package usecase - tenant rate limit tier service (Sprint 62).
//
// Resolves effective tier limits per tenant with in-memory cache:
//
//   1. Get(tenantID) → check cache
//   2. On miss → fetch from DB → cache for 60s
//   3. If not found in DB → use TierStandard defaults
//   4. Return a TierLimits struct (concrete values, not Optionals)
//
// Cache invalidation: explicit Invalidate(tenantID) when ops updates
// tier via API/ops tool.
package usecase

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/ratelimit"
)

// TierLimits is the concrete limits after defaults + overrides applied.
// Returned by TenantRateLimitTierService.GetLimits so the middleware
// can use them directly as rate limiter parameters.
//
// Source is for debugging (which path resolved the tier — DB vs default).
type TierLimits struct {
	TierName    domain.TierName
	IPBurst     int
	IPRPS       int
	UserBurst   int
	UserRPS     int
	TenantBurst int
	TenantRPS   int
	Source      string // "db" | "default" — for debugging
}

// TenantRateLimitTierRepo is the minimal interface this service needs.
// Postgres impl lives in internal/repository/postgres/tenant_rate_limit_tier_repo.go.
type TenantRateLimitTierRepo interface {
	GetByTenant(ctx context.Context, tenantID uuid.UUID) (domain.TenantRateLimitTier, error)
}

// TenantRateLimitTierService resolves per-tenant rate limits with caching.
type TenantRateLimitTierService struct {
	repo     TenantRateLimitTierRepo
	cacheTTL time.Duration

	mu    sync.Mutex
	cache map[uuid.UUID]tierCacheEntry
}

type tierCacheEntry struct {
	limits TierLimits
	expiry time.Time
}

// TenantRateLimitTierDeps bundles dependencies.
type TenantRateLimitTierDeps struct {
	Repo     TenantRateLimitTierRepo
	CacheTTL time.Duration // default 60s
}

// NewTenantRateLimitTierService constructs a service.
func NewTenantRateLimitTierService(deps TenantRateLimitTierDeps) *TenantRateLimitTierService {
	ttl := deps.CacheTTL
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &TenantRateLimitTierService{
		repo:     deps.Repo,
		cacheTTL: ttl,
		cache:    make(map[uuid.UUID]tierCacheEntry),
	}
}

// GetLimits returns the effective limits for a tenant. Always returns a
// valid TierLimits (never panics on missing config).
func (s *TenantRateLimitTierService) GetLimits(ctx context.Context, tenantID uuid.UUID) TierLimits {
	// Cache hit
	s.mu.Lock()
	entry, ok := s.cache[tenantID]
	s.mu.Unlock()
	if ok && time.Now().Before(entry.expiry) {
		return entry.limits
	}

	// Cache miss → DB lookup
	tier, err := s.repo.GetByTenant(ctx, tenantID)
	limits := TierLimits{}

	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			// No row in DB → use TierStandard defaults.
			def := domain.GetDefaultTierLimits(domain.TierStandard)
			limits = TierLimits{
				TierName: domain.TierStandard,
				IPBurst: def.IPBurst, IPRPS: def.IPRPS,
				UserBurst: def.UserBurst, UserRPS: def.UserRPS,
				TenantBurst: def.TenantBurst, TenantRPS: def.TenantRPS,
				Source: "default",
			}
		} else {
			// Transient DB error → use defaults, don't cache so we retry.
			def := domain.GetDefaultTierLimits(domain.TierStandard)
			return TierLimits{
				TierName: domain.TierStandard,
				IPBurst: def.IPBurst, IPRPS: def.IPRPS,
				UserBurst: def.UserBurst, UserRPS: def.UserRPS,
				TenantBurst: def.TenantBurst, TenantRPS: def.TenantRPS,
				Source: "default-on-error",
			}
		}
	} else {
		// Apply tier defaults then per-tenant overrides
		tierName := tier.TierName
		if tierName == "" {
			tierName = domain.TierStandard
		}
		def := domain.GetDefaultTierLimits(tierName)
		limits = TierLimits{
			TierName: tierName,
			IPBurst:     tier.EffectiveIPBurst(def.IPBurst),
			IPRPS:       tier.EffectiveIPRPS(def.IPRPS),
			UserBurst:   tier.EffectiveUserBurst(def.UserBurst),
			UserRPS:     tier.EffectiveUserRPS(def.UserRPS),
			TenantBurst: tier.EffectiveTenantBurst(def.TenantBurst),
			TenantRPS:   tier.EffectiveTenantRPS(def.TenantRPS),
			Source:      "db",
		}
	}

	s.mu.Lock()
	s.cache[tenantID] = tierCacheEntry{
		limits: limits,
		expiry: time.Now().Add(s.cacheTTL),
	}
	s.mu.Unlock()
	return limits
}

// Invalidate clears the cache for one tenant (call after ops updates tier).
func (s *TenantRateLimitTierService) Invalidate(tenantID uuid.UUID) {
	s.mu.Lock()
	delete(s.cache, tenantID)
	s.mu.Unlock()
}

// InvalidateAll clears the entire cache (e.g., after mass tier migration).
func (s *TenantRateLimitTierService) InvalidateAll() {
	s.mu.Lock()
	s.cache = make(map[uuid.UUID]tierCacheEntry)
	s.mu.Unlock()
}

// AsTierResolver returns a ratelimit.TierResolver function bound to
// this service. Lets the middleware package consume our service
// without a circular import — middleware just needs the function type.
//
// Returns limits + true when found (or fallback);
// returns limits + false when tenant has no row in DB (use defaults);
// returns zero + false + err on DB error.
func (s *TenantRateLimitTierService) AsTierResolver() func(ctx context.Context, tenantIDStr string) (ratelimit.TierLimits, bool, error) {
	return func(ctx context.Context, tenantIDStr string) (ratelimit.TierLimits, bool, error) {
		tenantID, err := uuid.Parse(tenantIDStr)
		if err != nil {
			return ratelimit.TierLimits{}, false, err
		}
		// Convert local TierLimits to ratelimit.TierLimits (only fields shared).
		local := s.GetLimits(ctx, tenantID)
		return ratelimit.TierLimits{
			TierName:    string(local.TierName),
			IPBurst:     local.IPBurst,
			IPRPS:       local.IPRPS,
			UserBurst:   local.UserBurst,
			UserRPS:     local.UserRPS,
			TenantBurst: local.TenantBurst,
			TenantRPS:   local.TenantRPS,
		}, true, nil
	}
}
