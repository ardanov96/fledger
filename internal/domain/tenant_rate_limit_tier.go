// Package domain - tenant rate limit tier (Sprint 62).
//
// One TenantRateLimitTier per tenant. NULL columns fall back to deployment-
// wide env defaults (RATE_LIMIT_GLOBAL_BURST/RPS).
//
// Tiers:
//   - free:        minimum RPS, hard caps
//   - standard:    default for new tenants
//   - premium:     higher RPS for paying customers
//   - enterprise:  custom (set per-customer by ops)
package domain

import (
	"time"

	"github.com/google/uuid"
)

// TierName is the canonical tier identifier.
type TierName string

const (
	TierFree       TierName = "free"
	TierStandard   TierName = "standard"
	TierPremium    TierName = "premium"
	TierEnterprise TierName = "enterprise"
)

// TenantRateLimitTier is one row in tenant_rate_limit_tiers.
type TenantRateLimitTier struct {
	TenantID     uuid.UUID
	TierName     TierName

	// Per-tier rate limit overrides. NULL = use env default.
	CustomIPBurst      *int
	CustomIPRPS        *int
	CustomUserBurst    *int
	CustomUserRPS      *int
	CustomTenantBurst  *int
	CustomTenantRPS    *int

	Notes     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// =============================================================================
// Effective accessors — return override OR fallback. Caller provides fallback.
// =============================================================================

// EffectiveIPBurst returns the per-tenant override or the fallback.
// Returns int (not *int) for direct use as limiter parameter.
func (t TenantRateLimitTier) EffectiveIPBurst(fallback int) int {
	if t.CustomIPBurst != nil && *t.CustomIPBurst > 0 {
		return *t.CustomIPBurst
	}
	return fallback
}

func (t TenantRateLimitTier) EffectiveIPRPS(fallback int) int {
	if t.CustomIPRPS != nil && *t.CustomIPRPS > 0 {
		return *t.CustomIPRPS
	}
	return fallback
}

func (t TenantRateLimitTier) EffectiveUserBurst(fallback int) int {
	if t.CustomUserBurst != nil && *t.CustomUserBurst > 0 {
		return *t.CustomUserBurst
	}
	return fallback
}

func (t TenantRateLimitTier) EffectiveUserRPS(fallback int) int {
	if t.CustomUserRPS != nil && *t.CustomUserRPS > 0 {
		return *t.CustomUserRPS
	}
	return fallback
}

func (t TenantRateLimitTier) EffectiveTenantBurst(fallback int) int {
	if t.CustomTenantBurst != nil && *t.CustomTenantBurst > 0 {
		return *t.CustomTenantBurst
	}
	return fallback
}

func (t TenantRateLimitTier) EffectiveTenantRPS(fallback int) int {
	if t.CustomTenantRPS != nil && *t.CustomTenantRPS > 0 {
		return *t.CustomTenantRPS
	}
	return fallback
}

// =============================================================================
// Default tier limits (Sprint 62: tier-based defaults when no row in DB)
// =============================================================================
//
// These are the deployment defaults for each tier. Used when no
// tenant_rate_limit_tiers row exists OR all override columns are NULL.
// Override via env vars at process startup (existing config).
type TierDefaults struct {
	IPBurst     int
	IPRPS       int
	UserBurst   int
	UserRPS     int
	TenantBurst int
	TenantRPS   int
}

// GetDefaultTierLimits returns sensible defaults for each tier.
// Operators can override these via custom columns in the DB.
func GetDefaultTierLimits(tier TierName) TierDefaults {
	switch tier {
	case TierFree:
		return TierDefaults{
			IPBurst: 20, IPRPS: 5,
			UserBurst: 30, UserRPS: 5,
			TenantBurst: 100, TenantRPS: 20,
		}
	case TierPremium:
		return TierDefaults{
			IPBurst: 200, IPRPS: 100,
			UserBurst: 500, UserRPS: 200,
			TenantBurst: 5000, TenantRPS: 1000,
		}
	case TierEnterprise:
		return TierDefaults{
			IPBurst: 1000, IPRPS: 500,
			UserBurst: 2000, UserRPS: 1000,
			TenantBurst: 20000, TenantRPS: 5000,
		}
	default: // TierStandard + unknown
		return TierDefaults{
			IPBurst: 100, IPRPS: 50,
			UserBurst: 100, UserRPS: 50,
			TenantBurst: 1000, TenantRPS: 500,
		}
	}
}