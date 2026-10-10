package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-order/internal/platform/errors"
	"github.com/fledger/fledger-order/internal/domain/pricing"
)

type PricingRepo struct {
	pool *pgxpool.Pool
}

func NewPricingRepo(p *pgxpool.Pool) *PricingRepo { return &PricingRepo{pool: p} }

const priceColumns = `id, tenant_id, product_id, tier, min_quantity, unit_price, created_at, updated_at`

func scanPrice(row pgx.Row) (pricing.PriceTier, error) {
	var t pricing.PriceTier
	var tier string
	if err := row.Scan(
		&t.ID, &t.TenantID, &t.ProductID, &tier, &t.MinQuantity, &t.UnitPrice,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return pricing.PriceTier{}, err
	}
	t.Tier = pricing.Tier(tier)
	return t, nil
}

func (r *PricingRepo) Insert(ctx context.Context, t pricing.PriceTier) (pricing.PriceTier, error) {
	if !t.Tier.Valid() {
		return pricing.PriceTier{}, fmt.Errorf("%w: invalid tier %q", apperrors.ErrInvalidInput, t.Tier)
	}
	if t.MinQuantity < 1 {
		t.MinQuantity = 1
	}
	if t.UnitPrice <= 0 {
		return pricing.PriceTier{}, fmt.Errorf("%w: unit_price must be > 0", apperrors.ErrInvalidInput)
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO order_price_tiers
		  (tenant_id, product_id, tier, min_quantity, unit_price)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)
		RETURNING `+priceColumns,
		t.TenantID, t.ProductID, string(t.Tier), t.MinQuantity, t.UnitPrice,
	)
	out, err := scanPrice(row)
	if err != nil {
		if isUniqueViolation(err) {
			return pricing.PriceTier{}, fmt.Errorf("%w: tier/min_quantity already exists", apperrors.ErrConflict)
		}
		return pricing.PriceTier{}, fmt.Errorf("insert pricing: %w", err)
	}
	return out, nil
}

// FindBestPrice returns the best applicable PriceTier for a (product, tier, qty).
// It picks the highest min_quantity <= qty (i.e. volume discount).
func (r *PricingRepo) FindBestPrice(ctx context.Context, tenantID, productID string, tier pricing.Tier, qty int) (pricing.PriceTier, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+priceColumns+` FROM order_price_tiers
		 WHERE tenant_id=$1 AND product_id=$2 AND tier=$3 AND min_quantity <= $4
		 ORDER BY min_quantity DESC LIMIT 1`,
		tenantID, productID, string(tier), qty,
	)
	t, err := scanPrice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pricing.PriceTier{}, fmt.Errorf("%w: no price tier for product/tier/qty", apperrors.ErrNotFound)
		}
		return pricing.PriceTier{}, err
	}
	return t, nil
}

// ListByProduct returns all price tiers for a product (for admin views).
func (r *PricingRepo) ListByProduct(ctx context.Context, productID string) ([]pricing.PriceTier, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+priceColumns+` FROM order_price_tiers WHERE product_id=$1 ORDER BY tier, min_quantity`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]pricing.PriceTier, 0, 4)
	for rows.Next() {
		v, err := scanPrice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}