package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-order/internal/platform/errors"
	"github.com/fledger/fledger-order/internal/domain/product"
)

type ProductRepo struct {
	pool *pgxpool.Pool
}

func NewProductRepo(p *pgxpool.Pool) *ProductRepo { return &ProductRepo{pool: p} }

const prodColumns = `id, tenant_id, sku, COALESCE(barcode,''), name, category, unit,
	weight_grams, is_active, metadata, created_at, updated_at`

func scanProduct(row pgx.Row) (product.Product, error) {
	var p product.Product
	var category, unit string
	var meta []byte
	if err := row.Scan(
		&p.ID, &p.TenantID, &p.SKU, &p.Barcode, &p.Name, &category, &unit,
		&p.WeightGrams, &p.IsActive, &meta, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return product.Product{}, err
	}
	p.Category = product.Category(category)
	p.Unit = product.Unit(unit)
	if len(meta) > 0 {
		_ = jsonUnmarshal(meta, &p.Metadata)
	}
	return p, nil
}

func (r *ProductRepo) Insert(ctx context.Context, p product.Product) (product.Product, error) {
	if p.Category == "" {
		p.Category = product.CategorySembako
	}
	if p.Unit == "" {
		p.Unit = product.UnitDus
	}
	if p.WeightGrams <= 0 {
		p.WeightGrams = 1000
	}
	meta := p.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	metaJSON, _ := jsonMarshal(meta)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO order_products
		  (tenant_id, sku, barcode, name, category, unit, weight_grams, is_active, metadata)
		VALUES ($1::uuid, $2, NULLIF($3,''), $4, $5, $6, $7, $8, $9::jsonb)
		RETURNING `+prodColumns,
		p.TenantID, p.SKU, p.Barcode, p.Name, string(p.Category), string(p.Unit),
		p.WeightGrams, p.IsActive, metaJSON,
	)
	out, err := scanProduct(row)
	if err != nil {
		if isUniqueViolation(err) {
			return product.Product{}, fmt.Errorf("%w: sku already exists", apperrors.ErrConflict)
		}
		return product.Product{}, fmt.Errorf("insert product: %w", err)
	}
	return out, nil
}

func (r *ProductRepo) Get(ctx context.Context, tenantID, id string) (product.Product, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+prodColumns+` FROM order_products WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	p, err := scanProduct(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return product.Product{}, fmt.Errorf("%w: product %s", apperrors.ErrNotFound, id)
		}
		return product.Product{}, err
	}
	return p, nil
}

func (r *ProductRepo) List(ctx context.Context, tenantID, category string) ([]product.Product, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if category == "" {
		rows, err = r.pool.Query(ctx, `SELECT `+prodColumns+` FROM order_products WHERE tenant_id=$1 ORDER BY sku`, tenantID)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT `+prodColumns+` FROM order_products WHERE tenant_id=$1 AND category=$2 ORDER BY sku`, tenantID, category)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]product.Product, 0, 8)
	for rows.Next() {
		v, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}