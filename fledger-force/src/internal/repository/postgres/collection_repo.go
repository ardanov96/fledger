package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-force/internal/platform/errors"
	"github.com/fledger/fledger-force/internal/domain/collection"
	"github.com/fledger/fledger-force/internal/domain/settlement"
)

type CollectionRepo struct {
	pool *pgxpool.Pool
}

func NewCollectionRepo(p *pgxpool.Pool) *CollectionRepo { return &CollectionRepo{pool: p} }

const collColumns = `id, tenant_id, COALESCE(visit_id::text, ''), sales_rep_id, store_id,
	fledger_invoice_id, receipt_number, amount, collected_at, payer_name,
	COALESCE(payer_phone, ''), wa_receipt_sent, status,
	COALESCE(settlement_id::text, ''), created_at`

func scanCollection(row pgx.Row) (collection.Collection, error) {
	var c collection.Collection
	var status string
	if err := row.Scan(
		&c.ID, &c.TenantID, &c.VisitID, &c.SalesRepID, &c.StoreID,
		&c.FledgerInvoiceID, &c.ReceiptNumber, &c.Amount, &c.CollectedAt,
		&c.PayerName, &c.PayerPhone, &c.WAReceiptSent, &status,
		&c.SettlementID, &c.CreatedAt,
	); err != nil {
		return collection.Collection{}, err
	}
	c.Status = collection.Status(status)
	if c.VisitID != nil && *c.VisitID == "" {
		c.VisitID = nil
	}
	if c.SettlementID != nil && *c.SettlementID == "" {
		c.SettlementID = nil
	}
	return c, nil
}

// Insert creates a new cash collection row.
func (r *CollectionRepo) Insert(ctx context.Context, c collection.Collection) (collection.Collection, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO force_cash_collections
		  (tenant_id, visit_id, sales_rep_id, store_id, fledger_invoice_id,
		   receipt_number, amount, collected_at, payer_name, payer_phone,
		   wa_receipt_sent, status)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8, $9, NULLIF($10,''),
		        $11, $12)
		RETURNING `+collColumns,
		c.TenantID, c.VisitID, c.SalesRepID, c.StoreID, c.FledgerInvoiceID,
		c.ReceiptNumber, c.Amount, c.CollectedAt, c.PayerName, c.PayerPhone,
		c.WAReceiptSent, string(c.Status),
	)
	out, err := scanCollection(row)
	if err != nil {
		if isUniqueViolation(err) {
			return collection.Collection{}, fmt.Errorf("%w: receipt_number already exists", apperrors.ErrConflict)
		}
		return collection.Collection{}, fmt.Errorf("insert collection: %w", err)
	}
	return out, nil
}

// ListByRep returns the rep's collections (optionally filtered by status).
func (r *CollectionRepo) ListByRep(ctx context.Context, repID, status string) ([]collection.Collection, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		rows, err = r.pool.Query(ctx, `SELECT `+collColumns+` FROM force_cash_collections WHERE sales_rep_id = $1 ORDER BY collected_at DESC`, repID)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT `+collColumns+` FROM force_cash_collections WHERE sales_rep_id = $1 AND status = $2 ORDER BY collected_at DESC`, repID, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]collection.Collection, 0, 4)
	for rows.Next() {
		v, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetByID returns one collection row.
func (r *CollectionRepo) GetByID(ctx context.Context, tenantID, id string) (collection.Collection, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+collColumns+` FROM force_cash_collections WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	c, err := scanCollection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return collection.Collection{}, fmt.Errorf("%w: collection %s", apperrors.ErrNotFound, id)
		}
		return collection.Collection{}, err
	}
	return c, nil
}

// MarkSettledToHQ flips status to SETTLED_TO_HQ and records settlement_id.
func (r *CollectionRepo) MarkSettledToHQ(ctx context.Context, settlementID string, collectionIDs []string) error {
	if len(collectionIDs) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE force_cash_collections
		   SET status = 'SETTLED_TO_HQ',
		       settlement_id = $1::uuid
		 WHERE id = ANY($2::uuid[])`,
		settlementID, collectionIDs)
	return err
}

// SettlementRepo handles EOD settlements.
type SettlementRepo struct {
	pool *pgxpool.Pool
}

func NewSettlementRepo(p *pgxpool.Pool) *SettlementRepo { return &SettlementRepo{pool: p} }

const settleColumns = `id, tenant_id, settlement_number, sales_rep_id, cashier_user_id,
	total_system_cash, total_physical_cash, discrepancy_amount, status,
	COALESCE(cashier_notes, ''), settled_at`

func scanSettle(row pgx.Row) (settlement.Settlement, error) {
	var s settlement.Settlement
	var status string
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.SettlementNumber, &s.SalesRepID, &s.CashierUserID,
		&s.TotalSystemCash, &s.TotalPhysicalCash, &s.DiscrepancyAmount, &status,
		&s.CashierNotes, &s.SettledAt,
	); err != nil {
		return settlement.Settlement{}, err
	}
	s.Status = settlement.Status(status)
	return s, nil
}

// Insert writes a new settlement row.
func (r *SettlementRepo) Insert(ctx context.Context, s settlement.Settlement) (settlement.Settlement, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO force_eod_settlements
		  (tenant_id, settlement_number, sales_rep_id, cashier_user_id,
		   total_system_cash, total_physical_cash, discrepancy_amount, status,
		   cashier_notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING `+settleColumns,
		s.TenantID, s.SettlementNumber, s.SalesRepID, s.CashierUserID,
		s.TotalSystemCash, s.TotalPhysicalCash, s.DiscrepancyAmount, string(s.Status),
		s.CashierNotes,
	)
	out, err := scanSettle(row)
	if err != nil {
		if isUniqueViolation(err) {
			return settlement.Settlement{}, fmt.Errorf("%w: settlement_number already exists", apperrors.ErrConflict)
		}
		return settlement.Settlement{}, fmt.Errorf("insert settlement: %w", err)
	}
	return out, nil
}

// SumUnsettledByRep returns the sum of collections still HELD_BY_SALES.
func (r *CollectionRepo) SumUnsettledByRep(ctx context.Context, repID string) (int64, int, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0), COUNT(*)
		  FROM force_cash_collections
		 WHERE sales_rep_id = $1 AND status = 'HELD_BY_SALES'`, repID)
	var sum int64
	var count int
	if err := row.Scan(&sum, &count); err != nil {
		return 0, 0, err
	}
	return sum, count, nil
}