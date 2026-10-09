package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/domain/driver"
	"github.com/fledger/fledger-fleet/internal/repository/postgres"
)

// DriverService exposes master-data operations on drivers.
type DriverService struct {
	repo *postgres.DriverRepo
	pool *pgxpool.Pool
}

func NewDriverService(repo *postgres.DriverRepo, pool *pgxpool.Pool) *DriverService {
	return &DriverService{repo: repo, pool: pool}
}

// Create validates and inserts a new driver.
func (s *DriverService) Create(ctx context.Context, d driver.Driver) (driver.Driver, error) {
	d.FullName = strings.TrimSpace(d.FullName)
	d.PhoneNumber = strings.TrimSpace(d.PhoneNumber)
	d.LicenseNumber = strings.TrimSpace(d.LicenseNumber)
	if d.FullName == "" {
		return driver.Driver{}, fmt.Errorf("%w: full_name required", apperrors.ErrInvalidInput)
	}
	if d.PhoneNumber == "" {
		return driver.Driver{}, fmt.Errorf("%w: phone_number required", apperrors.ErrInvalidInput)
	}
	if d.LicenseNumber == "" {
		return driver.Driver{}, fmt.Errorf("%w: license_number required", apperrors.ErrInvalidInput)
	}
	if d.Status == "" {
		d.Status = driver.StatusActive
	}
	if !d.Status.Valid() {
		return driver.Driver{}, fmt.Errorf("%w: invalid status %q", apperrors.ErrInvalidInput, d.Status)
	}
	return s.repo.Insert(ctx, d)
}

// List returns drivers filtered by status (empty = all).
func (s *DriverService) List(ctx context.Context, tenantID, status string) ([]driver.Driver, error) {
	if status != "" && !driver.Status(status).Valid() {
		return nil, fmt.Errorf("%w: invalid status filter %q", apperrors.ErrInvalidInput, status)
	}
	return s.repo.List(ctx, tenantID, status)
}

// Get returns one driver.
func (s *DriverService) Get(ctx context.Context, tenantID, id string) (driver.Driver, error) {
	return s.repo.Get(ctx, tenantID, id)
}