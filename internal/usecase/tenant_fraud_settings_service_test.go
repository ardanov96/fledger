// Package usecase - tests for TenantFraudSettingsService (Sprint 38).
package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/domain"
	"github.com/runut/fmcg-wallet/internal/platform/config"
)

// =============================================================================
// Fake repo
// =============================================================================

type fakeTenantFraudSettingsRepo struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]domain.TenantFraudSettings
	err    error
}

func (r *fakeTenantFraudSettingsRepo) GetByTenant(_ context.Context, id uuid.UUID) (domain.TenantFraudSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return domain.TenantFraudSettings{}, r.err
	}
	s, ok := r.byID[id]
	if !ok {
		return domain.TenantFraudSettings{}, apperrors.ErrNotFound
	}
	return s, nil
}

// =============================================================================
// Tests
// =============================================================================

func TestTenantFraudSettings_FallsBackToDefaultsWhenNoRow(t *testing.T) {
	repo := &fakeTenantFraudSettingsRepo{byID: map[uuid.UUID]domain.TenantFraudSettings{}}
	svc := NewTenantFraudSettingsService(TenantFraudSettingsDeps{
		Repo:     repo,
		Fallback: config.FraudConfig{
			LargeAmountThresholdMinor: 50_000_000,
			VelocityMaxCount:          10,
			VelocityWindow:            5 * time.Minute,
			OffHoursStart:             6,
			OffHoursEnd:               22,
		},
	})

	tenantID := uuid.New()
	got := svc.EffectiveLargeAmount(context.Background(), tenantID)
	if got != 50_000_000 {
		t.Errorf("LargeAmount fallback = %d, want 50M", got)
	}
}

func TestTenantFraudSettings_UsesPerTenantOverride(t *testing.T) {
	customThreshold := int64(500_000_000) // 500M
	tenantID := uuid.New()
	repo := &fakeTenantFraudSettingsRepo{byID: map[uuid.UUID]domain.TenantFraudSettings{
		tenantID: {
			TenantID:                tenantID,
			LargeAmountThresholdMinor: &customThreshold,
		},
	}}
	svc := NewTenantFraudSettingsService(TenantFraudSettingsDeps{
		Repo:     repo,
		Fallback: config.FraudConfig{LargeAmountThresholdMinor: 50_000_000},
	})

	got := svc.EffectiveLargeAmount(context.Background(), tenantID)
	if got != customThreshold {
		t.Errorf("LargeAmount = %d, want %d (per-tenant override)", got, customThreshold)
	}
}

func TestTenantFraudSettings_CacheInvalidation(t *testing.T) {
	customThreshold := int64(500_000_000)
	tenantID := uuid.New()
	repo := &fakeTenantFraudSettingsRepo{byID: map[uuid.UUID]domain.TenantFraudSettings{
		tenantID: {
			TenantID:                tenantID,
			LargeAmountThresholdMinor: &customThreshold,
		},
	}}
	svc := NewTenantFraudSettingsService(TenantFraudSettingsDeps{Repo: repo})

	// First call: cache miss → DB lookup
	got := svc.EffectiveLargeAmount(context.Background(), tenantID)
	if got != customThreshold {
		t.Fatalf("first call: got %d, want %d", got, customThreshold)
	}

	// Change DB (simulate operator update)
	repo.mu.Lock()
	newThreshold := int64(1_000_000_000)
	repo.byID[tenantID] = domain.TenantFraudSettings{
		TenantID:                tenantID,
		LargeAmountThresholdMinor: &newThreshold,
	}
	repo.mu.Unlock()

	// Without invalidation, cache still returns old value
	got2 := svc.EffectiveLargeAmount(context.Background(), tenantID)
	if got2 != customThreshold {
		t.Errorf("cached call: got %d, want %d (cached)", got2, customThreshold)
	}

	// After invalidation, returns new value
	svc.Invalidate(tenantID)
	got3 := svc.EffectiveLargeAmount(context.Background(), tenantID)
	if got3 != newThreshold {
		t.Errorf("post-invalidate call: got %d, want %d", got3, newThreshold)
	}
}

func TestTenantFraudSettings_DBErrorReturnsDefaults(t *testing.T) {
	repo := &fakeTenantFraudSettingsRepo{err: apperrors.ErrInvalidInput} // transient error
	svc := NewTenantFraudSettingsService(TenantFraudSettingsDeps{
		Repo:     repo,
		Fallback: config.FraudConfig{LargeAmountThresholdMinor: 99_000_000},
	})
	got := svc.EffectiveLargeAmount(context.Background(), uuid.New())
	if got != 99_000_000 {
		t.Errorf("on DB error, expected fallback 99M, got %d", got)
	}
}

func TestTenantFraudSettings_DisabledRules(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeTenantFraudSettingsRepo{byID: map[uuid.UUID]domain.TenantFraudSettings{
		tenantID: {
			TenantID:     tenantID,
			DisabledRules: []string{"velocity", "off_hours"},
		},
	}}
	svc := NewTenantFraudSettingsService(TenantFraudSettingsDeps{Repo: repo})
	settings := svc.GetForTenant(context.Background(), tenantID)

	if !settings.IsRuleDisabled("velocity") {
		t.Errorf("expected velocity to be disabled")
	}
	if !settings.IsRuleDisabled("off_hours") {
		t.Errorf("expected off_hours to be disabled")
	}
	if settings.IsRuleDisabled("large_amount") {
		t.Errorf("large_amount should NOT be disabled")
	}
}

func TestTenantFraudSettings_PartialOverrideUsesDefaultsForRest(t *testing.T) {
	// Only override off_hours_start_hour, expect other fields to use defaults.
	customStart := 8
	tenantID := uuid.New()
	repo := &fakeTenantFraudSettingsRepo{byID: map[uuid.UUID]domain.TenantFraudSettings{
		tenantID: {
			TenantID:          tenantID,
			OffHoursStartHour: &customStart,
		},
	}}
	svc := NewTenantFraudSettingsService(TenantFraudSettingsDeps{
		Repo: repo,
		Fallback: config.FraudConfig{
			LargeAmountThresholdMinor: 50_000_000,
			VelocityMaxCount:          10,
			OffHoursStart:             6,
			OffHoursEnd:               22,
		},
	})

	start, end := svc.EffectiveOffHours(context.Background(), tenantID)
	if start != customStart {
		t.Errorf("OffHoursStart = %d, want %d (override)", start, customStart)
	}
	if end != 22 {
		t.Errorf("OffHoursEnd = %d, want 22 (fallback)", end)
	}

	// LargeAmount should still be fallback
	if got := svc.EffectiveLargeAmount(context.Background(), tenantID); got != 50_000_000 {
		t.Errorf("LargeAmount = %d, want 50M (fallback)", got)
	}
}