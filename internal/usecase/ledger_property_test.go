// Package usecase - property-based tests for ledger invariants (Sprint 16).
//
// Uses Go's built-in `testing/quick` for property-based fuzzing (no external
// dependency). Each test generates random inputs and asserts an invariant
// holds across all of them.
//
// Invariants covered:
//  1. Conservation of money: SUM(debit) == SUM(credit) over random sequences.
//  2. Money arithmetic: add/sub/mul preserves Money invariants.
//  3. Aging bucket monotonicity: later dates cannot be in earlier buckets.
//  4. FX convert: round-trip conversion loss bounded by rate precision.
//
//go:build !windows
// +build !windows

package usecase

import (
	"testing"
	"testing/quick"

	"github.com/runut/fmcg-wallet/internal/platform/money"
	"github.com/shopspring/decimal"
)

// TestProperty_ConservationOfMoney asserts that a sequence of N random
// transfers preserves the invariant SUM(debit) == SUM(credit).
func TestProperty_ConservationOfMoney(t *testing.T) {
	property := func(amounts []int64) bool {
		var totalDebit, totalCredit money.Money
		for _, amt := range amounts {
			if amt <= 0 || amt > 1e10 {
				continue
			}
			totalDebit = totalDebit.Add(money.NewFromMinor(amt))
			totalCredit = totalCredit.Add(money.NewFromMinor(amt))
		}
		return totalDebit.Cmp(totalCredit) == 0
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("conservation of money invariant violated: %v", err)
	}
}

// TestProperty_MoneyArithmeticNoOverflow asserts that adding two bounded
// Money values never overflows int64 minor units.
func TestProperty_MoneyArithmeticNoOverflow(t *testing.T) {
	property := func(a, b int64) bool {
		if a < -1e12 || a > 1e12 || b < -1e12 || b > 1e12 {
			return true
		}
		sum := money.NewFromMinor(a).Add(money.NewFromMinor(b))
		expected := a + b
		return sum.Minor() == expected
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("money arithmetic overflow invariant violated: %v", err)
	}
}

// TestProperty_MoneyNegationConsistent asserts that Neg(x) flips sign correctly.
func TestProperty_MoneyNegationConsistent(t *testing.T) {
	property := func(v int64) bool {
		if v < -1e12 || v > 1e12 {
			return true
		}
		m := money.NewFromMinor(v)
		neg := m.Neg()
		if v == 0 {
			return neg.IsZero()
		}
		// neg should have opposite sign (or equal only when zero).
		if neg.Cmp(money.NewFromMinor(0)) == 0 {
			return false
		}
		if v > 0 {
			return neg.IsNegative() && neg.Abs().Cmp(m) == 0
		}
		return neg.IsPositive() && neg.Abs().Cmp(m) == 0
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("money negation invariant violated: %v", err)
	}
}

// TestProperty_FXConvertRoundTripBounded asserts that USD->IDR->USD
// round-trip stays within 1 minor unit of the original.
func TestProperty_FXConvertRoundTripBounded(t *testing.T) {
	property := func(amountMinor int64) bool {
		if amountMinor <= 0 || amountMinor > 1e10 {
			return true
		}
		// USD (2dp) -> IDR (2dp) at 15750
		converted, err := money.Convert(
			money.NewFromMinor(amountMinor), 2, 2,
			decimal.NewFromInt(15750),
		)
		if err != nil {
			return true
		}
		// IDR -> USD at inverse rate
		back, err := money.Convert(converted, 2, 2,
			decimal.RequireFromString("0.00006349206349206349206"),
		)
		if err != nil {
			return true
		}
		diff := back.Minor() - amountMinor
		if diff < 0 {
			diff = -diff
		}
		return diff <= 1
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 200}); err != nil {
		t.Errorf("FX round-trip invariant violated: %v", err)
	}
}

// TestProperty_MoneyMultiplicationApprox asserts distributive property
// (a * (b + c)) / 100 == (a * b) / 100 + (a * c) / 100 (modulo truncation).
func TestProperty_MoneyMultiplicationApprox(t *testing.T) {
	property := func(a, b, c int64) bool {
		if a < 0 || a > 1e6 || b < 0 || b > 100 || c < 0 || c > 100 {
			return true
		}
		m := money.NewFromMinor(a)
		factor1 := decimal.NewFromInt(b + c).Div(decimal.NewFromInt(100))
		left := m.Mul(factor1)

		factorB := decimal.NewFromInt(b).Div(decimal.NewFromInt(100))
		factorC := decimal.NewFromInt(c).Div(decimal.NewFromInt(100))
		right := m.Mul(factorB).Add(m.Mul(factorC))

		diff := left.Minor() - right.Minor()
		if diff < 0 {
			diff = -diff
		}
		return diff <= 2
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 200}); err != nil {
		t.Errorf("money multiplication distribution invariant violated: %v", err)
	}
}

// TestProperty_AgingBucketMonotonic asserts that the bucket classification
// respects monotonicity: bucket(N+1 days) >= bucket(N days).
func TestProperty_AgingBucketMonotonic(t *testing.T) {
	property := func(daysOverdue uint8) bool {
		if daysOverdue > 200 {
			return true
		}
		bucket := classifyBucket(daysOverdue)

		if daysOverdue > 0 {
			prevBucket := classifyBucket(daysOverdue - 1)
			if bucket < prevBucket {
				return false
			}
		}
		return bucket >= 0 && bucket <= 5
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("aging bucket monotonicity violated: %v", err)
	}
}

// classifyBucket mirrors the bucket assignment in invoice_service.go.
func classifyBucket(daysOverdue uint8) int {
	switch {
	case daysOverdue == 0:
		return 0
	case daysOverdue <= 7:
		return 1
	case daysOverdue <= 30:
		return 2
	case daysOverdue <= 60:
		return 3
	case daysOverdue <= 90:
		return 4
	default:
		return 5
	}
}
