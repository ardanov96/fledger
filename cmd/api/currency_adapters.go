// currency_adapters.go — adapters to wire CurrencyService → handler.CurrencyAPI
// + the FxRateLookup bridge that TransferService consumes.
//
// Three pieces:
//  1. currencyTxAdapter — wraps DB.RunInTxCurrencyDomain → usecase.CurrencyTxRunner
//  2. currencyAPIAdapter — adapts usecase.CurrencyService → handler.CurrencyAPI
//  3. fxRateLookupAdapter — exposes GetLatestFxRate + GetCurrency for
//     TransferService (avoids importing currency package into usecase/transfer)
package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// Compile-time guards.
var (
	_ usecase.CurrencyTxRunner = (*currencyTxAdapter)(nil)
)

// =============================================================================
// currencyTxAdapter
// =============================================================================

type currencyTxAdapter struct {
	db *postgres.DB
}

func (a *currencyTxAdapter) RunInTxCurrencyDomain(ctx context.Context, fn func(currency.Tx) error) error {
	return a.db.RunInTxCurrencyDomain(ctx, fn)
}

// =============================================================================
// currencyAPIAdapter — handler.CurrencyAPI adapter
// =============================================================================

// currencyAPIAdapter is a thin adapter over usecase.CurrencyService.
// Handler-side signatures already use usecase types directly (no translation
// required) — the adapter simply holds the service pointer.
type currencyAPIAdapter struct {
	svc *usecase.CurrencyService
}

func (a *currencyAPIAdapter) ListActiveCurrencies(ctx context.Context) ([]currency.Currency, error) {
	return a.svc.ListActiveCurrencies(ctx)
}

func (a *currencyAPIAdapter) GetCurrency(ctx context.Context, code string) (currency.Currency, error) {
	return a.svc.GetCurrency(ctx, code)
}

func (a *currencyAPIAdapter) CreateCurrency(ctx context.Context, in usecase.CreateCurrencyInput) (currency.Currency, error) {
	return a.svc.CreateCurrency(ctx, in)
}

func (a *currencyAPIAdapter) UpdateCurrency(ctx context.Context, in usecase.UpdateCurrencyInput) (currency.Currency, error) {
	return a.svc.UpdateCurrency(ctx, in)
}

func (a *currencyAPIAdapter) CreateFxRate(ctx context.Context, in usecase.CreateFxRateInput) (currency.FxRate, error) {
	return a.svc.CreateFxRate(ctx, in)
}

func (a *currencyAPIAdapter) GetFxRate(ctx context.Context, id uuid.UUID) (currency.FxRate, error) {
	return a.svc.GetFxRate(ctx, id)
}

func (a *currencyAPIAdapter) GetLatestFxRate(ctx context.Context, tenantID uuid.UUID, from, to string, at time.Time) (currency.FxRate, error) {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return a.svc.GetLatestFxRate(ctx, tenantID, from, to, at)
}

func (a *currencyAPIAdapter) ListFxRates(ctx context.Context, tenantID uuid.UUID, from, to string, limit int) ([]currency.FxRate, error) {
	return a.svc.ListFxRates(ctx, tenantID, from, to, limit)
}

func (a *currencyAPIAdapter) Convert(ctx context.Context, in usecase.ConvertInput) (usecase.ConvertOutput, error) {
	return a.svc.Convert(ctx, in)
}

// =============================================================================
// FxRateLookup adapter — used by TransferService for cross-currency transfers
// =============================================================================

// fxRateLookupAdapter exposes the two methods TransferService needs:
//   - GetLatestFxRate (lookup FX rate)
//   - GetCurrency (read decimal_places for FX math)
type fxRateLookupAdapter struct {
	svc *usecase.CurrencyService
}

func (a *fxRateLookupAdapter) GetLatestFxRate(ctx context.Context, tenantID uuid.UUID, from, to string, at time.Time) (currency.FxRate, error) {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return a.svc.GetLatestFxRate(ctx, tenantID, from, to, at)
}

func (a *fxRateLookupAdapter) GetCurrency(ctx context.Context, code string) (currency.Currency, error) {
	return a.svc.GetCurrency(ctx, code)
}
