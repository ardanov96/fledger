// Package currency defines the multi-currency domain — currencies registry
// and FX rate history with snapshot semantics per transaction.
//
// Sprint 12 / Fase 1D. All types are pure (no infra deps). Repository is an
// interface so use case can be unit-tested with an in-memory fake.
//
// Tx abstraction mirrors collection/period/reconciler — each domain defines
// its own narrow Tx so use cases don't import pgx directly.
package currency

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ============================================================================
// Errors (sentinels for use case / handler mapping)
// ============================================================================

var (
	// ErrCurrencyNotFound — currency code not in registry.
	ErrCurrencyNotFound = errors.New("currency not found")

	// ErrCurrencyAlreadyExists — code conflict on create.
	ErrCurrencyAlreadyExists = errors.New("currency already exists")

	// ErrCurrencyInactive — currency deactivated, no new ops allowed.
	ErrCurrencyInactive = errors.New("currency is inactive")

	// ErrFxRateNotFound — no active rate for the (from, to) pair at given time.
	ErrFxRateNotFound = errors.New("fx rate not found")

	// ErrFxRateExpired — rate exists but expires_at <= now.
	ErrFxRateExpired = errors.New("fx rate has expired")

	// ErrFxRateMismatch — provided fx_rate_id does not match the (from, to) pair.
	ErrFxRateMismatch = errors.New("fx rate does not match currency pair")

	// ErrInvalidCurrencyCode — empty / too long / non-uppercase.
	ErrInvalidCurrencyCode = errors.New("invalid currency code")

	// ErrInvalidDecimalPlaces — out of range (0..6).
	ErrInvalidDecimalPlaces = errors.New("invalid decimal_places (must be 0..6)")

	// ErrInvalidFxRate — rate <= 0 or window invalid.
	ErrInvalidFxRate = errors.New("invalid fx rate")

	// ErrInvalidWindow — effective_at >= expires_at.
	ErrInvalidWindow = errors.New("fx rate effective_at must be before expires_at")

	// ErrSameCurrency — fx rate declared with from == to.
	ErrSameCurrency = errors.New("fx rate from_currency must differ from to_currency")
)

// ============================================================================
// Currency entity
// ============================================================================

// Currency is a registered ISO 4217 currency. Code is the natural PK.
type Currency struct {
	Code          string    // ISO 4217 (e.g. "IDR", "USD", "JPY")
	DecimalPlaces int       // 0..6 (IDR=2, JPY=0, BHD=3)
	Name          string    // human-readable
	IsActive      bool      // soft-delete for currencies
	CreatedAt     time.Time
}

// ============================================================================
// FxRate entity
// ============================================================================

// FxRate is a snapshot of an exchange rate between two currencies over a
// validity window. Multiple rates can stack over time (history). The rate
// applicable at transfer-time is the most recent with effective_at <= t < expires_at.
type FxRate struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	FromCurrency string
	ToCurrency   string
	Rate         decimal.Decimal // high precision (NUMERIC(20,10))
	EffectiveAt  time.Time
	ExpiresAt    time.Time
	Source       string // "manual" | "api" | "bank" | "seed"
	CreatedBy    uuid.UUID
	CreatedAt    time.Time
}

// FxRate sources — only these values are accepted on insert.
const (
	FxRateSourceManual = "manual"
	FxRateSourceAPI    = "api"
	FxRateSourceBank   = "bank"
	FxRateSourceSeed   = "seed"
)

// IsActive reports whether the rate is currently valid at the given time.
func (r FxRate) IsActive(at time.Time) bool {
	return !at.Before(r.EffectiveAt) && at.Before(r.ExpiresAt)
}

// ============================================================================
// Transaction abstraction (mirrors collection/reconciler pattern)
// ============================================================================

// Tx is the transaction abstraction used by the currency module.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// CommandTag is the result of an Exec.
type CommandTag interface {
	RowsAffected() int64
}

// Row is a single-row query result.
type Row interface {
	Scan(dest ...any) error
}

// Rows is a multi-row query result.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// ============================================================================
// Repository interface
// ============================================================================

// Repository is the persistence boundary for currencies and FX rates. The
// implementation lives in internal/repository/postgres. All write methods
// take a Tx so use cases control the transaction boundary.
type Repository interface {
	// ---- Currencies ----
	CreateCurrency(ctx context.Context, tx Tx, c Currency) error
	GetCurrency(ctx context.Context, code string) (Currency, error)
	ListCurrencies(ctx context.Context, onlyActive bool) ([]Currency, error)
	UpdateCurrency(ctx context.Context, tx Tx, code string, c Currency) error

	// ---- FX Rates ----
	CreateFxRate(ctx context.Context, tx Tx, r FxRate) error
	GetFxRate(ctx context.Context, id uuid.UUID) (FxRate, error)
	GetLatestFxRate(ctx context.Context, tenantID uuid.UUID, from, to string, at time.Time) (FxRate, error)
	ListFxRates(ctx context.Context, tenantID uuid.UUID, from, to string, limit int) ([]FxRate, error)
}

// ============================================================================
// Convenience
// ============================================================================

// NewID is a convenience wrapper around uuid.NewString (exported for callers).
func NewID() string {
	return uuid.NewString()
}
