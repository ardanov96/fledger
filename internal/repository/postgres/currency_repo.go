// currency_repo.go — Postgres-backed implementation of currency.Repository.
//
// Methods mirror collection_repo / period_repo style. Tx-bound writes; read
// methods can run outside a tx.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
)

// =============================================================================
// DTOs
// =============================================================================

type currencyDTO struct {
	Code          string
	DecimalPlaces int
	Name          string
	IsActive      bool
	CreatedAt     time.Time
}

type fxRateDTO struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	FromCurrency string
	ToCurrency   string
	Rate         decimal.Decimal
	EffectiveAt  time.Time
	ExpiresAt    time.Time
	Source       string
	CreatedBy    uuid.UUID
	CreatedAt    time.Time
}

// =============================================================================
// Repo
// =============================================================================

// CurrencyRepository implements currency.Repository against Postgres.
// Tx-bound methods accept a currency.Tx (wrapping pgx.Tx) so use cases
// control the transaction boundary.
type CurrencyRepository struct {
	db *DB
}

// NewCurrencyRepository constructs a CurrencyRepository.
func NewCurrencyRepository(db *DB) *CurrencyRepository {
	return &CurrencyRepository{db: db}
}

// =============================================================================
// Currencies
// =============================================================================

// CreateCurrency inserts a new currency. Tx-bound.
func (r *CurrencyRepository) CreateCurrency(ctx context.Context, tx currency.Tx, c currency.Currency) error {
	pgxTx, err := UnwrapPgxTxFromCurrency(tx)
	if err != nil {
		return err
	}

	tag, err := pgxTx.Exec(ctx,
		`INSERT INTO currencies (code, decimal_places, name, is_active)
         VALUES ($1, $2, $3, $4)`,
		c.Code, c.DecimalPlaces, c.Name, c.IsActive,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return currency.ErrCurrencyAlreadyExists
		}
		return fmt.Errorf("create currency: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("create currency: expected 1 row, got %d", tag.RowsAffected())
	}
	return nil
}

// GetCurrency reads a currency by code. No tx required (read-only).
func (r *CurrencyRepository) GetCurrency(ctx context.Context, code string) (currency.Currency, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT code, decimal_places, name, is_active, created_at
         FROM currencies
         WHERE code = $1`,
		code,
	)
	var d currencyDTO
	if err := row.Scan(&d.Code, &d.DecimalPlaces, &d.Name, &d.IsActive, &d.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return currency.Currency{}, currency.ErrCurrencyNotFound
		}
		return currency.Currency{}, fmt.Errorf("get currency: %w", err)
	}
	return dtoToCurrency(d), nil
}

// ListCurrencies returns all currencies, optionally filtered by is_active.
func (r *CurrencyRepository) ListCurrencies(ctx context.Context, onlyActive bool) ([]currency.Currency, error) {
	q := `SELECT code, decimal_places, name, is_active, created_at FROM currencies`
	if onlyActive {
		q += ` WHERE is_active = TRUE`
	}
	q += ` ORDER BY code`

	rows, err := r.db.Pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list currencies: %w", err)
	}
	defer rows.Close()

	var out []currency.Currency
	for rows.Next() {
		var d currencyDTO
		if err := rows.Scan(&d.Code, &d.DecimalPlaces, &d.Name, &d.IsActive, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan currency: %w", err)
		}
		out = append(out, dtoToCurrency(d))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return out, nil
}

// UpdateCurrency updates mutable fields (decimal_places, name, is_active).
// Code is the natural PK and cannot be changed.
func (r *CurrencyRepository) UpdateCurrency(ctx context.Context, tx currency.Tx, code string, c currency.Currency) error {
	pgxTx, err := UnwrapPgxTxFromCurrency(tx)
	if err != nil {
		return err
	}

	tag, err := pgxTx.Exec(ctx,
		`UPDATE currencies
         SET decimal_places = $2, name = $3, is_active = $4
         WHERE code = $1`,
		code, c.DecimalPlaces, c.Name, c.IsActive,
	)
	if err != nil {
		return fmt.Errorf("update currency: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return currency.ErrCurrencyNotFound
	}
	return nil
}

// =============================================================================
// FX Rates
// =============================================================================

// CreateFxRate inserts a new FX rate. Tx-bound.
func (r *CurrencyRepository) CreateFxRate(ctx context.Context, tx currency.Tx, fr currency.FxRate) error {
	pgxTx, err := UnwrapPgxTxFromCurrency(tx)
	if err != nil {
		return err
	}

	tag, err := pgxTx.Exec(ctx,
		`INSERT INTO fx_rates
            (id, tenant_id, from_currency, to_currency, rate, effective_at, expires_at, source, created_by)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		fr.ID, fr.TenantID, fr.FromCurrency, fr.ToCurrency, fr.Rate,
		fr.EffectiveAt, fr.ExpiresAt, fr.Source, fr.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("create fx rate: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("create fx rate: expected 1 row, got %d", tag.RowsAffected())
	}
	return nil
}

// GetFxRate reads a rate by id.
func (r *CurrencyRepository) GetFxRate(ctx context.Context, id uuid.UUID) (currency.FxRate, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, tenant_id, from_currency, to_currency, rate,
                effective_at, expires_at, source, created_by, created_at
         FROM fx_rates
         WHERE id = $1`,
		id,
	)
	d, err := scanFxRate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return currency.FxRate{}, currency.ErrFxRateNotFound
		}
		return currency.FxRate{}, fmt.Errorf("get fx rate: %w", err)
	}
	return dtoToFxRate(d), nil
}

// GetLatestFxRate returns the most recent active rate for (from, to) at time t.
// "Active" means: effective_at <= t < expires_at. Most recent = largest effective_at.
func (r *CurrencyRepository) GetLatestFxRate(ctx context.Context, tenantID uuid.UUID, from, to string, at time.Time) (currency.FxRate, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, tenant_id, from_currency, to_currency, rate,
                effective_at, expires_at, source, created_by, created_at
         FROM fx_rates
         WHERE tenant_id    = $1
           AND from_currency = $2
           AND to_currency   = $3
           AND effective_at <= $4
           AND expires_at    > $4
         ORDER BY effective_at DESC
         LIMIT 1`,
		tenantID, from, to, at,
	)
	d, err := scanFxRate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return currency.FxRate{}, currency.ErrFxRateNotFound
		}
		return currency.FxRate{}, fmt.Errorf("get latest fx rate: %w", err)
	}
	return dtoToFxRate(d), nil
}

// ListFxRates returns rates for a (tenant, from, to) pair, most recent first.
// limit clamped to 1..500 by the caller; we apply a sensible cap here.
func (r *CurrencyRepository) ListFxRates(ctx context.Context, tenantID uuid.UUID, from, to string, limit int) ([]currency.FxRate, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	q := `SELECT id, tenant_id, from_currency, to_currency, rate,
                 effective_at, expires_at, source, created_by, created_at
          FROM fx_rates
          WHERE tenant_id = $1`
	args := []any{tenantID}
	idx := 2
	if from != "" {
		q += fmt.Sprintf(` AND from_currency = $%d`, idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		q += fmt.Sprintf(` AND to_currency = $%d`, idx)
		args = append(args, to)
		idx++
	}
	q += fmt.Sprintf(` ORDER BY effective_at DESC LIMIT $%d`, idx)
	args = append(args, limit)

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list fx rates: %w", err)
	}
	defer rows.Close()

	var out []currency.FxRate
	for rows.Next() {
		d, err := scanFxRate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan fx rate: %w", err)
		}
		out = append(out, dtoToFxRate(d))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return out, nil
}

// =============================================================================
// Helpers
// =============================================================================

func scanFxRate(row pgx.Row) (fxRateDTO, error) {
	var d fxRateDTO
	err := row.Scan(
		&d.ID, &d.TenantID, &d.FromCurrency, &d.ToCurrency, &d.Rate,
		&d.EffectiveAt, &d.ExpiresAt, &d.Source, &d.CreatedBy, &d.CreatedAt,
	)
	if err != nil {
		return fxRateDTO{}, err
	}
	return d, nil
}

func dtoToCurrency(d currencyDTO) currency.Currency {
	return currency.Currency{
		Code:          d.Code,
		DecimalPlaces: d.DecimalPlaces,
		Name:          d.Name,
		IsActive:      d.IsActive,
		CreatedAt:     d.CreatedAt,
	}
}

func dtoToFxRate(d fxRateDTO) currency.FxRate {
	return currency.FxRate{
		ID:           d.ID,
		TenantID:     d.TenantID,
		FromCurrency: d.FromCurrency,
		ToCurrency:   d.ToCurrency,
		Rate:         d.Rate,
		EffectiveAt:  d.EffectiveAt,
		ExpiresAt:    d.ExpiresAt,
		Source:       d.Source,
		CreatedBy:    d.CreatedBy,
		CreatedAt:    d.CreatedAt,
	}
}
