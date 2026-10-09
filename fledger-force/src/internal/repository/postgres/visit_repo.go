package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-force/internal/platform/errors"
	"github.com/fledger/fledger-force/internal/domain/beatplan"
	"github.com/fledger/fledger-force/internal/domain/visit"
)

type VisitRepo struct {
	pool *pgxpool.Pool
}

func NewVisitRepo(p *pgxpool.Pool) *VisitRepo { return &VisitRepo{pool: p} }

// InsertBeatPlan creates a new beat plan row.
func (r *VisitRepo) InsertBeatPlan(ctx context.Context, p beatplan.Plan) (beatplan.Plan, error) {
	if p.Status == "" {
		p.Status = beatplan.StatusAssigned
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO force_beat_plans
		  (tenant_id, plan_number, sales_rep_id, plan_date, territory,
		   target_stores_count, status, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, tenant_id, plan_number, sales_rep_id, plan_date, territory,
		          target_stores_count, visited_stores_count, status, notes,
		          created_at, updated_at`,
		p.TenantID, p.PlanNumber, p.SalesRepID, p.PlanDate, p.Territory,
		p.TargetStoresCount, string(p.Status), p.Notes,
	)
	var out beatplan.Plan
	var status string
	if err := row.Scan(
		&out.ID, &out.TenantID, &out.PlanNumber, &out.SalesRepID, &out.PlanDate,
		&out.Territory, &out.TargetStoresCount, &out.VisitedStoresCount,
		&status, &out.Notes, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		if isUniqueViolation(err) {
			return beatplan.Plan{}, fmt.Errorf("%w: plan_number already exists", apperrors.ErrConflict)
		}
		return beatplan.Plan{}, fmt.Errorf("insert beat plan: %w", err)
	}
	out.Status = beatplan.Status(status)
	return out, nil
}

// ListBeatPlansForRepDate returns the rep's plan for a specific date.
func (r *VisitRepo) ListBeatPlansForRepDate(ctx context.Context, tenantID, repID string, date time.Time) ([]beatplan.Plan, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, plan_number, sales_rep_id, plan_date, territory,
		       target_stores_count, visited_stores_count, status, notes,
		       created_at, updated_at
		  FROM force_beat_plans
		 WHERE tenant_id = $1 AND sales_rep_id = $2 AND plan_date = $3
		 ORDER BY created_at DESC`, tenantID, repID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]beatplan.Plan, 0, 2)
	for rows.Next() {
		var p beatplan.Plan
		var status string
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.PlanNumber, &p.SalesRepID, &p.PlanDate,
			&p.Territory, &p.TargetStoresCount, &p.VisitedStoresCount,
			&status, &p.Notes, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p.Status = beatplan.Status(status)
		out = append(out, p)
	}
	return out, rows.Err()
}

// InsertVisit writes a check-in row.
func (r *VisitRepo) InsertVisit(ctx context.Context, v visit.Visit) (visit.Visit, error) {
	if v.Status == "" {
		v.Status = visit.StatusCheckedIn
	}
	if v.VisitType == "" {
		v.VisitType = visit.VisitTakingOrderAndCollection
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO force_visits
		  (tenant_id, beat_plan_id, store_id, sales_rep_id, check_in_lat,
		   check_in_long, distance_meters, geofence_verified, visit_type, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, tenant_id, beat_plan_id, store_id, sales_rep_id,
		          check_in_at, check_out_at, check_in_lat, check_in_long,
		          distance_meters, geofence_verified, visit_type, status,
		          COALESCE(skip_reason,''), COALESCE(notes,''), created_at`,
		v.TenantID, v.BeatPlanID, v.StoreID, v.SalesRepID, v.CheckInLat,
		v.CheckInLong, v.DistanceMeters, v.GeofenceVerified, string(v.VisitType), string(v.Status),
	)
	var out visit.Visit
	var vt, st string
	if err := row.Scan(
		&out.ID, &out.TenantID, &out.BeatPlanID, &out.StoreID, &out.SalesRepID,
		&out.CheckInAt, &out.CheckOutAt, &out.CheckInLat, &out.CheckInLong,
		&out.DistanceMeters, &out.GeofenceVerified, &vt, &st,
		&out.SkipReason, &out.Notes, &out.CreatedAt,
	); err != nil {
		return visit.Visit{}, fmt.Errorf("insert visit: %w", err)
	}
	out.VisitType = visit.VisitType(vt)
	out.Status = visit.Status(st)
	return out, nil
}

// CompleteVisit flips a visit to COMPLETED.
func (r *VisitRepo) CompleteVisit(ctx context.Context, tenantID, id string) (visit.Visit, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE force_visits
		   SET status = 'COMPLETED', check_out_at = NOW()
		 WHERE tenant_id = $1 AND id = $2
		 RETURNING id, tenant_id, beat_plan_id, store_id, sales_rep_id,
		           check_in_at, check_out_at, check_in_lat, check_in_long,
		           distance_meters, geofence_verified, visit_type, status,
		           COALESCE(skip_reason,''), COALESCE(notes,''), created_at`,
		tenantID, id,
	)
	var out visit.Visit
	var vt, st string
	if err := row.Scan(
		&out.ID, &out.TenantID, &out.BeatPlanID, &out.StoreID, &out.SalesRepID,
		&out.CheckInAt, &out.CheckOutAt, &out.CheckInLat, &out.CheckInLong,
		&out.DistanceMeters, &out.GeofenceVerified, &vt, &st,
		&out.SkipReason, &out.Notes, &out.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return visit.Visit{}, fmt.Errorf("%w: visit %s", apperrors.ErrNotFound, id)
		}
		return visit.Visit{}, fmt.Errorf("complete visit: %w", err)
	}
	out.VisitType = visit.VisitType(vt)
	out.Status = visit.Status(st)
	return out, nil
}

// GetVisit fetches a single visit.
func (r *VisitRepo) GetVisit(ctx context.Context, tenantID, id string) (visit.Visit, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, beat_plan_id, store_id, sales_rep_id,
		       check_in_at, check_out_at, check_in_lat, check_in_long,
		       distance_meters, geofence_verified, visit_type, status,
		       COALESCE(skip_reason,''), COALESCE(notes,''), created_at
		  FROM force_visits
		 WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	var out visit.Visit
	var vt, st string
	if err := row.Scan(
		&out.ID, &out.TenantID, &out.BeatPlanID, &out.StoreID, &out.SalesRepID,
		&out.CheckInAt, &out.CheckOutAt, &out.CheckInLat, &out.CheckInLong,
		&out.DistanceMeters, &out.GeofenceVerified, &vt, &st,
		&out.SkipReason, &out.Notes, &out.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return visit.Visit{}, fmt.Errorf("%w: visit %s", apperrors.ErrNotFound, id)
		}
		return visit.Visit{}, err
	}
	out.VisitType = visit.VisitType(vt)
	out.Status = visit.Status(st)
	return out, nil
}