package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-force/internal/platform/errors"
	"github.com/fledger/fledger-force/internal/domain/salesrep"
)

type RepRepo struct {
	pool *pgxpool.Pool
}

func NewRepRepo(p *pgxpool.Pool) *RepRepo { return &RepRepo{pool: p} }

const repColumns = `id, tenant_id, employee_code, name, phone, role,
	fledger_wallet_account_id, current_cash_held, max_cash_limit, status,
	created_at, updated_at`

func scanRep(row pgx.Row) (salesrep.Rep, error) {
	var r salesrep.Rep
	var role, status string
	if err := row.Scan(
		&r.ID, &r.TenantID, &r.EmployeeCode, &r.Name, &r.Phone, &role,
		&r.FledgerWalletAccountID, &r.CurrentCashHeld, &r.MaxCashLimit, &status,
		&r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return salesrep.Rep{}, err
	}
	r.Role = salesrep.Role(role)
	r.Status = salesrep.Status(status)
	return r, nil
}

func (r *RepRepo) Insert(ctx context.Context, rep salesrep.Rep) (salesrep.Rep, error) {
	if rep.Status == "" {
		rep.Status = salesrep.StatusActive
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO force_sales_reps
		  (tenant_id, employee_code, name, phone, role, fledger_wallet_account_id,
		   current_cash_held, max_cash_limit, status)
		VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8)
		RETURNING `+repColumns,
		rep.TenantID, rep.EmployeeCode, rep.Name, rep.Phone, string(rep.Role),
		rep.FledgerWalletAccountID, rep.MaxCashLimit, string(rep.Status),
	)
	out, err := scanRep(row)
	if err != nil {
		if isUniqueViolation(err) {
			return salesrep.Rep{}, fmt.Errorf("%w: employee_code already exists", apperrors.ErrConflict)
		}
		return salesrep.Rep{}, fmt.Errorf("insert rep: %w", err)
	}
	return out, nil
}

func (r *RepRepo) Get(ctx context.Context, tenantID, id string) (salesrep.Rep, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+repColumns+` FROM force_sales_reps WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	rep, err := scanRep(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return salesrep.Rep{}, fmt.Errorf("%w: rep %s", apperrors.ErrNotFound, id)
		}
		return salesrep.Rep{}, err
	}
	return rep, nil
}

func (r *RepRepo) List(ctx context.Context, tenantID, status string) ([]salesrep.Rep, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		rows, err = r.pool.Query(ctx, `SELECT `+repColumns+` FROM force_sales_reps WHERE tenant_id = $1 ORDER BY employee_code`, tenantID)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT `+repColumns+` FROM force_sales_reps WHERE tenant_id = $1 AND status = $2 ORDER BY employee_code`, tenantID, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesrep.Rep, 0, 4)
	for rows.Next() {
		v, err := scanRep(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// IncrementCashHeld atomically adds delta to the rep's current_cash_held
// (can be negative for EOD reset). Returns the new rep row.
func (r *RepRepo) IncrementCashHeld(ctx context.Context, tenantID, id string, delta int64) (salesrep.Rep, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE force_sales_reps
		   SET current_cash_held = current_cash_held + $3,
		       updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
		 RETURNING `+repColumns, tenantID, id, delta)
	rep, err := scanRep(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return salesrep.Rep{}, fmt.Errorf("%w: rep %s", apperrors.ErrNotFound, id)
		}
		return salesrep.Rep{}, err
	}
	return rep, nil
}

// SetStatus moves a rep to a new status (e.g. SETTLEMENT_LOCKED after EOD mismatch).
func (r *RepRepo) SetStatus(ctx context.Context, tenantID, id string, status salesrep.Status) (salesrep.Rep, error) {
	if !status.Valid() {
		return salesrep.Rep{}, fmt.Errorf("%w: invalid status %q", apperrors.ErrInvalidInput, status)
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE force_sales_reps
		   SET status = $3, updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
		 RETURNING `+repColumns, tenantID, id, string(status))
	rep, err := scanRep(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return salesrep.Rep{}, fmt.Errorf("%w: rep %s", apperrors.ErrNotFound, id)
		}
		return salesrep.Rep{}, err
	}
	return rep, nil
}

// MatchWalletHint returns a stable per-bank wallet hint like
// "ACC_SALES_WALLET_BUDI" derived from the rep name. Used by the outbox
// worker to pick the destination account.
func MatchWalletHint(name string) string {
	cleaned := strings.ToUpper(strings.ReplaceAll(name, " ", "_"))
	cleaned = strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return -1
	}, cleaned)
	if len(cleaned) > 24 {
		cleaned = cleaned[:24]
	}
	return "ACC_SALES_WALLET_" + cleaned
}