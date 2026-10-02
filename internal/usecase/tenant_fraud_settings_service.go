// Package usecase - tenant fraud settings service (Sprint 38).
//
// Resolves effective thresholds per tenant with in-memory cache:
//
//   1. Get(tenantID) → check cache
//   2. On miss → fetch from DB
//   3. If not found in DB → use env defaults (empty struct)
//   4. Cache for 60s (configurable) — invalidate on Upsert/Delete
//
// The fraud service (Sprint 31) calls GetForTenant and uses the returned
// settings' EffectiveX methods with fallback values from config.FraudConfig.
package usecase

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/domain"
	"github.com/runut/fmcg-wallet/internal/platform/config"
)

// TenantFraudSettingsService resolves per-tenant fraud thresholds with caching.
type TenantFraudSettingsService struct {
	repo    tenantFraudSettingsRepo
	fallback config.FraudConfig
	cacheTTL time.Duration

	mu    sync.Mutex
	cache map[uuid.UUID]cacheEntry
}

// tenantFraudSettingsRepo is the minimal interface this service needs.
// Postgres impl lives in internal/repository/postgres/tenant_fraud_settings_repo.go.
type tenantFraudSettingsRepo interface {
	GetByTenant(ctx context.Context, tenantID uuid.UUID) (domain.TenantFraudSettings, error)
}

type cacheEntry struct {
	settings domain.TenantFraudSettings
	expiry   time.Time
}

// TenantFraudSettingsDeps bundles service deps.
type TenantFraudSettingsDeps struct {
	Repo     tenantFraudSettingsRepo
	Fallback config.FraudConfig
	CacheTTL time.Duration // default 60s
}

// NewTenantFraudSettingsService constructs a service.
func NewTenantFraudSettingsService(deps TenantFraudSettingsDeps) *TenantFraudSettingsService {
	ttl := deps.CacheTTL
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &TenantFraudSettingsService{
		repo:     deps.Repo,
		fallback: deps.Fallback,
		cacheTTL: ttl,
		cache:    make(map[uuid.UUID]cacheEntry),
	}
}

// GetForTenant returns the effective settings for one tenant. Cached for
// CacheTTL. Never returns an error from the DB (errors fall back to
// defaults); callers always get a usable struct.
func (s *TenantFraudSettingsService) GetForTenant(ctx context.Context, tenantID uuid.UUID) domain.TenantFraudSettings {
	// Fast path: cache hit
	s.mu.Lock()
	entry, ok := s.cache[tenantID]
	s.mu.Unlock()
	if ok && time.Now().Before(entry.expiry) {
		return entry.settings
	}

	// Slow path: DB lookup
	settings, err := s.repo.GetByTenant(ctx, tenantID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			settings = domain.TenantFraudSettings{} // all defaults
		} else {
			// Transient DB error → return defaults but DON'T cache
			// so we retry on the next call
			return domain.TenantFraudSettings{}
		}
	}

	s.mu.Lock()
	s.cache[tenantID] = cacheEntry{
		settings: settings,
		expiry:   time.Now().Add(s.cacheTTL),
	}
	s.mu.Unlock()
	return settings
}

// EffectiveLargeAmount returns the per-tenant threshold or env fallback.
func (s *TenantFraudSettingsService) EffectiveLargeAmount(ctx context.Context, tenantID uuid.UUID) int64 {
	settings := s.GetForTenant(ctx, tenantID)
	return settings.EffectiveLargeAmount(s.fallback.LargeAmountThresholdMinor)
}

// EffectiveVelocityMax returns the per-tenant max count or env fallback.
func (s *TenantFraudSettingsService) EffectiveVelocityMax(ctx context.Context, tenantID uuid.UUID) int {
	settings := s.GetForTenant(ctx, tenantID)
	return settings.EffectiveVelocityMax(s.fallback.VelocityMaxCount)
}

// EffectiveVelocityWindow returns the per-tenant window or env fallback.
func (s *TenantFraudSettingsService) EffectiveVelocityWindow(ctx context.Context, tenantID uuid.UUID) time.Duration {
	settings := s.GetForTenant(ctx, tenantID)
	secs := settings.EffectiveVelocityWindowSeconds(int(s.fallback.VelocityWindow.Seconds()))
	return time.Duration(secs) * time.Second
}

// EffectiveOffHours returns (start, end) hours per-tenant or env fallback.
func (s *TenantFraudSettingsService) EffectiveOffHours(ctx context.Context, tenantID uuid.UUID) (start, end int) {
	settings := s.GetForTenant(ctx, tenantID)
	return settings.EffectiveOffHoursStartHour(s.fallback.OffHoursStart),
		settings.EffectiveOffHoursEndHour(s.fallback.OffHoursEnd)
}

// Invalidate clears the cache for one tenant (call after Upsert/Delete).
func (s *TenantFraudSettingsService) Invalidate(tenantID uuid.UUID) {
	s.mu.Lock()
	delete(s.cache, tenantID)
	s.mu.Unlock()
}

// InvalidateAll clears the entire cache (rare; for ops tools).
func (s *TenantFraudSettingsService) InvalidateAll() {
	s.mu.Lock()
	s.cache = make(map[uuid.UUID]cacheEntry)
	s.mu.Unlock()
}