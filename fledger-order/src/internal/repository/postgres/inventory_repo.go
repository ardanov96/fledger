package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-order/internal/platform/errors"
	"github.com/fledger/fledger-order/internal/domain/inventory"
)

type InventoryRepo struct {
	pool *pgxpool.Pool
}

func NewInventoryRepo(p *pgxpool.Pool) *InventoryRepo { return &InventoryRepo{pool: p} }

const stockColumns = `id, tenant_id, product_id, warehouse_id, on_hand_qty,
	reserved_qty, available_qty, created_at, updated_at`

func scanStock(row pgx.Row) (inventory.Stock, error) {
	var s inventory.Stock
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.ProductID, &s.WarehouseID, &s.OnHandQty,
		&s.ReservedQty, &s.AvailableQty, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return inventory.Stock{}, err
	}
	return s, nil
}

// Get returns the stock for a product in a warehouse (creates if missing).
func (r *InventoryRepo) Get(ctx context.Context, tenantID, productID, warehouseID string) (inventory.Stock, error) {
	if warehouseID == "" {
		warehouseID = "WH-CENTRAL-01"
	}
	row := r.pool.QueryRow(ctx, `SELECT `+stockColumns+` FROM order_inventory_stocks WHERE tenant_id=$1 AND product_id=$2 AND warehouse_id=$3`, tenantID, productID, warehouseID)
	s, err := scanStock(row)
	if err == nil {
		return s, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return inventory.Stock{}, err
	}
	// Auto-create with 0 qty.
	row = r.pool.QueryRow(ctx, `
		INSERT INTO order_inventory_stocks (tenant_id, product_id, warehouse_id, on_hand_qty, reserved_qty)
		VALUES ($1::uuid, $2::uuid, $3, 0, 0)
		RETURNING `+stockColumns, tenantID, productID, warehouseID)
	return scanStock(row)
}

// AdjustOnHand increments on_hand_qty (positive or negative) and returns the
// updated stock row.
func (r *InventoryRepo) AdjustOnHand(ctx context.Context, tenantID, productID, warehouseID string, delta int) (inventory.Stock, error) {
	if warehouseID == "" {
		warehouseID = "WH-CENTRAL-01"
	}
	// Ensure row exists, then update atomically.
	if _, err := r.Get(ctx, tenantID, productID, warehouseID); err != nil {
		return inventory.Stock{}, err
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE order_inventory_stocks
		   SET on_hand_qty = GREATEST(on_hand_qty + $4, 0), updated_at = NOW()
		 WHERE tenant_id=$1 AND product_id=$2 AND warehouse_id=$3
		 RETURNING `+stockColumns, tenantID, productID, warehouseID, delta)
	return scanStock(row)
}

// Reserve atomically increments reserved_qty by qty IF available_qty >= qty.
// Returns ErrStockInsufficient when not enough stock.
func (r *InventoryRepo) Reserve(ctx context.Context, tenantID, productID, warehouseID string, qty int) (inventory.Stock, error) {
	if qty <= 0 {
		return inventory.Stock{}, fmt.Errorf("%w: qty must be > 0", apperrors.ErrInvalidInput)
	}
	if warehouseID == "" {
		warehouseID = "WH-CENTRAL-01"
	}
	if _, err := r.Get(ctx, tenantID, productID, warehouseID); err != nil {
		return inventory.Stock{}, err
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE order_inventory_stocks
		   SET reserved_qty = reserved_qty + $4, updated_at = NOW()
		 WHERE tenant_id=$1::uuid AND product_id=$2::uuid AND warehouse_id=$3::text
		   AND (on_hand_qty - reserved_qty) >= $4
		 RETURNING `+stockColumns, tenantID, productID, warehouseID, qty)
	s, err := scanStock(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return inventory.Stock{}, apperrors.ErrStockInsufficient
		}
		return inventory.Stock{}, err
	}
	return s, nil
}

// Release decrements reserved_qty (used on cancel / override).
func (r *InventoryRepo) Release(ctx context.Context, tenantID, productID, warehouseID string, qty int) (inventory.Stock, error) {
	if qty <= 0 {
		return inventory.Stock{}, fmt.Errorf("%w: qty must be > 0", apperrors.ErrInvalidInput)
	}
	if warehouseID == "" {
		warehouseID = "WH-CENTRAL-01"
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE order_inventory_stocks
		   SET reserved_qty = GREATEST(reserved_qty - $4, 0), updated_at = NOW()
		 WHERE tenant_id=$1 AND product_id=$2 AND warehouse_id=$3
		 RETURNING `+stockColumns, tenantID, productID, warehouseID, qty)
	return scanStock(row)
}

// ListByProduct returns all stock rows for a product.
func (r *InventoryRepo) ListByProduct(ctx context.Context, productID string) ([]inventory.Stock, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+stockColumns+` FROM order_inventory_stocks WHERE product_id=$1 ORDER BY warehouse_id`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]inventory.Stock, 0, 2)
	for rows.Next() {
		v, err := scanStock(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}