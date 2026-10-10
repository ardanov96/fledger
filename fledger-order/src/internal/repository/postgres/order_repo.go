package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-order/internal/platform/errors"
	"github.com/fledger/fledger-order/internal/domain/order"
)

type OrderRepo struct {
	pool *pgxpool.Pool
}

func NewOrderRepo(p *pgxpool.Pool) *OrderRepo { return &OrderRepo{pool: p} }

const orderColumns = `id, tenant_id, order_number, customer_id, customer_name, customer_tier,
	COALESCE(customer_phone,''), destination_address, total_weight_kg, subtotal, discount, total_amount,
	status, credit_gate_status, credit_check_details, COALESCE(override_by,''), COALESCE(override_reason,''),
	COALESCE(fledger_fleet_do_id::text,''), COALESCE(notes,''), created_at, updated_at`

const itemColumns = `id, tenant_id, order_id, product_id, sku, name, quantity, unit_price,
	line_total, weight_grams, created_at`

func scanOrder(row pgx.Row) (order.Order, error) {
	var o order.Order
	var status, cgs string
	var meta []byte
	if err := row.Scan(
		&o.ID, &o.TenantID, &o.OrderNumber, &o.CustomerID, &o.CustomerName, &o.CustomerTier,
		&o.CustomerPhone, &o.DestinationAddress, &o.TotalWeightKg, &o.Subtotal, &o.Discount, &o.TotalAmount,
		&status, &cgs, &meta, &o.OverrideBy, &o.OverrideReason,
		&o.FledgerFleetDOID, &o.Notes, &o.CreatedAt, &o.UpdatedAt,
	); err != nil {
		return order.Order{}, err
	}
	o.Status = order.Status(status)
	o.CreditGateStatus = order.CreditGateStatus(cgs)
	if len(meta) > 0 {
		_ = jsonUnmarshal(meta, &o.CreditCheckDetails)
	}
	return o, nil
}

func scanItem(row pgx.Row) (order.Item, error) {
	var it order.Item
	if err := row.Scan(
		&it.ID, &it.TenantID, &it.OrderID, &it.ProductID, &it.SKU, &it.Name,
		&it.Quantity, &it.UnitPrice, &it.LineTotal, &it.WeightGrams, &it.CreatedAt,
	); err != nil {
		return order.Item{}, err
	}
	return it, nil
}

// CreateWithItems inserts an order + its items in one transaction and
// returns the persisted order and items.
func (r *OrderRepo) CreateWithItems(ctx context.Context, o order.Order, items []order.Item) (order.Order, []order.Item, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return order.Order{}, nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	meta := o.CreditCheckDetails
	if meta == nil {
		meta = map[string]any{}
	}
	metaJSON, _ := jsonMarshal(meta)

	row := tx.QueryRow(ctx, `
		INSERT INTO order_orders
		  (tenant_id, order_number, customer_id, customer_name, customer_tier,
		   customer_phone, destination_address, total_weight_kg, subtotal, discount, total_amount,
		   status, credit_gate_status, credit_check_details, notes)
		VALUES ($1::uuid, $2, $3::uuid, $4, $5, NULLIF($6,''), $7, $8, $9, $10, $11, $12, $13, $14::jsonb, NULLIF($15,''))
		RETURNING `+orderColumns,
		o.TenantID, o.OrderNumber, o.CustomerID, o.CustomerName, o.CustomerTier,
		o.CustomerPhone, o.DestinationAddress, o.TotalWeightKg, o.Subtotal, o.Discount, o.TotalAmount,
		string(o.Status), string(o.CreditGateStatus), metaJSON, o.Notes,
	)
	created, err := scanOrder(row)
	if err != nil {
		if isUniqueViolation(err) {
			return order.Order{}, nil, fmt.Errorf("%w: order_number already exists", apperrors.ErrConflict)
		}
		return order.Order{}, nil, fmt.Errorf("insert order: %w", err)
	}

	out := make([]order.Item, 0, len(items))
	for _, it := range items {
		it.OrderID = created.ID
		row := tx.QueryRow(ctx, `
			INSERT INTO order_items
			  (tenant_id, order_id, product_id, sku, name, quantity, unit_price, line_total, weight_grams)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9)
			RETURNING `+itemColumns,
			it.TenantID, it.OrderID, it.ProductID, it.SKU, it.Name,
			it.Quantity, it.UnitPrice, it.LineTotal, it.WeightGrams,
		)
		got, err := scanItem(row)
		if err != nil {
			return order.Order{}, nil, fmt.Errorf("insert item: %w", err)
		}
		out = append(out, got)
	}

	if err := tx.Commit(ctx); err != nil {
		return order.Order{}, nil, fmt.Errorf("commit: %w", err)
	}
	return created, out, nil
}

// Get fetches a single order.
func (r *OrderRepo) Get(ctx context.Context, tenantID, id string) (order.Order, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+orderColumns+` FROM order_orders WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return order.Order{}, fmt.Errorf("%w: order %s", apperrors.ErrNotFound, id)
		}
		return order.Order{}, err
	}
	return o, nil
}

// ListItems returns all items for an order.
func (r *OrderRepo) ListItems(ctx context.Context, orderID string) ([]order.Item, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+itemColumns+` FROM order_items WHERE order_id=$1 ORDER BY created_at`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]order.Item, 0, 8)
	for rows.Next() {
		v, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// List returns recent orders for the tenant.
func (r *OrderRepo) List(ctx context.Context, tenantID, status string) ([]order.Order, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		rows, err = r.pool.Query(ctx, `SELECT `+orderColumns+` FROM order_orders WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT `+orderColumns+` FROM order_orders WHERE tenant_id=$1 AND status=$2 ORDER BY created_at DESC`, tenantID, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]order.Order, 0, 8)
	for rows.Next() {
		v, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// UpdateStatus moves an order to a new status.
func (r *OrderRepo) UpdateStatus(ctx context.Context, tenantID, id string, status order.Status, extra map[string]any) (order.Order, error) {
	if !status.Valid() {
		return order.Order{}, fmt.Errorf("%w: invalid status %q", apperrors.ErrInvalidInput, status)
	}
	meta := extra
	if meta == nil {
		meta = map[string]any{}
	}
	metaJSON, _ := jsonMarshal(meta)
	cgs := creditGateStatusFromMap(extra)
	row := r.pool.QueryRow(ctx, `
		UPDATE order_orders
		   SET status=$3, credit_gate_status=COALESCE(NULLIF($4,'')::varchar, credit_gate_status),
		       credit_check_details=COALESCE($5::jsonb, credit_check_details),
		       updated_at=NOW()
		 WHERE tenant_id=$1::uuid AND id=$2::uuid
		 RETURNING `+orderColumns, tenantID, id, string(status), cgs, metaJSON)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return order.Order{}, fmt.Errorf("%w: order %s", apperrors.ErrNotFound, id)
		}
		return order.Order{}, err
	}
	return o, nil
}

// UpdateStatusTx moves an order to a new status within an active transaction.
func (r *OrderRepo) UpdateStatusTx(ctx context.Context, tx pgx.Tx, tenantID, id string, status order.Status, extra map[string]any) (order.Order, error) {
	if !status.Valid() {
		return order.Order{}, fmt.Errorf("%w: invalid status %q", apperrors.ErrInvalidInput, status)
	}
	meta := extra
	if meta == nil {
		meta = map[string]any{}
	}
	metaJSON, _ := jsonMarshal(meta)
	cgs := creditGateStatusFromMap(extra)
	row := tx.QueryRow(ctx, `
		UPDATE order_orders
		   SET status=$3, credit_gate_status=COALESCE(NULLIF($4,'')::varchar, credit_gate_status),
		       credit_check_details=COALESCE($5::jsonb, credit_check_details),
		       updated_at=NOW()
		 WHERE tenant_id=$1::uuid AND id=$2::uuid
		 RETURNING `+orderColumns, tenantID, id, string(status), cgs, metaJSON)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return order.Order{}, fmt.Errorf("%w: order %s", apperrors.ErrNotFound, id)
		}
		return order.Order{}, err
	}
	return o, nil
}

// creditGateStatusFromMap returns the value of key "credit_gate_status" if set.
func creditGateStatusFromMap(m map[string]any) string {
	if m == nil {
		return ""
	}
	if v, ok := m["credit_gate_status"].(string); ok {
		return v
	}
	return ""
}

// ApplyOverride sets override_by, override_reason, status=APPROVED, credit_gate_status=OVERRIDDEN.
func (r *OrderRepo) ApplyOverride(ctx context.Context, tenantID, id, overrideBy, reason string, details map[string]any) (order.Order, error) {
	det := details
	if det == nil {
		det = map[string]any{}
	}
	detJSON, _ := jsonMarshal(det)
	row := r.pool.QueryRow(ctx, `
		UPDATE order_orders
		   SET status='APPROVED', credit_gate_status='OVERRIDDEN',
		       override_by=$3, override_reason=$4,
		       credit_check_details=$5::jsonb, updated_at=NOW()
		 WHERE tenant_id=$1::uuid AND id=$2::uuid
		 RETURNING `+orderColumns, tenantID, id, overrideBy, reason, detJSON)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return order.Order{}, fmt.Errorf("%w: order %s", apperrors.ErrNotFound, id)
		}
		return order.Order{}, err
	}
	return o, nil
}

// ApplyOverrideTx sets override_by, override_reason, status=APPROVED, credit_gate_status=OVERRIDDEN in a transaction.
func (r *OrderRepo) ApplyOverrideTx(ctx context.Context, tx pgx.Tx, tenantID, id, overrideBy, reason string, details map[string]any) (order.Order, error) {
	det := details
	if det == nil {
		det = map[string]any{}
	}
	detJSON, _ := jsonMarshal(det)
	row := tx.QueryRow(ctx, `
		UPDATE order_orders
		   SET status='APPROVED', credit_gate_status='OVERRIDDEN',
		       override_by=$3, override_reason=$4,
		       credit_check_details=$5::jsonb, updated_at=NOW()
		 WHERE tenant_id=$1::uuid AND id=$2::uuid
		 RETURNING `+orderColumns, tenantID, id, overrideBy, reason, detJSON)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return order.Order{}, fmt.Errorf("%w: order %s", apperrors.ErrNotFound, id)
		}
		return order.Order{}, err
	}
	return o, nil
}

// SetFleetDOID stores the Fledger Fleet delivery-order id and flips to DISPATCHED_TO_FLEET.
func (r *OrderRepo) SetFleetDOID(ctx context.Context, tenantID, id, fleetDOID string) (order.Order, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE order_orders
		   SET status='DISPATCHED_TO_FLEET', fledger_fleet_do_id=$3::uuid, updated_at=NOW()
		 WHERE tenant_id=$1 AND id=$2
		 RETURNING `+orderColumns, tenantID, id, fleetDOID)
	return scanOrder(row)
}