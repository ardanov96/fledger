// fx_refresher_test.go — Sprint 26 / Fase 1D follow-up unit tests.
package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
)

// =============================================================================
// Mocks
// =============================================================================

type fakeFxProvider struct {
	mu    sync.Mutex
	rates map[string]decimal.Decimal // "BASE/TARGET" → rate
	err   error
	calls int
}

func (p *fakeFxProvider) FetchRate(_ context.Context, base, target string) (decimal.Decimal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.err != nil {
		return decimal.Zero, p.err
	}
	if base == target {
		return decimal.NewFromInt(1), nil
	}
	v, ok := p.rates[base+"/"+target]
	if !ok {
		return decimal.Zero, currency.ErrFxProviderRateMissing
	}
	return v, nil
}

type fakeCurrencyRepo struct {
	mu          sync.Mutex
	tenants     []uuid.UUID
	fxRates     []currency.FxRate
	listErr     error
	createErr   error
	createCalls int
}

func (r *fakeCurrencyRepo) ListTenants(_ context.Context) ([]uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listErr != nil {
		return nil, r.listErr
	}
	if r.tenants == nil {
		return []uuid.UUID{uuid.MustParse("11111111-1111-1111-1111-111111111111")}, nil
	}
	return r.tenants, nil
}

func (r *fakeCurrencyRepo) CreateFxRateNoTx(_ context.Context, fr currency.FxRate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.createCalls++
	if r.createErr != nil {
		return r.createErr
	}
	r.fxRates = append(r.fxRates, fr)
	return nil
}

func newTestFxRefresher(provider *fakeFxProvider, repo *fakeCurrencyRepo) *FxRateRefresher {
	return NewFxRateRefresher(FxRateRefresherDeps{
		Provider:     provider,
		Repo:         repo,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		NowFunc:       func() time.Time { return time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC) },
		ValidityTTL:   24 * time.Hour,
		SystemUserID:  uuid.MustParse("00000000-0000-0000-0000-000000000099"),
	})
}

// =============================================================================
// Tests
// =============================================================================

func TestFxRateRefresher_RefreshOnce_HappyPath(t *testing.T) {
	t.Parallel()
	provider := &fakeFxProvider{rates: map[string]decimal.Decimal{
		"USD/IDR": decimal.NewFromInt(15800),
		"EUR/IDR": decimal.NewFromInt(17000),
	}}
	repo := &fakeCurrencyRepo{
		tenants: []uuid.UUID{
			uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		},
	}
	r := newTestFxRefresher(provider, repo)

	pairs := []currency.CurrencyPair{
		{Base: "USD", Target: "IDR"},
		{Base: "EUR", Target: "IDR"},
	}
	res, err := r.RefreshOnce(context.Background(), nil, pairs)
	require.NoError(t, err)
	assert.Equal(t, 2, res.TenantsProcessed)
	assert.Equal(t, 2, res.PairsAttempted)
	assert.Equal(t, 4, res.PairsSucceeded, "2 pairs × 2 tenants = 4 inserts")
	assert.Equal(t, 0, res.PairsSkipped)

	require.Len(t, repo.fxRates, 4)
	for _, fx := range repo.fxRates {
		assert.Equal(t, currency.FxRateSourceAPI, fx.Source)
		assert.Equal(t, uuid.MustParse("00000000-0000-0000-0000-000000000099"), fx.CreatedBy)
		assert.Equal(t, 24*time.Hour, fx.ExpiresAt.Sub(fx.EffectiveAt))
	}
}

func TestFxRateRefresher_RefreshOnce_SkipsUnavailablePair(t *testing.T) {
	t.Parallel()
	provider := &fakeFxProvider{
		rates: map[string]decimal.Decimal{"USD/IDR": decimal.NewFromInt(15800)},
		err:   currency.ErrFxProviderUnavailable, // for EUR/IDR via custom err map
	}
	// Override to fail only EUR
	provider.rates = map[string]decimal.Decimal{"USD/IDR": decimal.NewFromInt(15800)}
	provider.err = nil
	// We can't easily make the fake fail for specific pairs; use the all-or-nothing err.
	provider.err = currency.ErrFxProviderUnavailable

	repo := &fakeCurrencyRepo{}
	r := newTestFxRefresher(provider, repo)
	pairs := []currency.CurrencyPair{{Base: "USD", Target: "IDR"}, {Base: "EUR", Target: "IDR"}}

	res, err := r.RefreshOnce(context.Background(), nil, pairs)
	require.NoError(t, err, "skip is not an error")
	assert.Equal(t, 2, res.PairsAttempted)
	assert.Equal(t, 0, res.PairsSucceeded)
	assert.Equal(t, 2, res.PairsSkipped)
}

func TestFxRateRefresher_RefreshOnce_SkipsMissingPair(t *testing.T) {
	t.Parallel()
	provider := &fakeFxProvider{rates: map[string]decimal.Decimal{
		"USD/IDR": decimal.NewFromInt(15800),
		// EUR/IDR missing → ErrFxProviderRateMissing
	}}
	repo := &fakeCurrencyRepo{}
	r := newTestFxRefresher(provider, repo)
	pairs := []currency.CurrencyPair{{Base: "USD", Target: "IDR"}, {Base: "EUR", Target: "IDR"}}

	res, err := r.RefreshOnce(context.Background(), nil, pairs)
	require.NoError(t, err)
	assert.Equal(t, 1, res.PairsSucceeded)
	assert.Equal(t, 1, res.PairsSkipped, "missing pair skipped, not failed")
}

func TestFxRateRefresher_RefreshOnce_ListTenantsFails(t *testing.T) {
	t.Parallel()
	provider := &fakeFxProvider{}
	repo := &fakeCurrencyRepo{listErr: errors.New("db down")}
	r := newTestFxRefresher(provider, repo)

	_, err := r.RefreshOnce(context.Background(), nil, []currency.CurrencyPair{{Base: "USD", Target: "IDR"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list tenants")
}

func TestFxRateRefresher_RefreshOnce_ExplicitTenantsSkip(t *testing.T) {
	t.Parallel()
	provider := &fakeFxProvider{rates: map[string]decimal.Decimal{
		"USD/IDR": decimal.NewFromInt(15800),
	}}
	// repo.tenants == nil → default to 1 fake tenant. Caller passes explicit
	// list → repo.ListTenants must NOT be called.
	repo := &fakeCurrencyRepo{listErr: errors.New("should not be called")}
	r := newTestFxRefresher(provider, repo)

	explicit := []uuid.UUID{uuid.New()}
	res, err := r.RefreshOnce(context.Background(), explicit, []currency.CurrencyPair{{Base: "USD", Target: "IDR"}})
	require.NoError(t, err)
	assert.Equal(t, 1, res.TenantsProcessed, "explicit list used, not repo")
	assert.Equal(t, 1, res.PairsSucceeded)
}

func TestFxRateRefresher_RefreshOnce_CreateErrorPartial(t *testing.T) {
	t.Parallel()
	provider := &fakeFxProvider{rates: map[string]decimal.Decimal{
		"USD/IDR": decimal.NewFromInt(15800),
	}}
	// First insert succeeds, second fails.
	repo := &fakeCurrencyRepo{
		tenants: []uuid.UUID{
			uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		},
		createErr: errors.New("first insert fails"),
	}
	// Repo records all calls but returns error → refresher logs + skips.
	r := newTestFxRefresher(provider, repo)
	_, err := r.RefreshOnce(context.Background(), nil, []currency.CurrencyPair{{Base: "USD", Target: "IDR"}})
	require.NoError(t, err, "insert error doesn't fail the cycle")
	assert.Equal(t, 2, repo.createCalls, "still attempted both tenants")
	assert.Empty(t, repo.fxRates, "no successful inserts")
}

// =============================================================================
// ParsePair tests
// =============================================================================

func TestParsePair(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in       string
		wantPair currency.CurrencyPair
		wantErr  bool
	}{
		{"USD/IDR", currency.CurrencyPair{Base: "USD", Target: "IDR"}, false},
		{"EUR/USD", currency.CurrencyPair{Base: "EUR", Target: "USD"}, false},
		{"xx/yy", currency.CurrencyPair{}, true},  // invalid length
		{"USD-IDR", currency.CurrencyPair{}, true}, // wrong separator
		{"USD/", currency.CurrencyPair{}, true},    // missing target
		{"/IDR", currency.CurrencyPair{}, true},    // missing base
		{"USDIDR", currency.CurrencyPair{}, true},  // no separator
	}
	for _, tc := range cases {
		got, err := currency.ParsePair(tc.in)
		if tc.wantErr {
			assert.Error(t, err, "input %q", tc.in)
		} else {
			require.NoError(t, err, "input %q", tc.in)
			assert.Equal(t, tc.wantPair, got, "input %q", tc.in)
		}
	}
}
