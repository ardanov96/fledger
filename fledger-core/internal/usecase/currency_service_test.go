//go:build !windows
// +build !windows

// Package usecase — currency_service_test.go
//
// 8 test cases for CurrencyService (Sprint 12):
//  1. Same-currency Convert (identity, no rate lookup)
//  2. USD→IDR Convert correct (USD 100 × 15750 = IDR 1,575,000)
//  3. IDR→USD Convert correct (round-trip check)
//  4. Convert with missing FX rate → ErrFxRateNotFound
//  5. Convert with expired FX rate (expires_at <= now) → ErrFxRateNotFound
//  6. JPY→IDR with decimal-place shift (JPY 0dp → IDR 2dp)
//  7. CreateCurrency validation: empty code → ErrInvalidCurrencyCode
//  8. CreateFxRate validation: rate <= 0 → ErrInvalidFxRate
//
// All tests use an in-memory fake Repository + TxRunner (no DB needed).
package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// In-memory fakes
// =============================================================================

// memCurrencyRepo is an in-memory implementation of currency.Repository
// for unit tests.
type memCurrencyRepo struct {
	mu         sync.Mutex
	currencies map[string]currency.Currency
	rates      map[uuid.UUID]currency.FxRate
}

func newMemCurrencyRepo() *memCurrencyRepo {
	return &memCurrencyRepo{
		currencies: map[string]currency.Currency{},
		rates:      map[uuid.UUID]currency.FxRate{},
	}
}

// seed adds a currency without going through CreateCurrency (bypasses validation).
func (m *memCurrencyRepo) seed(c currency.Currency) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currencies[c.Code] = c
}

// seedRate adds an FX rate without going through CreateFxRate.
func (m *memCurrencyRepo) seedRate(r currency.FxRate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rates[r.ID] = r
}

func (m *memCurrencyRepo) CreateCurrency(ctx context.Context, tx currency.Tx, c currency.Currency) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.currencies[c.Code]; exists {
		return currency.ErrCurrencyAlreadyExists
	}
	c.CreatedAt = time.Now().UTC()
	m.currencies[c.Code] = c
	return nil
}

func (m *memCurrencyRepo) GetCurrency(ctx context.Context, code string) (currency.Currency, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.currencies[code]
	if !ok {
		return currency.Currency{}, currency.ErrCurrencyNotFound
	}
	return c, nil
}

func (m *memCurrencyRepo) ListCurrencies(ctx context.Context, onlyActive bool) ([]currency.Currency, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]currency.Currency, 0, len(m.currencies))
	for _, c := range m.currencies {
		if onlyActive && !c.IsActive {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func (m *memCurrencyRepo) UpdateCurrency(ctx context.Context, tx currency.Tx, code string, c currency.Currency) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.currencies[code]; !ok {
		return currency.ErrCurrencyNotFound
	}
	m.currencies[code] = c
	return nil
}

func (m *memCurrencyRepo) CreateFxRate(ctx context.Context, tx currency.Tx, r currency.FxRate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.CreatedAt = time.Now().UTC()
	m.rates[r.ID] = r
	return nil
}

func (m *memCurrencyRepo) GetFxRate(ctx context.Context, id uuid.UUID) (currency.FxRate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rates[id]
	if !ok {
		return currency.FxRate{}, currency.ErrFxRateNotFound
	}
	return r, nil
}

func (m *memCurrencyRepo) GetLatestFxRate(ctx context.Context, tenantID uuid.UUID, from, to string, at time.Time) (currency.FxRate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var best currency.FxRate
	var found bool
	for _, r := range m.rates {
		if r.TenantID != tenantID {
			continue
		}
		if r.FromCurrency != from || r.ToCurrency != to {
			continue
		}
		if r.EffectiveAt.After(at) {
			continue
		}
		if !r.ExpiresAt.After(at) {
			continue
		}
		if !found || r.EffectiveAt.After(best.EffectiveAt) {
			best = r
			found = true
		}
	}
	if !found {
		return currency.FxRate{}, currency.ErrFxRateNotFound
	}
	return best, nil
}

func (m *memCurrencyRepo) ListFxRates(ctx context.Context, tenantID uuid.UUID, from, to string, limit int) ([]currency.FxRate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]currency.FxRate, 0)
	for _, r := range m.rates {
		if r.TenantID != tenantID {
			continue
		}
		if from != "" && r.FromCurrency != from {
			continue
		}
		if to != "" && r.ToCurrency != to {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// memTxRunner is a no-op tx runner (just invokes fn).
type memTxRunner struct{}

func (memTxRunner) RunInTxCurrencyDomain(ctx context.Context, fn func(currency.Tx) error) error {
	return fn(memTx{})
}

// memTx is a no-op Tx for in-memory tests.
type memTx struct{}

func (memTx) Exec(ctx context.Context, sql string, args ...any) (currency.CommandTag, error) {
	return memTag{}, nil
}
func (memTx) Query(ctx context.Context, sql string, args ...any) (currency.Rows, error) {
	return nil, errors.New("not implemented")
}
func (memTx) QueryRow(ctx context.Context, sql string, args ...any) currency.Row {
	return nil
}

type memTag struct{}

func (memTag) RowsAffected() int64 { return 1 }

// =============================================================================
// Test helpers
// =============================================================================

func newTestService(t *testing.T) (*CurrencyService, *memCurrencyRepo) {
	repo := newMemCurrencyRepo()
	// Seed default currencies.
	repo.seed(currency.Currency{Code: "IDR", DecimalPlaces: 2, Name: "Indonesian Rupiah", IsActive: true})
	repo.seed(currency.Currency{Code: "USD", DecimalPlaces: 2, Name: "US Dollar", IsActive: true})
	repo.seed(currency.Currency{Code: "JPY", DecimalPlaces: 0, Name: "Japanese Yen", IsActive: true})

	svc := NewCurrencyService(repo, memTxRunner{})
	return svc, repo
}

func mustDecimal(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	require.NoError(t, err)
	return d
}

// =============================================================================
// Tests
// =============================================================================

func TestCurrencyService_SameCurrencyConvert(t *testing.T) {
	svc, _ := newTestService(t)
	tenantID := uuid.New()

	out, err := svc.Convert(context.Background(), ConvertInput{
		TenantID:     tenantID,
		FromCurrency: "IDR",
		ToCurrency:   "IDR",
		Amount:       money.NewFromMinor(100_000), // Rp 1,000
	})
	require.NoError(t, err)
	assert.Equal(t, money.NewFromMinor(100_000), out.ToAmount,
		"same-currency convert must be identity")
	assert.Empty(t, out.Rate.ID, "no rate should be looked up for same-currency")
}

func TestCurrencyService_ConvertUSDtoIDR(t *testing.T) {
	svc, repo := newTestService(t)
	tenantID := uuid.New()

	repo.seedRate(currency.FxRate{
		ID:           uuid.New(),
		TenantID:     tenantID,
		FromCurrency: "USD",
		ToCurrency:   "IDR",
		Rate:         mustDecimal(t, "15750"),
		EffectiveAt:  time.Now().Add(-1 * time.Hour),
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		Source:       currency.FxRateSourceManual,
		CreatedBy:    uuid.New(),
	})

	// USD 100.00 (10000 minor) × 15750 = IDR 1,575,000.00 (157,500,000 minor)
	out, err := svc.Convert(context.Background(), ConvertInput{
		TenantID:     tenantID,
		FromCurrency: "USD",
		ToCurrency:   "IDR",
		Amount:       money.NewFromMinor(10_000),
	})
	require.NoError(t, err)
	assert.Equal(t, money.NewFromMinor(157_500_000), out.ToAmount,
		"USD 100.00 × 15750 = IDR 1,575,000.00")
}

func TestCurrencyService_ConvertIDRtoUSD(t *testing.T) {
	svc, repo := newTestService(t)
	tenantID := uuid.New()

	// Inverse rate: 1 USD = 15750 IDR, so 1 IDR = 1/15750 USD.
	repo.seedRate(currency.FxRate{
		ID:           uuid.New(),
		TenantID:     tenantID,
		FromCurrency: "IDR",
		ToCurrency:   "USD",
		Rate:         mustDecimal(t, "0.00006349206"), // 1/15750 truncated
		EffectiveAt:  time.Now().Add(-1 * time.Hour),
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		Source:       currency.FxRateSourceManual,
		CreatedBy:    uuid.New(),
	})

	// IDR 1,575,000 (157,500,000 minor) × 0.00006349206 ≈ USD 99.99 (9999.x minor)
	out, err := svc.Convert(context.Background(), ConvertInput{
		TenantID:     tenantID,
		FromCurrency: "IDR",
		ToCurrency:   "USD",
		Amount:       money.NewFromMinor(157_500_000),
	})
	require.NoError(t, err)
	// Expected: 157500000 / 100 * 0.00006349206 = 1575000 * 0.00006349206
	//          = 99.99999... USD ≈ 99.99 USD = 9999 minor (after rounding)
	// Allow small tolerance due to truncation.
	assert.InDelta(t, int64(9999), int64(out.ToAmount), 2,
		"IDR 1,575,000 × 1/15750 ≈ USD 100 (rounded)")
}

func TestCurrencyService_ConvertMissingRate(t *testing.T) {
	svc, _ := newTestService(t)
	tenantID := uuid.New()

	_, err := svc.Convert(context.Background(), ConvertInput{
		TenantID:     tenantID,
		FromCurrency: "USD",
		ToCurrency:   "IDR",
		Amount:       money.NewFromMinor(10_000),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, currency.ErrFxRateNotFound)
}

func TestCurrencyService_ConvertExpiredRate(t *testing.T) {
	svc, repo := newTestService(t)
	tenantID := uuid.New()

	repo.seedRate(currency.FxRate{
		ID:           uuid.New(),
		TenantID:     tenantID,
		FromCurrency: "USD",
		ToCurrency:   "IDR",
		Rate:         mustDecimal(t, "15750"),
		EffectiveAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt:    time.Now().Add(-1 * time.Hour), // expired
		Source:       currency.FxRateSourceManual,
		CreatedBy:    uuid.New(),
	})

	_, err := svc.Convert(context.Background(), ConvertInput{
		TenantID:     tenantID,
		FromCurrency: "USD",
		ToCurrency:   "IDR",
		Amount:       money.NewFromMinor(10_000),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, currency.ErrFxRateNotFound,
		"expired rate must yield ErrFxRateNotFound")
}

func TestCurrencyService_ConvertJPYtoIDR_DecimalShift(t *testing.T) {
	svc, repo := newTestService(t)
	tenantID := uuid.New()

	repo.seedRate(currency.FxRate{
		ID:           uuid.New(),
		TenantID:     tenantID,
		FromCurrency: "JPY",
		ToCurrency:   "IDR",
		Rate:         mustDecimal(t, "105"), // 1 JPY = 105 IDR (rate is per major unit)
		EffectiveAt:  time.Now().Add(-1 * time.Hour),
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		Source:       currency.FxRateSourceManual,
		CreatedBy:    uuid.New(),
	})

	// JPY 1000 (1000 minor, since JPY=0dp) × 105 = IDR 105,000 = 10,500,000 minor
	out, err := svc.Convert(context.Background(), ConvertInput{
		TenantID:     tenantID,
		FromCurrency: "JPY",
		ToCurrency:   "IDR",
		Amount:       money.NewFromMinor(1_000),
	})
	require.NoError(t, err)
	assert.Equal(t, money.NewFromMinor(10_500_000), out.ToAmount,
		"JPY 1000 × 105 = IDR 105,000.00 (decimal-place shift 0dp -> 2dp)")
}

func TestCurrencyService_CreateCurrency_EmptyCode(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.CreateCurrency(context.Background(), CreateCurrencyInput{
		Code:          "",
		DecimalPlaces: 2,
		Name:          "Test",
		IsActive:      true,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, currency.ErrInvalidCurrencyCode)
}

func TestCurrencyService_CreateFxRate_NegativeRate(t *testing.T) {
	svc, _ := newTestService(t)
	tenantID := uuid.New()

	_, err := svc.CreateFxRate(context.Background(), CreateFxRateInput{
		TenantID:     tenantID,
		FromCurrency: "USD",
		ToCurrency:   "IDR",
		Rate:         mustDecimal(t, "-100"),
		EffectiveAt:  time.Now(),
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		Source:       currency.FxRateSourceManual,
		CreatedBy:    uuid.New(),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, currency.ErrInvalidFxRate)
}
