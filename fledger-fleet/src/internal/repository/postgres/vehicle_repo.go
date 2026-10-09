package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/domain/vehicle"
)

// VehicleRepo persists fleet vehicles.
type VehicleRepo struct {
	pool *pgxpool.Pool
}

func NewVehicleRepo(p *pgxpool.Pool) *VehicleRepo { return &VehicleRepo{pool: p} }

const vehicleColumns = `id, tenant_id, plate_number, vehicle_type, brand_model,
	capacity_kg, status, created_at, updated_at`

func scanVehicle(row pgx.Row) (vehicle.Vehicle, error) {
	var v vehicle.Vehicle
	var status string
	if err := row.Scan(
		&v.ID, &v.TenantID, &v.PlateNumber, &v.VehicleType, &v.BrandModel,
		&v.CapacityKg, &status, &v.CreatedAt, &v.UpdatedAt,
	); err != nil {
		return vehicle.Vehicle{}, err
	}
	v.Status = vehicle.Status(status)
	return v, nil
}

// Insert persists a new vehicle. Returns ErrConflict on plate_number collision.
func (r *VehicleRepo) Insert(ctx context.Context, v vehicle.Vehicle) (vehicle.Vehicle, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO fleet_vehicles (tenant_id, plate_number, vehicle_type, brand_model, capacity_kg, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+vehicleColumns,
		v.TenantID, v.PlateNumber, v.VehicleType, v.BrandModel, v.CapacityKg, string(v.Status),
	)
	out, err := scanVehicle(row)
	if err != nil {
		if isUniqueViolation(err) {
			return vehicle.Vehicle{}, fmt.Errorf("%w: plate_number already exists", apperrors.ErrConflict)
		}
		return vehicle.Vehicle{}, fmt.Errorf("insert vehicle: %w", err)
	}
	return out, nil
}

// Get fetches one vehicle by id (tenant-scoped).
func (r *VehicleRepo) Get(ctx context.Context, tenantID, id string) (vehicle.Vehicle, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+vehicleColumns+` FROM fleet_vehicles WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	v, err := scanVehicle(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return vehicle.Vehicle{}, fmt.Errorf("%w: vehicle %s", apperrors.ErrNotFound, id)
		}
		return vehicle.Vehicle{}, err
	}
	return v, nil
}

// List returns vehicles filtered by status (empty = all).
func (r *VehicleRepo) List(ctx context.Context, tenantID string, status string) ([]vehicle.Vehicle, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		rows, err = r.pool.Query(ctx,
			`SELECT `+vehicleColumns+` FROM fleet_vehicles WHERE tenant_id = $1 ORDER BY plate_number`,
			tenantID,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT `+vehicleColumns+` FROM fleet_vehicles WHERE tenant_id = $1 AND status = $2 ORDER BY plate_number`,
			tenantID, status,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list vehicles: %w", err)
	}
	defer rows.Close()
	out := make([]vehicle.Vehicle, 0, 8)
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// UpdateStatus moves a vehicle to the given status.
func (r *VehicleRepo) UpdateStatus(ctx context.Context, tenantID, id string, status vehicle.Status) (vehicle.Vehicle, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE fleet_vehicles SET status = $3, updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
		 RETURNING `+vehicleColumns,
		tenantID, id, string(status),
	)
	v, err := scanVehicle(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return vehicle.Vehicle{}, fmt.Errorf("%w: vehicle %s", apperrors.ErrNotFound, id)
		}
		return vehicle.Vehicle{}, err
	}
	return v, nil
}