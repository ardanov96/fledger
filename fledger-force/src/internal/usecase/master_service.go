package usecase

import (
	"context"
	"fmt"
	"strings"

	apperrors "github.com/fledger/fledger-force/internal/platform/errors"
	"github.com/fledger/fledger-force/internal/domain/salesrep"
	"github.com/fledger/fledger-force/internal/domain/store"
	"github.com/fledger/fledger-force/internal/repository/postgres"
)

// RepService handles master-data CRUD for sales reps.
type RepService struct {
	reps *postgres.RepRepo
}

func NewRepService(r *postgres.RepRepo) *RepService { return &RepService{reps: r} }

type CreateRepInput struct {
	TenantID               string
	EmployeeCode           string
	Name                   string
	Phone                  string
	Role                   salesrep.Role
	FledgerWalletAccountID string
	MaxCashLimit           int64
}

func (s *RepService) Create(ctx context.Context, in CreateRepInput) (salesrep.Rep, error) {
	if strings.TrimSpace(in.EmployeeCode) == "" {
		return salesrep.Rep{}, fmt.Errorf("%w: employee_code required", apperrors.ErrInvalidInput)
	}
	if strings.TrimSpace(in.Name) == "" {
		return salesrep.Rep{}, fmt.Errorf("%w: name required", apperrors.ErrInvalidInput)
	}
	if strings.TrimSpace(in.Phone) == "" {
		return salesrep.Rep{}, fmt.Errorf("%w: phone required", apperrors.ErrInvalidInput)
	}
	if in.Role == "" {
		in.Role = salesrep.RoleCanvasser
	}
	if !in.Role.Valid() {
		return salesrep.Rep{}, fmt.Errorf("%w: invalid role %q", apperrors.ErrInvalidInput, in.Role)
	}
	if in.MaxCashLimit <= 0 {
		in.MaxCashLimit = 50_000_000
	}
	// If no wallet account id is supplied, derive a stable one from the rep name.
	if strings.TrimSpace(in.FledgerWalletAccountID) == "" {
		in.FledgerWalletAccountID = postgres.MatchWalletHint(in.Name)
	}
	return s.reps.Insert(ctx, salesrep.Rep{
		TenantID:               in.TenantID,
		EmployeeCode:           strings.TrimSpace(in.EmployeeCode),
		Name:                   strings.TrimSpace(in.Name),
		Phone:                  strings.TrimSpace(in.Phone),
		Role:                   in.Role,
		FledgerWalletAccountID: in.FledgerWalletAccountID,
		MaxCashLimit:           in.MaxCashLimit,
		Status:                 salesrep.StatusActive,
	})
}

func (s *RepService) Get(ctx context.Context, tenantID, id string) (salesrep.Rep, error) {
	return s.reps.Get(ctx, tenantID, id)
}

func (s *RepService) List(ctx context.Context, tenantID, status string) ([]salesrep.Rep, error) {
	if status != "" && !salesrep.Status(status).Valid() {
		return nil, fmt.Errorf("%w: invalid status filter %q", apperrors.ErrInvalidInput, status)
	}
	return s.reps.List(ctx, tenantID, status)
}

// StoreService handles master-data CRUD for stores.
type StoreService struct {
	stores *postgres.StoreRepo
}

func NewStoreService(s *postgres.StoreRepo) *StoreService { return &StoreService{stores: s} }

type CreateStoreInput struct {
	TenantID             string
	StoreCode            string
	Name                 string
	OwnerName            string
	Phone                string
	Address              string
	Latitude             float64
	Longitude            float64
	GeofenceRadiusMeters int
	Tier                 store.Tier
	CreditLimit          int64
}

func (s *StoreService) Create(ctx context.Context, in CreateStoreInput) (store.Store, error) {
	if strings.TrimSpace(in.StoreCode) == "" {
		return store.Store{}, fmt.Errorf("%w: store_code required", apperrors.ErrInvalidInput)
	}
	if strings.TrimSpace(in.Name) == "" {
		return store.Store{}, fmt.Errorf("%w: name required", apperrors.ErrInvalidInput)
	}
	if strings.TrimSpace(in.Address) == "" {
		return store.Store{}, fmt.Errorf("%w: address required", apperrors.ErrInvalidInput)
	}
	if in.Tier == "" {
		in.Tier = store.TierRetail
	}
	if !in.Tier.Valid() {
		return store.Store{}, fmt.Errorf("%w: invalid tier %q", apperrors.ErrInvalidInput, in.Tier)
	}
	if in.GeofenceRadiusMeters <= 0 {
		in.GeofenceRadiusMeters = 100
	}
	if in.Latitude == 0 && in.Longitude == 0 {
		return store.Store{}, fmt.Errorf("%w: latitude/longitude required", apperrors.ErrInvalidInput)
	}
	return s.stores.Insert(ctx, store.Store{
		TenantID:             in.TenantID,
		StoreCode:            strings.TrimSpace(in.StoreCode),
		Name:                 strings.TrimSpace(in.Name),
		OwnerName:            strings.TrimSpace(in.OwnerName),
		Phone:                strings.TrimSpace(in.Phone),
		Address:              strings.TrimSpace(in.Address),
		Latitude:             in.Latitude,
		Longitude:            in.Longitude,
		GeofenceRadiusMeters: in.GeofenceRadiusMeters,
		Tier:                 in.Tier,
		CreditLimit:          in.CreditLimit,
	})
}

func (s *StoreService) Get(ctx context.Context, tenantID, id string) (store.Store, error) {
	return s.stores.Get(ctx, tenantID, id)
}

func (s *StoreService) List(ctx context.Context, tenantID, tier string) ([]store.Store, error) {
	if tier != "" && !store.Tier(tier).Valid() {
		return nil, fmt.Errorf("%w: invalid tier filter %q", apperrors.ErrInvalidInput, tier)
	}
	return s.stores.List(ctx, tenantID, tier)
}