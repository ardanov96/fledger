// Package usecase - tests for TenantRateLimitTierService (Sprint 62).
package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/domain"
)

// =============================================================================
// Fake repo
// =============================================================================

type fakeTierRepo struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]domain.TenantRateLimitTier
	err    error
	calls  int
}

func (r *fakeTierRepo) GetByTenant(_ context.Context, id uuid.UUID) (domain.TenantRateLimitTier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.err != nil {
		return domain.TenantRateLimitTier{}, r.err
	}
	s, ok := r.byID[id]
	if !ok {
		return domain.TenantRateLimitTier{}, apperrors.ErrNotFound
	}
	return s, nil
}

// =============================================================================
// Tests
// =============================================================================

func TestTenantRateLimitService_FallsBackToDefaultsWhenNoRow(t *testing.T) {
	repo := &fakeTierRepo{byID: map[uuid.UUID]domain.TenantRateLimitTier{}}
	svc := NewTenantRateLimitTierService(TenantRateLimitTierDeps{
		Repo:     repo,
		CacheTTL: 60 * time.Second,
	})
	tenantID := uuid.New()
	limits := svc.GetLimits(context.Background(), tenantID)

	def := domain.GetDefaultTierLimits(domain.TierStandard)
	assert.Equal(t, def.IPBurst, limits.IPBurst)
	assert.Equal(t, def.IPRPS, limits.IPRPS)
	assert.Equal(t, def.TenantBurst, limits.TenantBurst)
	assert.Equal(t, "default", limits.Source, "expected source=default when no row in DB")
}

func TestTenantRateLimitService_RespectsOverrides(t *testing.T) {
	customBurst := 999
	customRPS := 88
	tenantID := uuid.New()
	repo := &fakeTierRepo{byID: map[uuid.UUID]domain.TenantRateLimitTier{
		tenantID: {
			TenantID:         tenantID,
			TierName:         domain.TierPremium,
			CustomTenantBurst: &customBurst,
			CustomTenantRPS:   &customRPS,
		},
	}}
	svc := NewTenantRateLimitTierService(TenantRateLimitTierDeps{
		Repo:     repo,
		CacheTTL: 60 * time.Second,
	})
	limits := svc.GetLimits(context.Background(), tenantID)
	assert.Equal(t, 999, limits.TenantBurst, "CustomTenantBurst should override")
	assert.Equal(t, 88, limits.TenantRPS, "CustomTenantRPS should override")
	// Other fields use premium tier defaults
	def := domain.GetDefaultTierLimits(domain.TierPremium)
	assert.Equal(t, def.IPBurst, limits.IPBurst, "non-overridden fields use tier defaults")
	assert.Equal(t, "db", limits.Source)
}

func TestTenantRateLimitService_CachesResults(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeTierRepo{
		byID: map[uuid.UUID]domain.TenantRateLimitTier{
			tenantID: {TenantID: tenantID, TierName: domain.TierStandard},
		},
	}
	svc := NewTenantRateLimitTierService(TenantRateLimitTierDeps{
		Repo:     repo,
		CacheTTL: 60 * time.Second,
	})

	// First call → DB
	_ = svc.GetLimits(context.Background(), tenantID)
	repo.mu.Lock()
	firstCalls := repo.calls
	repo.mu.Unlock()
	require.Equal(t, 1, firstCalls)

	// Second call → cache (no DB hit)
	_ = svc.GetLimits(context.Background(), tenantID)
	repo.mu.Lock()
	secondCalls := repo.calls
	repo.mu.Unlock()
	assert.Equal(t, 1, secondCalls, "second call should hit cache, not DB")
}

func TestTenantRateLimitService_InvalidateRefreshesCache(t *testing.T) {
	customBurst1 := 100
	customBurst2 := 200
	tenantID := uuid.New()
	repo := &fakeTierRepo{
		byID: map[uuid.UUID]domain.TenantRateLimitTier{
			tenantID: {TenantID: tenantID, TierName: domain.TierStandard, CustomTenantBurst: &customBurst1},
		},
	}
	svc := NewTenantRateLimitTierService(TenantRateLimitTierDeps{Repo: repo, CacheTTL: 60 * time.Second})

	limits := svc.GetLimits(context.Background(), tenantID)
	require.Equal(t, 100, limits.TenantBurst)

	// Ops updates tier — Invalidate cache
	repo.mu.Lock()
	repo.byID[tenantID] = domain.TenantRateLimitTier{
		TenantID: tenantID, TierName: domain.TierStandard, CustomTenantBurst: &customBurst2,
	}
	repo.mu.Unlock()
	svc.Invalidate(tenantID)

	// Next call should see new value
	limits = svc.GetLimits(context.Background(), tenantID)
	assert.Equal(t, 200, limits.TenantBurst)
}

func TestTenantRateLimitService_DBErrorUsesDefaults(t *testing.T) {
	repo := &fakeTierRepo{err: errors.New("connection refused")}
	svc := NewTenantRateLimitTierService(TenantRateLimitTierDeps{Repo: repo, CacheTTL: 60 * time.Second})
	tenantID := uuid.New()

	limits := svc.GetLimits(context.Background(), tenantID)
	assert.Equal(t, "default-on-error", limits.Source, "DB error → defaults + error tag")
	def := domain.GetDefaultTierLimits(domain.TierStandard)
	assert.Equal(t, def.IPBurst, limits.IPBurst)
}

func TestTenantRateLimitService_AsTierResolverReturnsCorrectType(t *testing.T) {
	repo := &fakeTierRepo{byID: map[uuid.UUID]domain.TenantRateLimitTier{}}
	svc := NewTenantRateLimitTierService(TenantRateLimitTierDeps{Repo: repo, CacheTTL: 60 * time.Second})

	resolver := svc.AsTierResolver()
	require.NotNil(t, resolver)

	limits, ok, err := resolver(t.Context(), uuid.New().String())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "standard", limits.TierName, "default tier is 'standard'")
	assert.Greater(t, limits.TenantBurst, 0, "defaults should produce positive burst")
}