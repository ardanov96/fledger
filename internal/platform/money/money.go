// Package money provides a money type for the FMCG wallet.
//
// All money is stored as int64 minor units (e.g. sen for IDR, cents for USD)
// to avoid floating-point precision errors. Intermediate calculations use
// shopspring/decimal to preserve precision for division and multiplication.
//
// IMPORTANT: This package is the ONLY place where money arithmetic happens.
// Other layers MUST go through these methods, never raw int64 arithmetic.
//
// Why int64 minor units?
//   - Range: int64 can represent up to ~9.2 * 10^18, which is plenty even for
//     the largest conceivable balances when denominated in minor units.
//   - Precision: zero floating-point error.
//   - DB-friendly: 8-byte fixed size, easy to index and aggregate.
//   - Display formatting is a presentation concern (use String() for IDR).
//
// Why not just float64?
//   - 0.1 + 0.2 != 0.3 in IEEE 754. In financial systems, this is unacceptable.
//     See ADR-0010-money-representation.md
package money

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// Money represents an amount of currency in minor units (e.g. sen for IDR).
//
// The zero value is valid and represents zero amount.
type Money int64

// Sentinel errors for money package.
var (
	ErrInvalidAmount   = errors.New("money: invalid amount")
	ErrCurrencyMismatch = errors.New("money: currency mismatch")
	ErrDivisionByZero  = errors.New("money: division by zero")
)

// Common currency definitions.
// Indonesian Rupiah has no fractional unit in circulation, but we still use
// 2 minor units (sen) to keep the model consistent with multi-currency future.
//
// 1 IDR = 100 sen (conceptually, even if not circulating)
const (
	IDR = "IDR"
	USD = "USD"
)

// MinorUnitsPerMajor returns how many minor units equal one major unit.
func MinorUnitsPerMajor(currency string) int32 {
	switch strings.ToUpper(currency) {
	case IDR, USD, "EUR", "GBP", "JPY":
		// For consistency in ledger, we treat all currencies as 2 decimal places.
		// JPY technically has 0, but ledger-wise this is OK for MVP.
		// TODO: Move to currencies table for full multi-currency.
		return 100
	default:
		return 100
	}
}

// NewFromMajor creates a Money from a major-unit value (e.g. rupiah) using
// the currency's standard decimal places.
//
// Example: NewFromMajor(10000, "IDR") -> 1,000,000 (sen)
func NewFromMajor(major int64, currency string) Money {
	multiplier := decimal.NewFromInt32(MinorUnitsPerMajor(currency))
	amt := decimal.NewFromInt(major).Mul(multiplier)
	return Money(amt.IntPart())
}

// NewFromMinor creates a Money directly from minor units (e.g. sen).
//
// Example: NewFromMinor(1000000) -> 1,000,000 sen = Rp 10,000
func NewFromMinor(minor int64) Money {
	return Money(minor)
}

// NewFromDecimal creates a Money from a decimal major-unit value.
//
// Example: NewFromDecimal("1234.56", "IDR") -> 123,456 sen = Rp 1,234.56
func NewFromDecimal(major string, currency string) (Money, error) {
	d, err := decimal.NewFromString(major)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidAmount, err)
	}
	multiplier := decimal.NewFromInt32(MinorUnitsPerMajor(currency))
	result := d.Mul(multiplier)
	if !result.IsInteger() {
		return 0, fmt.Errorf("%w: amount has more decimal places than currency supports", ErrInvalidAmount)
	}
	return Money(result.IntPart()), nil
}

// IsZero returns true if the amount is exactly zero.
func (m Money) IsZero() bool {
	return m == 0
}

// IsPositive returns true if the amount is greater than zero.
func (m Money) IsPositive() bool {
	return m > 0
}

// IsNegative returns true if the amount is less than zero.
func (m Money) IsNegative() bool {
	return m < 0
}

// Minor returns the amount in minor units.
func (m Money) Minor() int64 {
	return int64(m)
}

// Major returns the amount in major units (e.g. rupiah for IDR).
// Truncates fractional minor units.
func (m Money) Major(currency string) int64 {
	return int64(m) / int64(MinorUnitsPerMajor(currency))
}

// Add returns a new Money that is the sum of m and other.
// Returns error if currencies are not equivalent (currently always OK since
// Money is currency-agnostic; left for future multi-currency).
func (m Money) Add(other Money) Money {
	return m + other
}

// Sub returns a new Money that is the difference of m and other.
func (m Money) Sub(other Money) Money {
	return m - other
}

// Neg returns the negation of m.
func (m Money) Neg() Money {
	return -m
}

// Mul returns a new Money multiplied by a decimal factor.
//
// Using decimal.Decimal here to avoid float precision loss in intermediate
// calculations. Useful for percentage discounts, tax rates, etc.
//
// Example: Rp 10,000 * 11% PPN -> Rp 1,100 (110,000 sen)
func (m Money) Mul(factor decimal.Decimal) Money {
	if m == 0 {
		return 0
	}
	// Multiply in higher precision (minor units * factor with 4 decimal places)
	result := decimal.NewFromInt(int64(m)).Mul(factor)
	// Round half-up to nearest minor unit (banker's rounding for money is NOT used;
	// in finance, half-up is the convention to avoid systematic downward bias).
	result = result.Round(0)
	return Money(result.IntPart())
}

// Div returns a new Money divided by an integer divisor.
//
// Integer division only — uses floor (truncation toward zero for positive,
// away from zero for negative). For exact division, see DivExact.
//
// Example: Rp 1,000 / 3 -> Rp 333 (33,300 sen), remainder Rp 1
func (m Money) Div(divisor int64) (Money, error) {
	if divisor == 0 {
		return 0, ErrDivisionByZero
	}
	return Money(int64(m) / divisor), nil
}

// DivExact returns quotient and remainder for exact division.
// Useful when you need to split a payment across N invoices.
func (m Money) DivExact(divisor int64) (quotient Money, remainder Money, err error) {
	if divisor == 0 {
		return 0, 0, ErrDivisionByZero
	}
	q := int64(m) / divisor
	r := int64(m) % divisor
	return Money(q), Money(r), nil
}

// Cmp compares two Money values.
// Returns -1 if m < other, 0 if m == other, 1 if m > other.
func (m Money) Cmp(other Money) int {
	switch {
	case m < other:
		return -1
	case m > other:
		return 1
	default:
		return 0
	}
}

// Abs returns the absolute value of m.
func (m Money) Abs() Money {
	if m < 0 {
		return -m
	}
	return m
}

// String returns a human-readable IDR-formatted string.
// For other currencies, use Format().
func (m Money) String() string {
	return m.Format(IDR)
}

// Format returns a human-readable string for the given currency.
// Format: "Rp 1.234.567,89" for IDR, "1,234,567.89" for others.
func (m Money) Format(currency string) string {
	major := int64(m) / int64(MinorUnitsPerMajor(currency))
	minor := int64(m) % int64(MinorUnitsPerMajor(currency))
	// Ensure minor is non-negative for display (handles negative amounts)
	if minor < 0 {
		minor = -minor
	}

	if strings.ToUpper(currency) == IDR {
		// Indonesian formatting: dot thousands, comma decimal
		return fmt.Sprintf("Rp %s,%02d", formatIDR(major), minor)
	}

	// International formatting: comma thousands, dot decimal
	return fmt.Sprintf("%s.%02d", formatIntComma(major), minor)
}

// formatIDR formats an integer with dot as thousands separator.
// Example: 1234567 -> "1.234.567"
func formatIDR(n int64) string {
	negative := n < 0
	if negative {
		n = -n
	}
	digits := fmt.Sprintf("%d", n)

	// Insert dots every 3 digits from the right
	var b strings.Builder
	length := len(digits)
	for i, ch := range digits {
		if i > 0 && (length-i)%3 == 0 {
			b.WriteRune('.')
		}
		b.WriteRune(ch)
	}
	if negative {
		return "-" + b.String()
	}
	return b.String()
}

// formatIntComma formats with comma thousands separator.
func formatIntComma(n int64) string {
	negative := n < 0
	if negative {
		n = -n
	}
	digits := fmt.Sprintf("%d", n)
	var b strings.Builder
	length := len(digits)
	for i, ch := range digits {
		if i > 0 && (length-i)%3 == 0 {
			b.WriteRune(',')
		}
		b.WriteRune(ch)
	}
	if negative {
		return "-" + b.String()
	}
	return b.String()
}

// Sum returns the sum of a slice of Money.
// Convenience for aggregating entries in a transaction.
func Sum(amounts ...Money) Money {
	var total int64
	for _, a := range amounts {
		total += int64(a)
	}
	return Money(total)
}
