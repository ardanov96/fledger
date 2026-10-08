// Package fraud - unit tests for the 4 built-in rules (Sprint 31).
//
// Table-driven tests; no infrastructure. We pass stub lookup functions to
// VelocityRule + FirstTimeRecipientRule so the tests stay hermetic.
package fraud

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// sampleEvent builds a TransferEvent with sensible defaults.
func sampleEvent() TransferEvent {
	return TransferEvent{
		TenantID:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		TransferID:  uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		FromAccount: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		ToAccount:   uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		AmountMinor: 100_000, // Rp 100k
		Currency:    "IDR",
		OccurredAt:  time.Now(),
	}
}

// =============================================================================
// LargeAmountRule
// =============================================================================

func TestLargeAmountRule_BelowThreshold_NoMatch(t *testing.T) {
	r := LargeAmountRule{ThresholdMinor: 1_000_000}
	ev := sampleEvent()
	ev.AmountMinor = 999_999

	matches, err := r.Evaluate(context.Background(), ev)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestLargeAmountRule_AtThreshold_Match(t *testing.T) {
	r := LargeAmountRule{ThresholdMinor: 1_000_000}
	ev := sampleEvent()
	ev.AmountMinor = 1_000_000

	matches, err := r.Evaluate(context.Background(), ev)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].RuleName != RuleLargeAmount {
		t.Errorf("RuleName = %q, want %q", matches[0].RuleName, RuleLargeAmount)
	}
	if matches[0].Severity != SeverityCritical {
		t.Errorf("Severity = %q, want critical", matches[0].Severity)
	}
	if matches[0].Evidence["amount_minor"] != int64(1_000_000) {
		t.Errorf("evidence.amount_minor = %v, want 1000000", matches[0].Evidence["amount_minor"])
	}
}

func TestLargeAmountRule_ZeroThreshold_Error(t *testing.T) {
	r := LargeAmountRule{ThresholdMinor: 0}
	_, err := r.Evaluate(context.Background(), sampleEvent())
	if err == nil {
		t.Errorf("expected error for zero threshold")
	}
}

// =============================================================================
// OffHoursRule
// =============================================================================

func TestOffHoursRule_WithinHours_NoMatch(t *testing.T) {
	// Mock clock at 10:00 (within 6-22)
	r := OffHoursRule{StartHour: 6, EndHour: 22, Now: func() time.Time {
		return time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	}}
	matches, err := r.Evaluate(context.Background(), sampleEvent())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches at 10am, got %d", len(matches))
	}
}

func TestOffHoursRule_OutsideHours_Match(t *testing.T) {
	// Mock clock at 23:00 (outside 6-22)
	r := OffHoursRule{StartHour: 6, EndHour: 22, Now: func() time.Time {
		return time.Date(2026, 10, 1, 23, 0, 0, 0, time.UTC)
	}}
	matches, err := r.Evaluate(context.Background(), sampleEvent())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match at 11pm, got %d", len(matches))
	}
	if matches[0].Severity != SeverityWarn {
		t.Errorf("Severity = %q, want warn", matches[0].Severity)
	}
}

func TestOffHoursRule_BoundaryHourStart_NoMatch(t *testing.T) {
	// 06:00 is INCLUSIVE start — should NOT match
	r := OffHoursRule{StartHour: 6, EndHour: 22, Now: func() time.Time {
		return time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC)
	}}
	matches, _ := r.Evaluate(context.Background(), sampleEvent())
	if len(matches) != 0 {
		t.Errorf("06:00 should be within business hours (inclusive start), got %d matches", len(matches))
	}
}

func TestOffHoursRule_BoundaryHourEnd_Match(t *testing.T) {
	// 22:00 is EXCLUSIVE end — SHOULD match (off-hours)
	r := OffHoursRule{StartHour: 6, EndHour: 22, Now: func() time.Time {
		return time.Date(2026, 10, 1, 22, 0, 0, 0, time.UTC)
	}}
	matches, _ := r.Evaluate(context.Background(), sampleEvent())
	if len(matches) != 1 {
		t.Errorf("22:00 should be off-hours (exclusive end), got %d matches", len(matches))
	}
}

func TestOffHoursRule_InvalidHours_Error(t *testing.T) {
	r := OffHoursRule{StartHour: -1, EndHour: 22}
	_, err := r.Evaluate(context.Background(), sampleEvent())
	if err == nil {
		t.Errorf("expected error for negative hour")
	}
}

// =============================================================================
// VelocityRule
// =============================================================================

func TestVelocityRule_BelowMax_NoMatch(t *testing.T) {
	r := VelocityRule{
		MaxCount: 10,
		Window:   5 * time.Minute,
		LookupCount: func(_ context.Context, _, _ uuid.UUID, _ time.Duration) (int, error) {
			return 5, nil
		},
	}
	matches, err := r.Evaluate(context.Background(), sampleEvent())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches below max, got %d", len(matches))
	}
}

func TestVelocityRule_AtMax_Match(t *testing.T) {
	r := VelocityRule{
		MaxCount: 10,
		Window:   5 * time.Minute,
		LookupCount: func(_ context.Context, _, _ uuid.UUID, _ time.Duration) (int, error) {
			return 10, nil
		},
	}
	matches, err := r.Evaluate(context.Background(), sampleEvent())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match at max, got %d", len(matches))
	}
	if matches[0].Severity != SeverityCritical {
		t.Errorf("Severity = %q, want critical", matches[0].Severity)
	}
}

func TestVelocityRule_LookupError_Propagates(t *testing.T) {
	r := VelocityRule{
		MaxCount: 10,
		Window:   5 * time.Minute,
		LookupCount: func(_ context.Context, _, _ uuid.UUID, _ time.Duration) (int, error) {
			return 0, errors.New("db down")
		},
	}
	_, err := r.Evaluate(context.Background(), sampleEvent())
	if err == nil {
		t.Errorf("expected error from lookup to propagate")
	}
}

func TestVelocityRule_NilLookup_Error(t *testing.T) {
	r := VelocityRule{MaxCount: 10, Window: time.Minute}
	_, err := r.Evaluate(context.Background(), sampleEvent())
	if err == nil {
		t.Errorf("expected error for nil LookupCount")
	}
}

// =============================================================================
// FirstTimeRecipientRule
// =============================================================================

func TestFirstTimeRecipientRule_NoHistory_Match(t *testing.T) {
	r := FirstTimeRecipientRule{
		HasHistory: func(_ context.Context, _, _, _ uuid.UUID) (bool, error) {
			return false, nil
		},
	}
	matches, err := r.Evaluate(context.Background(), sampleEvent())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for new recipient, got %d", len(matches))
	}
	if matches[0].Severity != SeverityInfo {
		t.Errorf("Severity = %q, want info", matches[0].Severity)
	}
}

func TestFirstTimeRecipientRule_HasHistory_NoMatch(t *testing.T) {
	r := FirstTimeRecipientRule{
		HasHistory: func(_ context.Context, _, _, _ uuid.UUID) (bool, error) {
			return true, nil
		},
	}
	matches, err := r.Evaluate(context.Background(), sampleEvent())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for repeat recipient, got %d", len(matches))
	}
}

func TestFirstTimeRecipientRule_NilHasHistory_Error(t *testing.T) {
	r := FirstTimeRecipientRule{}
	_, err := r.Evaluate(context.Background(), sampleEvent())
	if err == nil {
		t.Errorf("expected error for nil HasHistory")
	}
}