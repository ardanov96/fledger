// Package usecase — CurrencyService.
//
// Use cases for Sprint 12 (Fase 1D — multi-currency):
//   - Convert: pure FX math via money.Convert
//   - LookupRate: get the currently-applicable FX rate for a (tenant, from, to)
//   - LookupRateForTransfer: rate lookup + validation for transfer-time use
//   - ListActiveCurrencies: read-only list for handler
//   - CreateCurrency: admin creates a currency code
//   - CreateFxRate: admin creates an FX rate entry
//
// All write methods go through RunInTxCurrencyDomain so the operation is
// atomic against the underlying Postgres connection pool. The service holds
// narrow dependency interfaces (FxRateLookup, CurrencyRead) for testability.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// Errors (use-case-level, mapped by handler)
// =============================================================================

var (
	// ErrInvalidInput — empty currency codes, negative amounts, etc.
	ErrInvalidInput = errors.New("currency: invalid input")
)

// =============================================================================
// Dependency interfaces (narrow, for testability)
// =============================================================================

// FxRateLookup is a narrow read interface for FX rates. Implemented by an
// adapter over currency.Repository.GetLatestFxRate (and used by TransferService).
type FxRateLookup interface {
	GetLatestFxRate(ctx context.Context, tenantID uuid.UUID, from, to string, at time.Time) (currency.FxRate, error)
}

// CurrencyRead is a narrow read interface for currencies. Used by
// Convert to look up decimal_places for source and target.
type CurrencyRead interface {
	GetCurrency(ctx context.Context, code string) (currency.Currency, error)
}

// CurrencyTxRunner runs a function inside a currency-domain transaction.
type CurrencyTxRunner interface {
	RunInTxCurrencyDomain(ctx context.Context, fn func(currency.Tx) error) error
}

// =============================================================================
// Input / Output
// =============================================================================

// ConvertInput is the input for the Convert use case.
type ConvertInput struct {
	TenantID     uuid.UUID
	FromCurrency string
	ToCurrency   string
	Amount       money.Money // amount in source currency minor units
	At           time.Time   // lookup time (default: now)
}

// ConvertOutput is the result of Convert.
type ConvertOutput struct {
	FromCurrency string
	ToCurrency   string
	FromAmount   money.Money
	ToAmount     money.Money
	Rate         currency.FxRate
	At           time.Time
}

// =============================================================================
// Service
// =============================================================================

// CurrencyService is the use case for multi-currency operations.
type CurrencyService struct {
	repo     currency.Repository
	txRunner CurrencyTxRunner
}

// NewCurrencyService constructs a CurrencyService.
func NewCurrencyService(repo currency.Repository, txRunner CurrencyTxRunner) *CurrencyService {
	return &CurrencyService{repo: repo, txRunner: txRunner}
}

// =============================================================================
// Read APIs
// =============================================================================

// ListActiveCurrencies returns currencies where is_active=TRUE.
func (s *CurrencyService) ListActiveCurrencies(ctx context.Context) ([]currency.Currency, error) {
	return s.repo.ListCurrencies(ctx, true)
}

// ListCurrencies returns all currencies (active + inactive) — admin only.
func (s *CurrencyService) ListCurrencies(ctx context.Context) ([]currency.Currency, error) {
	return s.repo.ListCurrencies(ctx, false)
}

// GetCurrency reads a single currency by code.
func (s *CurrencyService) GetCurrency(ctx context.Context, code string) (currency.Currency, error) {
	if code == "" {
		return currency.Currency{}, fmt.Errorf("%w: empty code", ErrInvalidInput)
	}
	return s.repo.GetCurrency(ctx, code)
}

// GetFxRate reads a single FX rate by id.
func (s *CurrencyService) GetFxRate(ctx context.Context, id uuid.UUID) (currency.FxRate, error) {
	return s.repo.GetFxRate(ctx, id)
}

// ListFxRates returns rate history for a (tenant, from, to) pair.
func (s *CurrencyService) ListFxRates(ctx context.Context, tenantID uuid.UUID, from, to string, limit int) ([]currency.FxRate, error) {
	return s.repo.ListFxRates(ctx, tenantID, from, to, limit)
}

// GetLatestFxRate returns the active rate for (tenant, from, to) at `at`.
func (s *CurrencyService) GetLatestFxRate(ctx context.Context, tenantID uuid.UUID, from, to string, at time.Time) (currency.FxRate, error) {
	if err := validateCurrencyPair(from, to); err != nil {
		return currency.FxRate{}, err
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return s.repo.GetLatestFxRate(ctx, tenantID, from, to, at)
}

// LookupRateForTransfer is the public hook used by TransferService. It
// returns ErrFxRateNotFound if no rate is currently active.
func (s *CurrencyService) LookupRateForTransfer(ctx context.Context, tenantID uuid.UUID, from, to string) (currency.FxRate, error) {
	return s.GetLatestFxRate(ctx, tenantID, from, to, time.Now().UTC())
}

// =============================================================================
// Write APIs (admin)
// =============================================================================

// CreateCurrencyInput is the input for CreateCurrency.
type CreateCurrencyInput struct {
	Code          string
	DecimalPlaces int
	Name          string
	IsActive      bool
}

// CreateCurrency inserts a new currency. Validates code format and decimals.
func (s *CurrencyService) CreateCurrency(ctx context.Context, in CreateCurrencyInput) (currency.Currency, error) {
	if err := validateCurrencyCode(in.Code); err != nil {
		return currency.Currency{}, err
	}
	if in.DecimalPlaces < 0 || in.DecimalPlaces > 6 {
		return currency.Currency{}, fmt.Errorf("%w: decimal_places", currency.ErrInvalidDecimalPlaces)
	}
	if in.Name == "" {
		return currency.Currency{}, fmt.Errorf("%w: name required", ErrInvalidInput)
	}

	c := currency.Currency{
		Code:          in.Code,
		DecimalPlaces: in.DecimalPlaces,
		Name:          in.Name,
		IsActive:      in.IsActive,
	}

	err := s.txRunner.RunInTxCurrencyDomain(ctx, func(tx currency.Tx) error {
		return s.repo.CreateCurrency(ctx, tx, c)
	})
	if err != nil {
		return currency.Currency{}, err
	}

	// Re-read to get created_at (and validate insert succeeded)
	return s.repo.GetCurrency(ctx, in.Code)
}

// UpdateCurrencyInput is the input for UpdateCurrency.
type UpdateCurrencyInput struct {
	Code          string
	DecimalPlaces int
	Name          string
	IsActive      bool
}

// UpdateCurrency updates mutable fields on an existing currency.
func (s *CurrencyService) UpdateCurrency(ctx context.Context, in UpdateCurrencyInput) (currency.Currency, error) {
	if in.Code == "" {
		return currency.Currency{}, fmt.Errorf("%w: code required", ErrInvalidInput)
	}
	if in.DecimalPlaces < 0 || in.DecimalPlaces > 6 {
		return currency.Currency{}, fmt.Errorf("%w: decimal_places", currency.ErrInvalidDecimalPlaces)
	}

	c := currency.Currency{
		Code:          in.Code,
		DecimalPlaces: in.DecimalPlaces,
		Name:          in.Name,
		IsActive:      in.IsActive,
	}

	err := s.txRunner.RunInTxCurrencyDomain(ctx, func(tx currency.Tx) error {
		return s.repo.UpdateCurrency(ctx, tx, in.Code, c)
	})
	if err != nil {
		return currency.Currency{}, err
	}
	return s.repo.GetCurrency(ctx, in.Code)
}

// CreateFxRateInput is the input for CreateFxRate.
type CreateFxRateInput struct {
	TenantID     uuid.UUID
	FromCurrency string
	ToCurrency   string
	Rate         decimal.Decimal
	EffectiveAt  time.Time
	ExpiresAt    time.Time
	Source       string // optional; default "manual"
	CreatedBy    uuid.UUID
}

// CreateFxRate inserts a new FX rate. Validates window + rate > 0 + currencies exist.
func (s *CurrencyService) CreateFxRate(ctx context.Context, in CreateFxRateInput) (currency.FxRate, error) {
	if err := validateCurrencyPair(in.FromCurrency, in.ToCurrency); err != nil {
		return currency.FxRate{}, err
	}
	if in.Rate.IsZero() || in.Rate.IsNegative() {
		return currency.FxRate{}, fmt.Errorf("%w: rate must be > 0", currency.ErrInvalidFxRate)
	}
	if !in.ExpiresAt.After(in.EffectiveAt) {
		return currency.FxRate{}, currency.ErrInvalidWindow
	}
	if in.EffectiveAt.IsZero() || in.ExpiresAt.IsZero() {
		return currency.FxRate{}, fmt.Errorf("%w: effective_at and expires_at required", ErrInvalidInput)
	}
	if in.Source == "" {
		in.Source = currency.FxRateSourceManual
	}
	if !isValidFxSource(in.Source) {
		return currency.FxRate{}, fmt.Errorf("%w: invalid source %q", ErrInvalidInput, in.Source)
	}

	// Validate currencies exist (FK enforces on insert but we want a clean error).
	if _, err := s.repo.GetCurrency(ctx, in.FromCurrency); err != nil {
		return currency.FxRate{}, err
	}
	if _, err := s.repo.GetCurrency(ctx, in.ToCurrency); err != nil {
		return currency.FxRate{}, err
	}

	r := currency.FxRate{
		ID:           uuid.New(),
		TenantID:     in.TenantID,
		FromCurrency: in.FromCurrency,
		ToCurrency:   in.ToCurrency,
		Rate:         in.Rate,
		EffectiveAt:  in.EffectiveAt,
		ExpiresAt:    in.ExpiresAt,
		Source:       in.Source,
		CreatedBy:    in.CreatedBy,
	}

	err := s.txRunner.RunInTxCurrencyDomain(ctx, func(tx currency.Tx) error {
		return s.repo.CreateFxRate(ctx, tx, r)
	})
	if err != nil {
		return currency.FxRate{}, err
	}
	return s.repo.GetFxRate(ctx, r.ID)
}

// =============================================================================
// Convert (FX math)
// =============================================================================

// Convert computes target amount in `to_currency` minor units using the
// currently-active FX rate for (tenant, from, to) at `in.At` (default now).
//
// Returns ErrFxRateNotFound if no active rate. The conversion itself uses
// money.Convert which handles decimal-place shifts (JPY 0dp -> IDR 2dp).
func (s *CurrencyService) Convert(ctx context.Context, in ConvertInput) (ConvertOutput, error) {
	if err := validateCurrencyPair(in.FromCurrency, in.ToCurrency); err != nil {
		return ConvertOutput{}, err
	}
	if in.Amount <= 0 {
		return ConvertOutput{}, fmt.Errorf("%w: amount must be > 0", ErrInvalidInput)
	}
	if in.TenantID == uuid.Nil {
		return ConvertOutput{}, fmt.Errorf("%w: tenant_id required", ErrInvalidInput)
	}
	if in.At.IsZero() {
		in.At = time.Now().UTC()
	}

	// Same-currency: identity, no rate lookup.
	if in.FromCurrency == in.ToCurrency {
		return ConvertOutput{
			FromCurrency: in.FromCurrency,
			ToCurrency:   in.ToCurrency,
			FromAmount:   in.Amount,
			ToAmount:     in.Amount,
			At:           in.At,
		}, nil
	}

	rate, err := s.repo.GetLatestFxRate(ctx, in.TenantID, in.FromCurrency, in.ToCurrency, in.At)
	if err != nil {
		return ConvertOutput{}, err
	}

	// Look up decimal places for both currencies.
	fromCur, err := s.repo.GetCurrency(ctx, in.FromCurrency)
	if err != nil {
		return ConvertOutput{}, err
	}
	toCur, err := s.repo.GetCurrency(ctx, in.ToCurrency)
	if err != nil {
		return ConvertOutput{}, err
	}

	toAmount, err := money.Convert(in.Amount, fromCur.DecimalPlaces, toCur.DecimalPlaces, rate.Rate)
	if err != nil {
		return ConvertOutput{}, fmt.Errorf("convert: %w", err)
	}

	return ConvertOutput{
		FromCurrency: in.FromCurrency,
		ToCurrency:   in.ToCurrency,
		FromAmount:   in.Amount,
		ToAmount:     toAmount,
		Rate:         rate,
		At:           in.At,
	}, nil
}

// =============================================================================
// Validation helpers
// =============================================================================

func validateCurrencyCode(code string) error {
	if code == "" {
		return fmt.Errorf("%w: empty code", currency.ErrInvalidCurrencyCode)
	}
	if len(code) < 3 || len(code) > 6 {
		return fmt.Errorf("%w: code length must be 3..6 chars", currency.ErrInvalidCurrencyCode)
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return fmt.Errorf("%w: code must be uppercase A-Z", currency.ErrInvalidCurrencyCode)
		}
	}
	return nil
}

func validateCurrencyPair(from, to string) error {
	if err := validateCurrencyCode(from); err != nil {
		return err
	}
	if err := validateCurrencyCode(to); err != nil {
		return err
	}
	if from == to {
		return currency.ErrSameCurrency
	}
	return nil
}

func isValidFxSource(s string) bool {
	switch s {
	case currency.FxRateSourceManual,
		currency.FxRateSourceAPI,
		currency.FxRateSourceBank,
		currency.FxRateSourceSeed:
		return true
	}
	return false
}
