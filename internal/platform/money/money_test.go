package money_test

import (
	"testing"

	"github.com/runut/fmcg-wallet/internal/platform/money"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromMajor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		major    int64
		currency string
		want     money.Money
	}{
		{"IDR 10k", 10000, money.IDR, 1_000_000},
		{"IDR 1", 1, money.IDR, 100},
		{"IDR 0", 0, money.IDR, 0},
		{"USD 100", 100, money.USD, 10_000},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := money.NewFromMajor(tt.major, tt.currency)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewFromDecimal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		decimal  string
		currency string
		want     money.Money
		wantErr  bool
	}{
		{"simple IDR", "1000.50", money.IDR, 100_050, false},
		{"integer IDR", "1000", money.IDR, 100_000, false},
		{"too many decimals", "1000.001", money.IDR, 0, true},
		{"invalid string", "abc", money.IDR, 0, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := money.NewFromDecimal(tt.decimal, tt.currency)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMoney_Add(t *testing.T) {
	t.Parallel()
	a := money.NewFromMinor(100)
	b := money.NewFromMinor(250)
	assert.Equal(t, money.NewFromMinor(350), a.Add(b))
}

func TestMoney_Sub(t *testing.T) {
	t.Parallel()
	a := money.NewFromMinor(500)
	b := money.NewFromMinor(200)
	assert.Equal(t, money.NewFromMinor(300), a.Sub(b))
}

func TestMoney_Neg(t *testing.T) {
	t.Parallel()
	assert.Equal(t, money.NewFromMinor(-100), money.NewFromMinor(100).Neg())
	assert.Equal(t, money.NewFromMinor(100), money.NewFromMinor(-100).Neg())
}

func TestMoney_Mul(t *testing.T) {
	t.Parallel()
	// Rp 10,000 (1,000,000 sen) * 11% PPN = Rp 1,100 (110,000 sen)
	ppn := decimal.NewFromFloat(0.11)
	base := money.NewFromMajor(10_000, money.IDR)
	expected := money.NewFromMajor(1_100, money.IDR)
	got := base.Mul(ppn)
	assert.Equal(t, expected, got)
}

func TestMoney_Mul_RoundHalfUp(t *testing.T) {
	t.Parallel()
	// 101 sen * 0.5 = 50.5 -> rounds to 51
	half := decimal.NewFromFloat(0.5)
	base := money.NewFromMinor(101) // odd number
	got := base.Mul(half)
	assert.Equal(t, money.NewFromMinor(51), got)
}

func TestMoney_Div(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		m        money.Money
		divisor  int64
		want     money.Money
		wantErr  bool
	}{
		{"even", money.NewFromMinor(100), 4, money.NewFromMinor(25), false},
		{"truncate", money.NewFromMinor(100), 3, money.NewFromMinor(33), false},
		{"zero divisor", money.NewFromMinor(100), 0, 0, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.m.Div(tt.divisor)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Minor unit is 1/100 of major unit (1 IDR = 100 sen).
// 100,000 sen (= Rp 1,000) / 3 = 33,333 sen quotient, 1 sen remainder.
// In major units: Rp 333,33 quotient, Rp 0,01 remainder.
func TestMoney_DivExact(t *testing.T) {
	t.Parallel()
	const oneThousandSen int64 = 100_000
	q, r, err := money.NewFromMinor(oneThousandSen).DivExact(3)
	require.NoError(t, err)
	assert.Equal(t, money.NewFromMinor(33_333), q)
	assert.Equal(t, money.NewFromMinor(1), r)
}

func TestMoney_Cmp(t *testing.T) {
	t.Parallel()
	a := money.NewFromMinor(100)
	b := money.NewFromMinor(200)
	c := money.NewFromMinor(100)
	assert.Equal(t, -1, a.Cmp(b))
	assert.Equal(t, 1, b.Cmp(a))
	assert.Equal(t, 0, a.Cmp(c))
}

func TestMoney_Abs(t *testing.T) {
	t.Parallel()
	assert.Equal(t, money.NewFromMinor(100), money.NewFromMinor(100).Abs())
	assert.Equal(t, money.NewFromMinor(100), money.NewFromMinor(-100).Abs())
	assert.Equal(t, money.NewFromMinor(0), money.NewFromMinor(0).Abs())
}

func TestMoney_IsZero_Positive_Negative(t *testing.T) {
	t.Parallel()
	assert.True(t, money.NewFromMinor(0).IsZero())
	assert.False(t, money.NewFromMinor(1).IsZero())
	assert.True(t, money.NewFromMinor(100).IsPositive())
	assert.False(t, money.NewFromMinor(0).IsPositive())
	assert.True(t, money.NewFromMinor(-100).IsNegative())
}

// Format always shows 2 decimal places for consistency in financial reports.
// E.g. Rp 1.000,00 instead of Rp 1.000.
func TestMoney_Format_IDR(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		m    money.Money
		want string
	}{
		{"zero", money.NewFromMinor(0), "Rp 0,00"},
		{"small", money.NewFromMinor(100), "Rp 1,00"},
		{"thousand", money.NewFromMinor(100_000), "Rp 1.000,00"},
		{"million", money.NewFromMinor(100_000_000), "Rp 1.000.000,00"},
		{"complex", money.NewFromMinor(123_456_789), "Rp 1.234.567,89"},
		{"negative", money.NewFromMinor(-50_000), "Rp -500,00"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.m.Format(money.IDR))
		})
	}
}

func TestMoney_Format_USD(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		m    money.Money
		want string
	}{
		{"zero", money.NewFromMinor(0), "0.00"},
		{"thousand", money.NewFromMinor(100_000), "1,000.00"},
		{"complex", money.NewFromMinor(123_456_789), "1,234,567.89"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.m.Format(money.USD))
		})
	}
}

func TestMoney_Sum(t *testing.T) {
	t.Parallel()
	sum := money.Sum(
		money.NewFromMinor(100),
		money.NewFromMinor(200),
		money.NewFromMinor(300),
	)
	assert.Equal(t, money.NewFromMinor(600), sum)
}

func TestSum(t *testing.T) {
	t.Parallel()
	// Property: Sum() must equal left-fold
	a := money.NewFromMinor(123)
	b := money.NewFromMinor(456)
	c := money.NewFromMinor(789)
	sum := money.Sum(a, b, c)
	assert.Equal(t, a.Add(b).Add(c), sum)
}

// Property-based invariant: Add is commutative
func TestProperty_AddCommutative(t *testing.T) {
	t.Parallel()
	for _, amt := range []int64{0, 1, 100, 1_000_000, -500, 999_999_999} {
		amt := amt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			a := money.NewFromMinor(amt)
			b := money.NewFromMinor(amt * 3)
			assert.Equal(t, a.Add(b), b.Add(a), "add must be commutative for amt=%d", amt)
		})
	}
}

// Property-based invariant: (a + b) - b = a
func TestProperty_SubInverseOfAdd(t *testing.T) {
	t.Parallel()
	for _, amt := range []int64{0, 100, 1_000, 1_000_000} {
		amt := amt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			a := money.NewFromMinor(amt)
			b := money.NewFromMinor(amt / 2)
			assert.Equal(t, a, a.Add(b).Sub(b), "sub must invert add")
		})
	}
}
