// Package domain - tenant fraud settings (Sprint 38).
//
// One TenantFraudSettings per tenant. NULL fields mean "fall back to the
// deployment-wide FraudConfig default". Operators can override individual
// thresholds without specifying all of them.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// TenantFraudSettings is one row in tenant_fraud_settings.
type TenantFraudSettings struct {
	TenantID               uuid.UUID
	LargeAmountThresholdMinor *int64 // NULL = use env default
	VelocityMaxCount        *int    // NULL = use env default
	VelocityWindowSeconds  *int    // NULL = use env default
	OffHoursStartHour      *int    // NULL = use env default
	OffHoursEndHour        *int    // NULL = use env default
	DisabledRules          []string // empty = all rules enabled
	CreatedAt              time.Time
	UpdatedAt              time.Time
	UpdatedBy              *uuid.UUID
}

// IsRuleDisabled returns true if the rule name is in DisabledRules.
// DisabledRules is checked case-sensitively; invalid rule names are
// logged but not enforced (the rule evaluation skips unknown rules).
func (s TenantFraudSettings) IsRuleDisabled(ruleName string) bool {
	if s.DisabledRules == nil {
		return false
	}
	for _, r := range s.DisabledRules {
		if r == ruleName {
			return true
		}
	}
	return false
}

// EffectiveLargeAmount returns the per-tenant threshold or fallback.
// fallback must be > 0.
func (s TenantFraudSettings) EffectiveLargeAmount(fallback int64) int64 {
	if s.LargeAmountThresholdMinor != nil && *s.LargeAmountThresholdMinor > 0 {
		return *s.LargeAmountThresholdMinor
	}
	return fallback
}

// EffectiveVelocityMax returns the per-tenant max count or fallback.
func (s TenantFraudSettings) EffectiveVelocityMax(fallback int) int {
	if s.VelocityMaxCount != nil && *s.VelocityMaxCount > 0 {
		return *s.VelocityMaxCount
	}
	return fallback
}

// EffectiveVelocityWindowSeconds returns the per-tenant window or fallback.
func (s TenantFraudSettings) EffectiveVelocityWindowSeconds(fallback int) int {
	if s.VelocityWindowSeconds != nil && *s.VelocityWindowSeconds > 0 {
		return *s.VelocityWindowSeconds
	}
	return fallback
}

// EffectiveOffHoursStartHour returns the per-tenant start or fallback.
func (s TenantFraudSettings) EffectiveOffHoursStartHour(fallback int) int {
	if s.OffHoursStartHour != nil && *s.OffHoursStartHour >= 0 && *s.OffHoursStartHour <= 23 {
		return *s.OffHoursStartHour
	}
	return fallback
}

// EffectiveOffHoursEndHour returns the per-tenant end or fallback.
func (s TenantFraudSettings) EffectiveOffHoursEndHour(fallback int) int {
	if s.OffHoursEndHour != nil && *s.OffHoursEndHour >= 0 && *s.OffHoursEndHour <= 23 {
		return *s.OffHoursEndHour
	}
	return fallback
}