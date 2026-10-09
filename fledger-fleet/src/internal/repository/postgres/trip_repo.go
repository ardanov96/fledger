package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/domain/trip"
)

// TripRepo persists trip (surat tugas perjalanan) records.
type TripRepo struct {
	pool *pgxpool.Pool
}

func NewTripRepo(p *pgxpool.Pool) *TripRepo { return &TripRepo{pool: p} }

const tripColumns = `id, tenant_id, trip_number, vehicle_id, driver_id,
	departure_time, completed_time, status, total_stops, total_delivered, notes,
	created_at, updated_at`

func scanTrip(row pgx.Row) (trip.Trip, error) {
	var t trip.Trip
	var status string
	if err := row.Scan(
		&t.ID, &t.TenantID, &t.TripNumber, &t.VehicleID, &t.DriverID,
		&t.DepartureTime, &t.CompletedTime, &status, &t.TotalStops, &t.TotalDelivered, &t.Notes,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return trip.Trip{}, err
	}
	t.Status = trip.Status(status)
	return t, nil
}

// Insert creates a trip in DRAFT with N=total_stops; updates DO rows to set trip_id.
func (r *TripRepo) Insert(ctx context.Context, t trip.Trip, doIDs []string) (trip.Trip, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return trip.Trip{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO fleet_trips (tenant_id, trip_number, vehicle_id, driver_id, status, total_stops, notes)
		VALUES ($1, $2, $3, $4, 'DRAFT', $5, $6)
		RETURNING `+tripColumns,
		t.TenantID, t.TripNumber, t.VehicleID, t.DriverID, t.TotalStops, t.Notes,
	)
	out, err := scanTrip(row)
	if err != nil {
		if isUniqueViolation(err) {
			return trip.Trip{}, fmt.Errorf("%w: trip_number already exists", apperrors.ErrConflict)
		}
		return trip.Trip{}, fmt.Errorf("insert trip: %w", err)
	}

	if len(doIDs) > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE fleet_delivery_orders SET trip_id = $1, updated_at = NOW()
			 WHERE tenant_id = $2 AND id = ANY($3::uuid[])`,
			out.ID, out.TenantID, doIDs,
		); err != nil {
			return trip.Trip{}, fmt.Errorf("link DO to trip: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return trip.Trip{}, fmt.Errorf("commit: %w", err)
	}
	return out, nil
}

// Get fetches one trip.
func (r *TripRepo) Get(ctx context.Context, tenantID, id string) (trip.Trip, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+tripColumns+` FROM fleet_trips WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	t, err := scanTrip(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return trip.Trip{}, fmt.Errorf("%w: trip %s", apperrors.ErrNotFound, id)
		}
		return trip.Trip{}, err
	}
	return t, nil
}

// List returns trips filtered by status (empty = all).
func (r *TripRepo) List(ctx context.Context, tenantID string, status string) ([]trip.Trip, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if status == "" {
		rows, err = r.pool.Query(ctx,
			`SELECT `+tripColumns+` FROM fleet_trips WHERE tenant_id = $1 ORDER BY created_at DESC`,
			tenantID,
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT `+tripColumns+` FROM fleet_trips WHERE tenant_id = $1 AND status = $2 ORDER BY created_at DESC`,
			tenantID, status,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list trips: %w", err)
	}
	defer rows.Close()
	out := make([]trip.Trip, 0, 8)
	for rows.Next() {
		t, err := scanTrip(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateStatus moves a trip to the new status (used by dispatch/complete).
func (r *TripRepo) UpdateStatus(ctx context.Context, tenantID, id string, newStatus trip.Status) (trip.Trip, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE fleet_trips SET status = $3, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+tripColumns,
		tenantID, id, string(newStatus),
	)
	t, err := scanTrip(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return trip.Trip{}, fmt.Errorf("%w: trip %s", apperrors.ErrNotFound, id)
		}
		return trip.Trip{}, err
	}
	return t, nil
}

// Dispatch moves DRAFT → IN_TRANSIT, sets departure_time, and cascades
// OUT_FOR_DELIVERY to all linked DOs.
func (r *TripRepo) Dispatch(ctx context.Context, tenantID, id string) (trip.Trip, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return trip.Trip{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		UPDATE fleet_trips
		   SET status = 'IN_TRANSIT',
		       departure_time = NOW(),
		       updated_at = NOW()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'DRAFT'
		 RETURNING `+tripColumns,
		tenantID, id,
	)
	t, err := scanTrip(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return trip.Trip{}, fmt.Errorf("%w: trip %s not in DRAFT", apperrors.ErrConflict, id)
		}
		return trip.Trip{}, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE fleet_delivery_orders
		    SET status = 'OUT_FOR_DELIVERY', updated_at = NOW()
		  WHERE tenant_id = $1 AND trip_id = $2 AND status IN ('PENDING','LOADED')`,
		tenantID, id,
	); err != nil {
		return trip.Trip{}, fmt.Errorf("cascade DO: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return trip.Trip{}, fmt.Errorf("commit: %w", err)
	}
	return t, nil
}