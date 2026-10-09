package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/domain/driver"
	"github.com/fledger/fledger-fleet/internal/domain/trip"
	"github.com/fledger/fledger-fleet/internal/domain/vehicle"
	"github.com/fledger/fledger-fleet/internal/repository/postgres"
)

// KgPerBox is the estimated weight in kilograms per box/item.
const KgPerBox = 10.0

// EstimateWeightKg returns the total estimated weight for a given number of boxes.
func EstimateWeightKg(boxes int) float64 {
	return float64(boxes) * KgPerBox
}

// CheckCapacity verifies that estimated weight does not exceed vehicle capacity.
func CheckCapacity(capacityKg float64, totalBoxes int) error {
	est := EstimateWeightKg(totalBoxes)
	if est > capacityKg {
		return fmt.Errorf("%w: total estimated weight %.1fkg exceeds vehicle capacity %.1fkg",
			apperrors.ErrConflict, est, capacityKg)
	}
	return nil
}

// TripService orchestrates trip creation and dispatching.
type TripService struct {
	trips    *postgres.TripRepo
	vehicles *postgres.VehicleRepo
	drivers  *postgres.DriverRepo
	pool     *pgxpool.Pool
}

func NewTripService(trips *postgres.TripRepo, vehicles *postgres.VehicleRepo, drivers *postgres.DriverRepo, pool *pgxpool.Pool) *TripService {
	return &TripService{trips: trips, vehicles: vehicles, drivers: drivers, pool: pool}
}

// CreateTripInput bundles the input of POST /v1/fleet/trips.
type CreateTripInput struct {
	TenantID   string
	TripNumber string
	VehicleID  string
	DriverID   string
	DoIDs      []string
	Notes      string
}

// Create validates inputs, ensures the vehicle + driver are available, and
// inserts the trip in DRAFT. The Sprint 2 unit-testable invariant lives here:
// total trip weight (kg) must not exceed vehicle.capacity_kg.
func (s *TripService) Create(ctx context.Context, in CreateTripInput) (trip.Trip, error) {
	in.TripNumber = strings.TrimSpace(in.TripNumber)
	if in.TripNumber == "" {
		return trip.Trip{}, fmt.Errorf("%w: trip_number required", apperrors.ErrInvalidInput)
	}
	if in.VehicleID == "" || in.DriverID == "" {
		return trip.Trip{}, fmt.Errorf("%w: vehicle_id and driver_id required", apperrors.ErrInvalidInput)
	}
	if len(in.DoIDs) == 0 {
		return trip.Trip{}, fmt.Errorf("%w: at least one DO required", apperrors.ErrInvalidInput)
	}

	v, err := s.vehicles.Get(ctx, in.TenantID, in.VehicleID)
	if err != nil {
		return trip.Trip{}, err
	}
	if v.Status != vehicle.StatusAvailable {
		return trip.Trip{}, fmt.Errorf("%w: vehicle is %s, cannot dispatch new trip", apperrors.ErrConflict, v.Status)
	}
	d, err := s.drivers.Get(ctx, in.TenantID, in.DriverID)
	if err != nil {
		return trip.Trip{}, err
	}
	if d.Status != driver.StatusActive {
		return trip.Trip{}, fmt.Errorf("%w: driver is %s, cannot assign to new trip", apperrors.ErrConflict, d.Status)
	}

	totalBoxes, err := s.countBoxes(ctx, in.TenantID, in.DoIDs)
	if err != nil {
		return trip.Trip{}, err
	}
	if err := CheckCapacity(v.CapacityKg, totalBoxes); err != nil {
		return trip.Trip{}, err
	}

	t := trip.Trip{
		TenantID:   in.TenantID,
		TripNumber: in.TripNumber,
		VehicleID:  in.VehicleID,
		DriverID:   in.DriverID,
		TotalStops: len(in.DoIDs),
		Notes:      in.Notes,
	}
	return s.trips.Insert(ctx, t, in.DoIDs)
}

// Dispatch flips trip DRAFT → IN_TRANSIT and cascades OUT_FOR_DELIVERY on DOs.
// It also flips vehicle.status to ON_TRIP and driver.status to ON_DUTY.
func (s *TripService) Dispatch(ctx context.Context, tenantID, id string) (trip.Trip, error) {
	t, err := s.trips.Dispatch(ctx, tenantID, id)
	if err != nil {
		return trip.Trip{}, err
	}
	if _, err := s.vehicles.UpdateStatus(ctx, tenantID, t.VehicleID, vehicle.StatusOnTrip); err != nil {
		return trip.Trip{}, fmt.Errorf("update vehicle: %w", err)
	}
	if _, err := s.drivers.UpdateStatus(ctx, tenantID, t.DriverID, driver.StatusOnDuty); err != nil {
		return trip.Trip{}, fmt.Errorf("update driver: %w", err)
	}
	return t, nil
}

// Get returns one trip.
func (s *TripService) Get(ctx context.Context, tenantID, id string) (trip.Trip, error) {
	return s.trips.Get(ctx, tenantID, id)
}

// List returns trips filtered by status.
func (s *TripService) List(ctx context.Context, tenantID, status string) ([]trip.Trip, error) {
	if status != "" && !trip.Status(status).Valid() {
		return nil, fmt.Errorf("%w: invalid status filter %q", apperrors.ErrInvalidInput, status)
	}
	return s.trips.List(ctx, tenantID, status)
}

// ListToday returns trips created today or currently in transit/dispatched.
func (s *TripService) ListToday(ctx context.Context, tenantID string) ([]trip.Trip, error) {
	return s.trips.ListToday(ctx, tenantID)
}

// countBoxes counts qty_ordered across all items of the supplied DO ids. It
// is the proxy for "total boxes" used by the capacity check.
func (s *TripService) countBoxes(ctx context.Context, tenantID string, doIDs []string) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(SUM(i.qty_ordered), 0)
		  FROM fleet_do_items i
		  JOIN fleet_delivery_orders d ON d.id = i.do_id
		 WHERE d.tenant_id = $1 AND d.id = ANY($2::uuid[])`, tenantID, doIDs)
	if err != nil {
		return 0, fmt.Errorf("count boxes: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, nil
	}
	var total int
	if err := rows.Scan(&total); err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return total, nil
}