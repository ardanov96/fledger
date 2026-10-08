// Package ratelimit - shared types for the rate-limit subsystem.
//
// Sprint 62 introduced per-tenant rate-limit tiers. Both the
// middleware package (which consumes TierLimits) and the usecase
// package (which produces them via TenantRateLimitTierService) need
// to reference these types. Without this shared package we'd have a
// circular import (usecase → middleware → usecase).
//
// Putting the shared types in a small leaf package breaks the cycle:
//   domain ← no internal deps
//   platform ← no internal deps
//   ratelimit ← no internal deps (this file)
//   middleware ← ratelimit, domain, platform
//   usecase ← ratelimit, domain, platform, repository
package ratelimit

import "context"

// TierLimits holds the resolved rate-limit parameters for one tenant.
// Returned by TenantRateLimitTierService.GetLimits (usecase layer) and
// consumed by MultiTierLimiter (middleware layer).
//
// Sprint 62: NULL columns in tenant_rate_limit_tiers fall back to
// TierDefaults (defined in internal/domain/tenant_rate_limit_tier.go).
type TierLimits struct {
	TierName    string
	IPBurst     int
	IPRPS       int
	UserBurst   int
	UserRPS     int
	TenantBurst int
	TenantRPS   int
}

// TierResolver returns the effective limits for a tenant (Sprint 62).
// Implementations live in usecase (TenantRateLimitTierService) and are
// passed to NewMultiTierLimiterWithTierResolver in cmd/api.
//
// Returns:
//   - limits + true + nil: tenant resolved (from DB or fallback)
//   - limits + false + nil: tenant not found, use defaults
//   - zero + false + err: DB error, fallback to defaults
type TierResolver func(ctx context.Context, tenantID string) (TierLimits, bool, error)