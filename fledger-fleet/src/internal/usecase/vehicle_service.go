package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/domain/vehicle"
	"github.com/fledger/fledger-fleet/internal/repository/postgres"
)

// VehicleService exposes master-data operations on vehicles.
type VehicleService struct {
	repo *postgres.VehicleRepo
	pool *pgxpool.Pool
}

func NewVehicleService(repo *postgres.VehicleRepo, pool *pgxpool.Pool) *VehicleService {
	return &VehicleService{repo: repo, pool: pool}
}

// Create validates and inserts a new vehicle.
func (s *VehicleService) Create(ctx context.Context, v vehicle.Vehicle) (vehicle.Vehicle, error) {
	v.PlateNumber = strings.TrimSpace(v.PlateNumber)
	v.BrandModel = strings.TrimSpace(v.BrandModel)
	if v.PlateNumber == "" {
		return vehicle.Vehicle{}, fmt.Errorf("%w: plate_number required", apperrors.ErrInvalidInput)
	}
	if v.VehicleType == "" {
		return vehicle.Vehicle{}, fmt.Errorf("%w: vehicle_type required", apperrors.ErrInvalidInput)
	}
	if v.CapacityKg <= 0 {
		return vehicle.Vehicle{}, fmt.Errorf("%w: capacity_kg must be > 0", apperrors.ErrInvalidInput)
	}
	if v.Status == "" {
		v.Status = vehicle.StatusAvailable
	}
	if !v.Status.Valid() {
		return vehicle.Vehicle{}, fmt.Errorf("%w: invalid status %q", apperrors.ErrInvalidInput, v.Status)
	}
	return s.repo.Insert(ctx, v)
}

// List returns vehicles filtered by status (empty = all).
func (s *VehicleService) List(ctx context.Context, tenantID, status string) ([]vehicle.Vehicle, error) {
	if status != "" && !vehicle.Status(status).Valid() {
		return nil, fmt.Errorf("%w: invalid status filter %q", apperrors.ErrInvalidInput, status)
	}
	return s.repo.List(ctx, tenantID, status)
}

// Get returns one vehicle by id.
func (s *VehicleService) Get(ctx context.Context, tenantID, id string) (vehicle.Vehicle, error) {
	return s.repo.Get(ctx, tenantID, id)
}

// IsAvailable returns true if the vehicle is currently in AVAILABLE state.
func (s *VehicleService) IsAvailable(ctx context.Context, tenantID, id string) (bool, error) {
	v, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return false, err
	}
	return v.Status == vehicle.StatusAvailable, nil
}

// SetStatus is exposed for the trip-dispatch transition.
func (s *VehicleService) SetStatus(ctx context.Context, tenantID, id string, st vehicle.Status) (vehicle.Vehicle, error) {
	if !st.Valid() {
		return vehicle.Vehicle{}, fmt.Errorf("%w: invalid status %q", apperrors.ErrInvalidInput, st)
	}
	return s.repo.UpdateStatus(ctx, tenantID, id, st)
}