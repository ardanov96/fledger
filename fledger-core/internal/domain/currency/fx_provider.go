// fx_provider.go — Sprint 26 / Fase 1D follow-up.
//
// FxRateProvider is the abstract rate source. The Sprint 12 enum already
// lists possible sources (manual/api/bank/seed); this interface makes
// the "api" source pluggable so operators can choose between providers
// without code changes.
//
// Implementations live in internal/infra/fxprovider/.
//
// Used by:
//   - usecase.FxRateRefresher — orchestrates fetch → store
//   - worker.FxRateWorker       — periodic scheduler
package currency

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
)

// =============================================================================
// Provider interface
// =============================================================================

// FxRateProvider returns an exchange rate for a single (base, target) pair
// at the current moment. Implementations may fetch from an HTTP API, a bank
// feed, a CSV file, or a deterministic stub for tests.
//
// Implementations MUST:
//   - Return ErrFxProviderUnavailable when the upstream is unreachable
//     (network failure, HTTP 5xx, timeout). Caller treats as transient.
//   - Return ErrFxProviderRateMissing when the upstream returns a response
//     without the requested target (currency not supported by provider).
//   - Return ErrFxProviderInvalid when the response is malformed.
type FxRateProvider interface {
	FetchRate(ctx context.Context, base, target string) (decimal.Decimal, error)
}

// =============================================================================
// Error sentinels
// =============================================================================

var (
	// ErrFxProviderUnavailable — transient upstream failure.
	// Caller logs + retries next tick; does not mark run as 'failed' permanently.
	ErrFxProviderUnavailable = errors.New("fx provider unavailable")

	// ErrFxProviderRateMissing — provider returned OK but the target is not in the response.
	// Treat as warning; skip the pair.
	ErrFxProviderRateMissing = errors.New("fx provider did not return rate for target")

	// ErrFxProviderInvalid — provider returned malformed/unparseable data.
	// Caller logs + skips tick; not retried this cycle.
	ErrFxProviderInvalid = errors.New("fx provider returned invalid data")
)

// =============================================================================
// Provider interface variant — batch fetch (optional optimization)
// =============================================================================

// FxRateBatchProvider is an optional optimization for providers that can
// return multiple rates in one call. Default providers implement only
// FxRateProvider; the refresher calls FetchRate once per pair.
//
// BatchProvider can reduce HTTP roundtrips dramatically (1 vs N) for
// providers like exchangerate-api.com which return all rates for a base
// in one response.
type FxRateBatchProvider interface {
	FxRateProvider
	FetchAllRates(ctx context.Context, base string) (map[string]decimal.Decimal, error)
}

// =============================================================================
// Configuration helpers
// =============================================================================

// CurrencyPair represents a single (base, target) pair to refresh.
type CurrencyPair struct {
	Base   string
	Target string
}

// ParsePair parses "USD/IDR" → CurrencyPair{Base:"USD", Target:"IDR"}.
// Returns error if format invalid (must be exactly "AAA/BBB").
func ParsePair(s string) (CurrencyPair, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			base := s[:i]
			target := s[i+1:]
			if len(base) != 3 || len(target) != 3 {
				return CurrencyPair{}, errors.New("invalid pair: codes must be 3 chars")
			}
			return CurrencyPair{Base: base, Target: target}, nil
		}
	}
	return CurrencyPair{}, errors.New("invalid pair: must be 'AAA/BBB'")
}
