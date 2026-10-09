// Package usecase — visit service (Sprint 2: GPS check-in + geofencing).
package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/fledger/fledger-force/internal/platform/errors"
	"github.com/fledger/fledger-force/internal/domain/audit"
	"github.com/fledger/fledger-force/internal/domain/beatplan"
	"github.com/fledger/fledger-force/internal/domain/store"
	"github.com/fledger/fledger-force/internal/domain/visit"
	"github.com/fledger/fledger-force/internal/platform/geo"
	"github.com/fledger/fledger-force/internal/repository/postgres"
)

// VisitService handles beat plans, store lookups, and GPS check-ins.
type VisitService struct {
	visits *postgres.VisitRepo
	stores *postgres.StoreRepo
	audit  *postgres.AuditRepo
}

func NewVisitService(v *postgres.VisitRepo, s *postgres.StoreRepo, a *postgres.AuditRepo) *VisitService {
	return &VisitService{visits: v, stores: s, audit: a}
}

// CreateBeatPlanInput is the input for creating a beat plan.
type CreateBeatPlanInput struct {
	TenantID          string
	PlanNumber        string
	SalesRepID        string
	PlanDate          time.Time
	Territory         string
	TargetStoresCount int
	Notes             string
}

// CreateBeatPlan persists a new beat plan row.
func (s *VisitService) CreateBeatPlan(ctx context.Context, in CreateBeatPlanInput) (beatplan.Plan, error) {
	if _, err := uuid.Parse(in.SalesRepID); err != nil {
		return beatplan.Plan{}, fmt.Errorf("%w: sales_rep_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if strings.TrimSpace(in.Territory) == "" {
		return beatplan.Plan{}, fmt.Errorf("%w: territory required", apperrors.ErrInvalidInput)
	}
	if in.PlanDate.IsZero() {
		return beatplan.Plan{}, fmt.Errorf("%w: plan_date required", apperrors.ErrInvalidInput)
	}
	if in.PlanNumber == "" {
		in.PlanNumber = GenerateBeatPlanNumber(time.Now().UTC())
	}
	plan, err := s.visits.InsertBeatPlan(ctx, beatplanFrom(in))
	if err != nil {
		return beatplan.Plan{}, err
	}
	return plan, nil
}

// TodayBeatPlans returns the rep's beat plans for today.
func (s *VisitService) TodayBeatPlans(ctx context.Context, tenantID, repID string) ([]beatplan.Plan, error) {
	if _, err := uuid.Parse(repID); err != nil {
		return nil, fmt.Errorf("%w: sales_rep_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	plans, err := s.visits.ListBeatPlansForRepDate(ctx, tenantID, repID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return plans, nil
}

// CheckInInput is the input of POST /v1/force/visits/check-in.
type CheckInInput struct {
	TenantID  string
	BeatPlanID string
	StoreID   string
	SalesRepID string
	Latitude  float64
	Longitude float64
	VisitType visit.VisitType
	ActorID   string
	IPAddress string
}

// CheckInResult is the response shape (matches API-SPECIFICATION.md §2.1).
type CheckInResult struct {
	ID               string    `json:"id"`
	StoreName        string    `json:"store_name"`
	DistanceMeters   int       `json:"distance_meters"`
	GeofenceVerified bool      `json:"geofence_verified"`
	CheckInAt        time.Time `json:"check_in_at"`
	Status           string    `json:"status"`
}

// CheckIn validates the GPS position and writes a new visit row. If the
// position is outside the geofence radius the visit is still recorded but
// flagged `geofence_verified=false` and an audit row is written.
func (s *VisitService) CheckIn(ctx context.Context, in CheckInInput) (*CheckInResult, error) {
	if _, err := uuid.Parse(in.BeatPlanID); err != nil {
		return nil, fmt.Errorf("%w: beat_plan_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(in.StoreID); err != nil {
		return nil, fmt.Errorf("%w: store_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(in.SalesRepID); err != nil {
		return nil, fmt.Errorf("%w: sales_rep_id must be a valid UUID", apperrors.ErrInvalidInput)
	}
	if in.Latitude < -90 || in.Latitude > 90 {
		return nil, fmt.Errorf("%w: latitude out of range", apperrors.ErrInvalidInput)
	}
	if in.Longitude < -180 || in.Longitude > 180 {
		return nil, fmt.Errorf("%w: longitude out of range", apperrors.ErrInvalidInput)
	}
	if in.VisitType == "" {
		in.VisitType = visit.VisitTakingOrderAndCollection
	}
	if !in.VisitType.Valid() {
		return nil, fmt.Errorf("%w: invalid visit_type %q", apperrors.ErrInvalidInput, in.VisitType)
	}

	st, err := s.stores.Get(ctx, in.TenantID, in.StoreID)
	if err != nil {
		return nil, err
	}

	ok, distance := geo.WithinRadius(in.Latitude, in.Longitude, st.Latitude, st.Longitude, st.GeofenceRadiusMeters)

	v := visit.Visit{
		TenantID:         in.TenantID,
		BeatPlanID:       in.BeatPlanID,
		StoreID:          in.StoreID,
		SalesRepID:       in.SalesRepID,
		CheckInAt:        time.Now().UTC(),
		CheckInLat:       in.Latitude,
		CheckInLong:      in.Longitude,
		DistanceMeters:   int(distance + 0.5),
		GeofenceVerified: ok,
		VisitType:        in.VisitType,
		Status:           visit.StatusCheckedIn,
	}
	saved, err := s.visits.InsertVisit(ctx, v)
	if err != nil {
		return nil, err
	}

	if !ok {
		_ = s.audit.Append(ctx, audit.Log{
			TenantID:     in.TenantID,
			ActorID:      in.ActorID,
			ActorRole:    "salesman",
			Action:       audit.ActionGeofenceOverride,
			ResourceType: "store_visit",
			ResourceID:   saved.ID,
			Details: map[string]any{
				"store_id":         in.StoreID,
				"store_name":       st.Name,
				"distance_meters":  saved.DistanceMeters,
				"radius_meters":    st.GeofenceRadiusMeters,
				"check_in_lat":     in.Latitude,
				"check_in_long":    in.Longitude,
				"store_lat":        st.Latitude,
				"store_long":       st.Longitude,
			},
			IPAddress: in.IPAddress,
		})
	}

	return &CheckInResult{
		ID:               saved.ID,
		StoreName:        st.Name,
		DistanceMeters:   saved.DistanceMeters,
		GeofenceVerified: saved.GeofenceVerified,
		CheckInAt:        saved.CheckInAt,
		Status:           string(saved.Status),
	}, nil
}

// CompleteVisit flips a visit to COMPLETED.
func (s *VisitService) CompleteVisit(ctx context.Context, tenantID, id string) (visit.Visit, error) {
	return s.visits.CompleteVisit(ctx, tenantID, id)
}

// LookupStore is a small convenience used by the PWA to render store info.
func (s *VisitService) LookupStore(ctx context.Context, tenantID, id string) (store.Store, error) {
	return s.stores.Get(ctx, tenantID, id)
}

// beatplanFrom converts the service input to a domain value.
func beatplanFrom(in CreateBeatPlanInput) beatplan.Plan {
	return beatplan.Plan{
		TenantID:          in.TenantID,
		PlanNumber:        in.PlanNumber,
		SalesRepID:        in.SalesRepID,
		PlanDate:          in.PlanDate,
		Territory:         in.Territory,
		TargetStoresCount: in.TargetStoresCount,
		Status:            beatplan.StatusAssigned,
		Notes:             in.Notes,
	}
}