package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/domain/delivery_order"
)

// DORepo persists Delivery Order (Surat Jalan) records and their line items.
type DORepo struct {
	pool *pgxpool.Pool
}

func NewDORepo(p *pgxpool.Pool) *DORepo { return &DORepo{pool: p} }

const doColumns = `id, tenant_id, trip_id, do_number, customer_id, customer_name,
	destination_address, destination_lat, destination_lng,
	total_items_ordered, total_items_delivered, total_items_rejected,
	nominal_ordered_cents, nominal_delivered_cents, status, fledger_invoice_id,
	created_at, updated_at`

func scanDO(row pgx.Row) (delivery_order.DeliveryOrder, error) {
	var d delivery_order.DeliveryOrder
	var status string
	var tripID, invoiceID *string
	var lat, lng *float64
	if err := row.Scan(
		&d.ID, &d.TenantID, &tripID, &d.DoNumber, &d.CustomerID, &d.CustomerName,
		&d.DestinationAddress, &lat, &lng,
		&d.TotalItemsOrdered, &d.TotalItemsDelivered, &d.TotalItemsRejected,
		&d.NominalOrderedCents, &d.NominalDeliveredCents, &status, &invoiceID,
		&d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return delivery_order.DeliveryOrder{}, err
	}
	d.Status = delivery_order.Status(status)
	d.TripID = tripID
	d.FledgerInvoiceID = invoiceID
	d.DestinationLat = lat
	d.DestinationLng = lng
	return d, nil
}

// CreateDOWithItems inserts a DO header plus its items inside a transaction.
func (r *DORepo) CreateDOWithItems(ctx context.Context, d delivery_order.DeliveryOrder, items []delivery_order.DOItem) (delivery_order.DeliveryOrder, []delivery_order.DOItem, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO fleet_delivery_orders
		  (tenant_id, do_number, customer_id, customer_name, destination_address,
		   destination_lat, destination_lng, total_items_ordered,
		   nominal_ordered_cents, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'PENDING')
		RETURNING `+doColumns,
		d.TenantID, d.DoNumber, d.CustomerID, d.CustomerName, d.DestinationAddress,
		d.DestinationLat, d.DestinationLng, d.TotalItemsOrdered,
		d.NominalOrderedCents,
	)
	created, err := scanDO(row)
	if err != nil {
		if isUniqueViolation(err) {
			return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("%w: do_number already exists", apperrors.ErrConflict)
		}
		return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("insert DO: %w", err)
	}

	outItems := make([]delivery_order.DOItem, 0, len(items))
	for _, it := range items {
		var row pgx.Row
		row = tx.QueryRow(ctx, `
			INSERT INTO fleet_do_items
			  (do_id, product_sku, product_name, qty_ordered, unit_price_cents)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, do_id, product_sku, product_name, qty_ordered, qty_delivered, qty_rejected, unit_price_cents, rejection_reason, created_at`,
			created.ID, it.ProductSKU, it.ProductName, it.QtyOrdered, it.UnitPriceCents,
		)
		var got delivery_order.DOItem
		var rej *string
		if err := row.Scan(
			&got.ID, &got.DOID, &got.ProductSKU, &got.ProductName,
			&got.QtyOrdered, &got.QtyDelivered, &got.QtyRejected,
			&got.UnitPriceCents, &rej, &got.CreatedAt,
		); err != nil {
			return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("insert item: %w", err)
		}
		got.RejectionReason = rej
		outItems = append(outItems, got)
	}

	if err := tx.Commit(ctx); err != nil {
		return delivery_order.DeliveryOrder{}, nil, fmt.Errorf("commit: %w", err)
	}
	return created, outItems, nil
}

// Get fetches one DO.
func (r *DORepo) Get(ctx context.Context, tenantID, id string) (delivery_order.DeliveryOrder, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+doColumns+` FROM fleet_delivery_orders WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	d, err := scanDO(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return delivery_order.DeliveryOrder{}, fmt.Errorf("%w: DO %s", apperrors.ErrNotFound, id)
		}
		return delivery_order.DeliveryOrder{}, err
	}
	return d, nil
}

// List returns DOs filtered by status (empty = all).
func (r *DORepo) List(ctx context.Context, tenantID string, status string) ([]delivery_order.DeliveryOrder, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		rows, err = r.pool.Query(ctx,
			`SELECT `+doColumns+` FROM fleet_delivery_orders WHERE tenant_id = $1 ORDER BY created_at DESC`,
			tenantID,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT `+doColumns+` FROM fleet_delivery_orders WHERE tenant_id = $1 AND status = $2 ORDER BY created_at DESC`,
			tenantID, status,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list DOs: %w", err)
	}
	defer rows.Close()
	out := make([]delivery_order.DeliveryOrder, 0, 8)
	for rows.Next() {
		d, err := scanDO(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListItems returns the SKU lines for one DO.
func (r *DORepo) ListItems(ctx context.Context, doID string) ([]delivery_order.DOItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, do_id, product_sku, product_name, qty_ordered,
		       qty_delivered, qty_rejected, unit_price_cents, rejection_reason, created_at
		  FROM fleet_do_items
		 WHERE do_id = $1
		 ORDER BY created_at`,
		doID,
	)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()
	out := make([]delivery_order.DOItem, 0, 4)
	for rows.Next() {
		var it delivery_order.DOItem
		var rej *string
		if err := rows.Scan(
			&it.ID, &it.DOID, &it.ProductSKU, &it.ProductName, &it.QtyOrdered,
			&it.QtyDelivered, &it.QtyRejected, &it.UnitPriceCents, &rej, &it.CreatedAt,
		); err != nil {
			return nil, err
		}
		it.RejectionReason = rej
		out = append(out, it)
	}
	return out, rows.Err()
}

// ApplyPODResult updates items with the result of a POD submission and
// finalises the DO header. Returns the updated DO + items.
func (r *DORepo) ApplyPODResult(
	ctx context.Context,
	tenantID, doID string,
	status delivery_order.Status,
	totalDelivered, totalRejected int,
	nominalDelivered int64,
	itemResults map[string]struct {
		QtyDelivered    int
		QtyRejected     int
		RejectionReason string
	},
	fledgerInvoiceID *string,
) (delivery_order.DeliveryOrder, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return delivery_order.DeliveryOrder{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for sku, r := range itemResults {
		var rej *string
		if r.RejectionReason != "" {
			s := r.RejectionReason
			rej = &s
		}
		if _, err := tx.Exec(ctx, `
			UPDATE fleet_do_items
			   SET qty_delivered = $3, qty_rejected = $4, rejection_reason = $5
			 WHERE do_id = $1 AND product_sku = $2`,
			doID, sku, r.QtyDelivered, r.QtyRejected, rej,
		); err != nil {
			return delivery_order.DeliveryOrder{}, fmt.Errorf("update item %s: %w", sku, err)
		}
	}

	row := tx.QueryRow(ctx, `
		UPDATE fleet_delivery_orders
		   SET status = $3,
		       total_items_delivered = $4,
		       total_items_rejected = $5,
		       nominal_delivered_cents = $6,
		       fledger_invoice_id = $7,
		       updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
		 RETURNING `+doColumns,
		tenantID, doID, string(status), totalDelivered, totalRejected, nominalDelivered, fledgerInvoiceID,
	)
	updated, err := scanDO(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return delivery_order.DeliveryOrder{}, fmt.Errorf("%w: DO %s", apperrors.ErrNotFound, doID)
		}
		return delivery_order.DeliveryOrder{}, fmt.Errorf("update DO: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return delivery_order.DeliveryOrder{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}

// UpdateFledgerInvoiceID updates the fledger_invoice_id on a DO (used by the outbox worker).
func (r *DORepo) UpdateFledgerInvoiceID(ctx context.Context, tenantID, doID, invoiceID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE fleet_delivery_orders
		   SET fledger_invoice_id = $3, updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2`,
		tenantID, doID, invoiceID,
	)
	if err != nil {
		return fmt.Errorf("update fledger invoice id: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: DO %s", apperrors.ErrNotFound, doID)
	}
	return nil
}