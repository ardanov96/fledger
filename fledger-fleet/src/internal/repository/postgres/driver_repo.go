package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/domain/driver"
)

// DriverRepo persists fleet drivers.
type DriverRepo struct {
	pool *pgxpool.Pool
}

func NewDriverRepo(p *pgxpool.Pool) *DriverRepo { return &DriverRepo{pool: p} }

const driverColumns = `id, tenant_id, full_name, phone_number, license_number,
	status, created_at, updated_at`

func scanDriver(row pgx.Row) (driver.Driver, error) {
	var d driver.Driver
	var status string
	if err := row.Scan(
		&d.ID, &d.TenantID, &d.FullName, &d.PhoneNumber, &d.LicenseNumber,
		&status, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return driver.Driver{}, err
	}
	d.Status = driver.Status(status)
	return d, nil
}

// Insert persists a new driver.
func (r *DriverRepo) Insert(ctx context.Context, d driver.Driver) (driver.Driver, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO fleet_drivers (tenant_id, full_name, phone_number, license_number, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+driverColumns,
		d.TenantID, d.FullName, d.PhoneNumber, d.LicenseNumber, string(d.Status),
	)
	out, err := scanDriver(row)
	if err != nil {
		if isUniqueViolation(err) {
			return driver.Driver{}, fmt.Errorf("%w: phone_number already exists", apperrors.ErrConflict)
		}
		return driver.Driver{}, fmt.Errorf("insert driver: %w", err)
	}
	return out, nil
}

// Get fetches one driver by id.
func (r *DriverRepo) Get(ctx context.Context, tenantID, id string) (driver.Driver, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+driverColumns+` FROM fleet_drivers WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	d, err := scanDriver(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return driver.Driver{}, fmt.Errorf("%w: driver %s", apperrors.ErrNotFound, id)
		}
		return driver.Driver{}, err
	}
	return d, nil
}

// List returns drivers filtered by status (empty = all).
func (r *DriverRepo) List(ctx context.Context, tenantID string, status string) ([]driver.Driver, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		rows, err = r.pool.Query(ctx,
			`SELECT `+driverColumns+` FROM fleet_drivers WHERE tenant_id = $1 ORDER BY full_name`,
			tenantID,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT `+driverColumns+` FROM fleet_drivers WHERE tenant_id = $1 AND status = $2 ORDER BY full_name`,
			tenantID, status,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list drivers: %w", err)
	}
	defer rows.Close()
	out := make([]driver.Driver, 0, 8)
	for rows.Next() {
		d, err := scanDriver(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// UpdateStatus updates the driver's status.
func (r *DriverRepo) UpdateStatus(ctx context.Context, tenantID, id string, status driver.Status) (driver.Driver, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE fleet_drivers
		   SET status = $3, updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
		 RETURNING `+driverColumns,
		tenantID, id, string(status),
	)
	d, err := scanDriver(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return driver.Driver{}, fmt.Errorf("%w: driver %s", apperrors.ErrNotFound, id)
		}
		return driver.Driver{}, err
	}
	return d, nil
}