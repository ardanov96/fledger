// Package fraud — built-in rules engine (Sprint 31 / Fase 8).
//
// All rules are pure functions of TransferEvent (and an optional clock
// for OffHoursRule). They take no I/O dependencies, so they can be unit
// tested with zero infrastructure.
//
// To add a new rule:
//  1. Implement `fraud.Rule`
//  2. Add the rule's name to the DB CHECK in migration 000024
//  3. Wire it into `FraudScannerService.Rules()` in fraud_service.go
package fraud

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// LargeAmountRule
// =============================================================================

// LargeAmountRule flags transfers above a tenant-configurable threshold
// (default 50_000_000 IDR minor = Rp 50M ≈ $3k USD). Severity: critical.
type LargeAmountRule struct {
	ThresholdMinor int64
}

func (r LargeAmountRule) Name() RuleName { return RuleLargeAmount }

func (r LargeAmountRule) Evaluate(_ context.Context, ev TransferEvent) ([]Match, error) {
	if r.ThresholdMinor <= 0 {
		return nil, fmt.Errorf("LargeAmountRule: threshold must be > 0 (got %d)", r.ThresholdMinor)
	}
	if ev.AmountMinor >= r.ThresholdMinor {
		return []Match{{
			RuleName: RuleLargeAmount,
				Severity: SeverityCritical,
				Evidence: map[string]any{
					"amount_minor":  ev.AmountMinor,
					"threshold_minor": r.ThresholdMinor,
					"currency":      ev.Currency,
				},
			}}, nil
	}
	return nil, nil
}

// =============================================================================
// OffHoursRule
// =============================================================================

// OffHoursRule flags transfers outside business hours. Business hours
// are defined as a window in 24-hour local time (default 06:00-22:00).
// Severity: warn (suspicious but not necessarily fraud — could be late
// reconciliation work).
type OffHoursRule struct {
	StartHour int // inclusive, 0-23
	EndHour   int // exclusive, 0-23
	Now       func() time.Time // optional; defaults to time.Now in the local TZ
}

func (r OffHoursRule) Name() RuleName { return RuleOffHours }

func (r OffHoursRule) Evaluate(_ context.Context, ev TransferEvent) ([]Match, error) {
	if r.StartHour < 0 || r.StartHour > 23 || r.EndHour < 0 || r.EndHour > 23 {
		return nil, fmt.Errorf("OffHoursRule: hours must be in [0,23] (got start=%d end=%d)", r.StartHour, r.EndHour)
	}
	now := r.Now
	if now == nil {
		now = time.Now
	}
	hour := now().Hour()
	if hour < r.StartHour || hour >= r.EndHour {
		return []Match{{
			RuleName: RuleOffHours,
				Severity: SeverityWarn,
				Evidence: map[string]any{
					"transfer_at_hour":   ev.OccurredAt.Hour(),
					"business_start_hour": r.StartHour,
					"business_end_hour":   r.EndHour,
				},
			}}, nil
	}
	return nil, nil
}

// =============================================================================
// VelocityRule
// =============================================================================

// VelocityRule flags accounts that post N+ transfers in a rolling window.
// Severity: critical.
//
// Implementation note (Sprint 31): this rule needs DB I/O to count
// recent transfers for the account. We expose the lookup as a function
// so the rule stays pure-ish (the function is injected by the service
// layer which knows how to query postgres). For tests, callers inject
// a stub.
type VelocityRule struct {
	MaxCount       int           // e.g. 10
	Window         time.Duration // e.g. 5 * time.Minute
	LookupCount    func(ctx context.Context, tenantID, accountID uuid.UUID, window time.Duration) (int, error)
}

func (r VelocityRule) Name() RuleName { return RuleVelocity }

func (r VelocityRule) Evaluate(ctx context.Context, ev TransferEvent) ([]Match, error) {
	if r.MaxCount <= 0 {
		return nil, fmt.Errorf("VelocityRule: MaxCount must be > 0 (got %d)", r.MaxCount)
	}
	if r.Window <= 0 {
		return nil, fmt.Errorf("VelocityRule: Window must be > 0 (got %v)", r.Window)
	}
	if r.LookupCount == nil {
		return nil, fmt.Errorf("VelocityRule: LookupCount not configured")
	}
	count, err := r.LookupCount(ctx, ev.TenantID, ev.FromAccount, r.Window)
	if err != nil {
		return nil, fmt.Errorf("velocity lookup: %w", err)
	}
	// count includes the current event (which is already in DB since
	// outbox event was emitted by the transfer service). Threshold breach
	// when count >= MaxCount.
	if count >= r.MaxCount {
		return []Match{{
			RuleName: RuleVelocity,
				Severity: SeverityCritical,
				Evidence: map[string]any{
					"count":    count,
					"window_minutes": int(r.Window.Minutes()),
					"max_count":      r.MaxCount,
				},
			}}, nil
	}
	return nil, nil
}

// =============================================================================
// FirstTimeRecipientRule
// =============================================================================

// FirstTimeRecipientRule flags transfers to an account the sender has
// never transferred to before. Severity: info (low signal, mainly used
// to populate the operator's UI for review).
type FirstTimeRecipientRule struct {
	HasHistory func(ctx context.Context, tenantID, fromAccount, toAccount uuid.UUID) (bool, error)
}

func (r FirstTimeRecipientRule) Name() RuleName { return RuleFirstTimeRecipient }

func (r FirstTimeRecipientRule) Evaluate(ctx context.Context, ev TransferEvent) ([]Match, error) {
	if r.HasHistory == nil {
		return nil, fmt.Errorf("FirstTimeRecipientRule: HasHistory not configured")
	}
	has, err := r.HasHistory(ctx, ev.TenantID, ev.FromAccount, ev.ToAccount)
	if err != nil {
		return nil, fmt.Errorf("first-time recipient lookup: %w", err)
	}
	if !has {
		return []Match{{
			RuleName: RuleFirstTimeRecipient,
				Severity: SeverityInfo,
				Evidence: map[string]any{
					"from_account": ev.FromAccount.String(),
					"to_account":   ev.ToAccount.String(),
				},
			}}, nil
	}
	return nil, nil
}