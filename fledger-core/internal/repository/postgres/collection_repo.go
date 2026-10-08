// Package postgres — CollectionRepository implements collection.Repository.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"

	"github.com/runut/fmcg-wallet/internal/domain/collection"
)

// =============================================================================
// DTOs (mirror row shape, used for scanning)
// =============================================================================

type collectionRouteDTO struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	SalesRepID          uuid.UUID
	RouteDate           time.Time
	Status              string
	TotalPlannedMinor   int64
	TotalCollectedMinor int64
	CreatedAt           time.Time
	StartedAt           *time.Time
	CompletedAt         *time.Time
	SettledAt           *time.Time
	Metadata            []byte
}

type routeStopDTO struct {
	ID                    uuid.UUID
	RouteID               uuid.UUID
	CustomerID            uuid.UUID
	Sequence              int
	PlannedInvoiceIDs     []uuid.UUID
	ActualCollectionMinor int64
	Status                string
	VisitedAt             *time.Time
	ClosedAt              *time.Time
	Notes                 string
	Latitude              *float64
	Longitude             *float64
}

type collectionEventDTO struct {
	ID            uuid.UUID
	StopID        uuid.UUID
	AmountMinor   int64
	PaymentMethod string
	Reference     string
	CollectedAt   time.Time
	Notes         string
	RecordedBy    uuid.UUID
}

type settlementDTO struct {
	ID                  uuid.UUID
	RouteID             uuid.UUID
	ExpectedAmountMinor int64
	SettledAmountMinor  int64
	DiscrepancyMinor    int64
	Status              string
	SubmittedAt         *time.Time
	ApprovedAt          *time.Time
	ApprovedBy          *uuid.UUID
	Notes               string
}

// =============================================================================
// Repository
// =============================================================================

type CollectionRepository struct {
	db *DB
}

func NewCollectionRepository(db *DB) *CollectionRepository {
	return &CollectionRepository{db: db}
}

func (r *CollectionRepository) assertTx(tx collection.Tx) (*collectionTxAdapter, error) {
	a, ok := tx.(*collectionTxAdapter)
	if !ok {
		return nil, fmt.Errorf("expected *collectionTxAdapter, got %T", tx)
	}
	return a, nil
}

// ----- Routes -----

func (r *CollectionRepository) CreateRoute(ctx context.Context, tx collection.Tx, route collection.CollectionRoute) error {
	a, err := r.assertTx(tx)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO collection_routes (
    id, tenant_id, sales_rep_id, route_date, status,
    total_planned_minor, total_collected_minor,
    metadata
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7,
    $8
)
`
	_, err = a.pgxTx.Exec(ctx, q,
		route.ID, route.TenantID, route.SalesRepID, route.RouteDate, string(route.Status),
		route.TotalPlannedMinor, route.TotalCollectedMinor,
		jsonRaw(route.Metadata),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.ErrAlreadyExists
		}
		return fmt.Errorf("create collection route: %w", err)
	}
	return nil
}

func (r *CollectionRepository) GetRoute(ctx context.Context, id string) (collection.CollectionRoute, error) {
	return r.getRouteByID(ctx, r.db.Pool.QueryRow, id, false)
}

func (r *CollectionRepository) LockRoute(ctx context.Context, tx collection.Tx, id string) (collection.CollectionRoute, error) {
	a, err := r.assertTx(tx)
	if err != nil {
		return collection.CollectionRoute{}, err
	}
	return r.getRouteByID(ctx, a.pgxTx.QueryRow, id, true)
}

func (r *CollectionRepository) getRouteByID(
	ctx context.Context,
	queryRow func(ctx context.Context, sql string, args ...any) pgx.Row,
	id string, withLock bool,
) (collection.CollectionRoute, error) {
	q := `
SELECT id, tenant_id, sales_rep_id, route_date, status,
       total_planned_minor, total_collected_minor,
       created_at, started_at, completed_at, settled_at, metadata
FROM collection_routes
WHERE id = $1
`
	if withLock {
		q += " FOR UPDATE"
	}
	var dto collectionRouteDTO
	err := queryRow(ctx, q, id).Scan(
		&dto.ID, &dto.TenantID, &dto.SalesRepID, &dto.RouteDate, &dto.Status,
		&dto.TotalPlannedMinor, &dto.TotalCollectedMinor,
		&dto.CreatedAt, &dto.StartedAt, &dto.CompletedAt, &dto.SettledAt, &dto.Metadata,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return collection.CollectionRoute{}, apperrors.ErrNotFound
		}
		return collection.CollectionRoute{}, fmt.Errorf("scan collection route: %w", err)
	}
	return dtoToCollectionRoute(dto), nil
}

func (r *CollectionRepository) UpdateRouteStatus(
	ctx context.Context, tx collection.Tx, id string,
	status collection.RouteStatus,
	startedAt, completedAt, settledAt *time.Time,
) error {
	a, err := r.assertTx(tx)
	if err != nil {
		return err
	}
	const q = `
UPDATE collection_routes
SET status = $2,
    started_at = COALESCE($3, started_at),
    completed_at = COALESCE($4, completed_at),
    settled_at = COALESCE($5, settled_at)
WHERE id = $1
`
	_, err = a.pgxTx.Exec(ctx, q, id, string(status), startedAt, completedAt, settledAt)
	if err != nil {
		return fmt.Errorf("update collection route status: %w", err)
	}
	return nil
}

func (r *CollectionRepository) ListRoutesByDate(ctx context.Context, tenantID string, date time.Time) ([]collection.CollectionRoute, error) {
	const q = `
SELECT id, tenant_id, sales_rep_id, route_date, status,
       total_planned_minor, total_collected_minor,
       created_at, started_at, completed_at, settled_at, metadata
FROM collection_routes
WHERE tenant_id = $1 AND route_date = $2
ORDER BY created_at DESC
`
	return r.queryRoutes(ctx, q, tenantID, date)
}

func (r *CollectionRepository) ListRoutesBySalesRep(ctx context.Context, tenantID, salesRepID string, limit int) ([]collection.CollectionRoute, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
SELECT id, tenant_id, sales_rep_id, route_date, status,
       total_planned_minor, total_collected_minor,
       created_at, started_at, completed_at, settled_at, metadata
FROM collection_routes
WHERE tenant_id = $1 AND sales_rep_id = $2
ORDER BY route_date DESC
LIMIT $3
`
	return r.queryRoutes(ctx, q, tenantID, salesRepID, limit)
}

func (r *CollectionRepository) queryRoutes(ctx context.Context, q string, args ...any) ([]collection.CollectionRoute, error) {
	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query collection routes: %w", err)
	}
	defer rows.Close()

	out := make([]collection.CollectionRoute, 0, 8)
	for rows.Next() {
		var dto collectionRouteDTO
		if err := rows.Scan(
			&dto.ID, &dto.TenantID, &dto.SalesRepID, &dto.RouteDate, &dto.Status,
			&dto.TotalPlannedMinor, &dto.TotalCollectedMinor,
			&dto.CreatedAt, &dto.StartedAt, &dto.CompletedAt, &dto.SettledAt, &dto.Metadata,
		); err != nil {
			return nil, fmt.Errorf("scan collection route: %w", err)
		}
		out = append(out, dtoToCollectionRoute(dto))
	}
	return out, rows.Err()
}

// ----- Stops -----

func (r *CollectionRepository) InsertStop(ctx context.Context, tx collection.Tx, stop collection.RouteStop) error {
	a, err := r.assertTx(tx)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO route_stops (
    id, route_id, customer_id, sequence,
    planned_invoice_ids, actual_collection_minor,
    status, notes, latitude, longitude
) VALUES (
    $1, $2, $3, $4,
    $5, $6,
    $7, $8, $9, $10
)
`
	invoiceUUIDs := make([]uuid.UUID, len(stop.PlannedInvoiceIDs))
	for i, s := range stop.PlannedInvoiceIDs {
		id, perr := uuid.Parse(s)
		if perr != nil {
			return fmt.Errorf("invalid planned_invoice_ids[%d]: %w", i, perr)
		}
		invoiceUUIDs[i] = id
	}
	_, err = a.pgxTx.Exec(ctx, q,
		stop.ID, stop.RouteID, stop.CustomerID, stop.Sequence,
		invoiceUUIDs, stop.ActualCollectionMinor,
		string(stop.Status), stop.Notes, stop.Latitude, stop.Longitude,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.ErrAlreadyExists
		}
		return fmt.Errorf("insert route stop: %w", err)
	}
	return nil
}

func (r *CollectionRepository) GetStop(ctx context.Context, id string) (collection.RouteStop, error) {
	return r.getStopByID(ctx, r.db.Pool.QueryRow, id, false)
}

func (r *CollectionRepository) LockStop(ctx context.Context, tx collection.Tx, id string) (collection.RouteStop, error) {
	a, err := r.assertTx(tx)
	if err != nil {
		return collection.RouteStop{}, err
	}
	return r.getStopByID(ctx, a.pgxTx.QueryRow, id, true)
}

func (r *CollectionRepository) getStopByID(
	ctx context.Context,
	queryRow func(ctx context.Context, sql string, args ...any) pgx.Row,
	id string, withLock bool,
) (collection.RouteStop, error) {
	q := `
SELECT id, route_id, customer_id, sequence,
       planned_invoice_ids, actual_collection_minor,
       status, visited_at, closed_at, notes,
       latitude, longitude
FROM route_stops
WHERE id = $1
`
	if withLock {
		q += " FOR UPDATE"
	}
	var dto routeStopDTO
	err := queryRow(ctx, q, id).Scan(
		&dto.ID, &dto.RouteID, &dto.CustomerID, &dto.Sequence,
		&dto.PlannedInvoiceIDs, &dto.ActualCollectionMinor,
		&dto.Status, &dto.VisitedAt, &dto.ClosedAt, &dto.Notes,
		&dto.Latitude, &dto.Longitude,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return collection.RouteStop{}, apperrors.ErrNotFound
		}
		return collection.RouteStop{}, fmt.Errorf("scan route stop: %w", err)
	}
	return dtoToRouteStop(dto), nil
}

func (r *CollectionRepository) ListStopsByRoute(ctx context.Context, routeID string) ([]collection.RouteStop, error) {
	const q = `
SELECT id, route_id, customer_id, sequence,
       planned_invoice_ids, actual_collection_minor,
       status, visited_at, closed_at, notes,
       latitude, longitude
FROM route_stops
WHERE route_id = $1
ORDER BY sequence ASC
`
	rows, err := r.db.Pool.Query(ctx, q, routeID)
	if err != nil {
		return nil, fmt.Errorf("list route stops: %w", err)
	}
	defer rows.Close()

	out := make([]collection.RouteStop, 0, 8)
	for rows.Next() {
		var dto routeStopDTO
		if err := rows.Scan(
			&dto.ID, &dto.RouteID, &dto.CustomerID, &dto.Sequence,
			&dto.PlannedInvoiceIDs, &dto.ActualCollectionMinor,
			&dto.Status, &dto.VisitedAt, &dto.ClosedAt, &dto.Notes,
			&dto.Latitude, &dto.Longitude,
		); err != nil {
			return nil, fmt.Errorf("scan route stop: %w", err)
		}
		out = append(out, dtoToRouteStop(dto))
	}
	return out, rows.Err()
}

func (r *CollectionRepository) MarkStopVisited(ctx context.Context, tx collection.Tx, id string, visitedAt time.Time) error {
	a, err := r.assertTx(tx)
	if err != nil {
		return err
	}
	const q = `
UPDATE route_stops
SET status = 'visited', visited_at = $2
WHERE id = $1
`
	_, err = a.pgxTx.Exec(ctx, q, id, visitedAt)
	if err != nil {
		return fmt.Errorf("mark stop visited: %w", err)
	}
	return nil
}

func (r *CollectionRepository) MarkStopClosed(ctx context.Context, tx collection.Tx, id string, closedAt time.Time, notes string) error {
	a, err := r.assertTx(tx)
	if err != nil {
		return err
	}
	const q = `
UPDATE route_stops
SET status = 'closed', closed_at = $2, notes = COALESCE(NULLIF($3, ''), notes)
WHERE id = $1
`
	_, err = a.pgxTx.Exec(ctx, q, id, closedAt, notes)
	if err != nil {
		return fmt.Errorf("mark stop closed: %w", err)
	}
	return nil
}

// ----- Events -----

func (r *CollectionRepository) InsertEvent(ctx context.Context, tx collection.Tx, event collection.CollectionEvent) error {
	a, err := r.assertTx(tx)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO collection_events (
    id, stop_id, amount_minor, payment_method,
    reference, collected_at, notes, recorded_by
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8
)
`
	recordedBy, perr := uuid.Parse(event.RecordedBy)
	if perr != nil {
		return fmt.Errorf("invalid recorded_by: %w", perr)
	}
	_, err = a.pgxTx.Exec(ctx, q,
		event.ID, event.StopID, event.AmountMinor, string(event.PaymentMethod),
		event.Reference, event.CollectedAt, event.Notes, recordedBy,
	)
	if err != nil {
		return fmt.Errorf("insert collection event: %w", err)
	}
	return nil
}

func (r *CollectionRepository) ListEventsByStop(ctx context.Context, stopID string) ([]collection.CollectionEvent, error) {
	const q = `
SELECT id, stop_id, amount_minor, payment_method,
       reference, collected_at, notes, recorded_by
FROM collection_events
WHERE stop_id = $1
ORDER BY collected_at ASC
`
	rows, err := r.db.Pool.Query(ctx, q, stopID)
	if err != nil {
		return nil, fmt.Errorf("list collection events: %w", err)
	}
	defer rows.Close()

	out := make([]collection.CollectionEvent, 0, 4)
	for rows.Next() {
		var dto collectionEventDTO
		if err := rows.Scan(
			&dto.ID, &dto.StopID, &dto.AmountMinor, &dto.PaymentMethod,
			&dto.Reference, &dto.CollectedAt, &dto.Notes, &dto.RecordedBy,
		); err != nil {
			return nil, fmt.Errorf("scan collection event: %w", err)
		}
		out = append(out, dtoToCollectionEvent(dto))
	}
	return out, rows.Err()
}

// ----- Settlements -----

func (r *CollectionRepository) CreateSettlement(ctx context.Context, tx collection.Tx, s collection.Settlement) error {
	a, err := r.assertTx(tx)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO settlements (
    id, route_id, expected_amount_minor, settled_amount_minor,
    discrepancy_minor, status, submitted_at, notes
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8
)
`
	_, err = a.pgxTx.Exec(ctx, q,
		s.ID, s.RouteID, s.ExpectedAmountMinor, s.SettledAmountMinor,
		s.DiscrepancyMinor, string(s.Status), s.SubmittedAt, s.Notes,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.ErrAlreadyExists
		}
		return fmt.Errorf("create settlement: %w", err)
	}
	return nil
}

func (r *CollectionRepository) GetSettlement(ctx context.Context, id string) (collection.Settlement, error) {
	const q = `
SELECT id, route_id, expected_amount_minor, settled_amount_minor,
       discrepancy_minor, status, submitted_at, approved_at, approved_by, notes
FROM settlements
WHERE id = $1
`
	var dto settlementDTO
	err := r.db.Pool.QueryRow(ctx, q, id).Scan(
		&dto.ID, &dto.RouteID, &dto.ExpectedAmountMinor, &dto.SettledAmountMinor,
		&dto.DiscrepancyMinor, &dto.Status, &dto.SubmittedAt, &dto.ApprovedAt, &dto.ApprovedBy, &dto.Notes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return collection.Settlement{}, apperrors.ErrNotFound
		}
		return collection.Settlement{}, fmt.Errorf("scan settlement: %w", err)
	}
	return dtoToSettlement(dto), nil
}

func (r *CollectionRepository) GetSettlementByRoute(ctx context.Context, routeID string) (collection.Settlement, error) {
	const q = `
SELECT id, route_id, expected_amount_minor, settled_amount_minor,
       discrepancy_minor, status, submitted_at, approved_at, approved_by, notes
FROM settlements
WHERE route_id = $1
`
	var dto settlementDTO
	err := r.db.Pool.QueryRow(ctx, q, routeID).Scan(
		&dto.ID, &dto.RouteID, &dto.ExpectedAmountMinor, &dto.SettledAmountMinor,
		&dto.DiscrepancyMinor, &dto.Status, &dto.SubmittedAt, &dto.ApprovedAt, &dto.ApprovedBy, &dto.Notes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return collection.Settlement{}, apperrors.ErrNotFound
		}
		return collection.Settlement{}, fmt.Errorf("scan settlement: %w", err)
	}
	return dtoToSettlement(dto), nil
}

func (r *CollectionRepository) UpdateSettlementStatus(
	ctx context.Context, tx collection.Tx, id string,
	status collection.SettlementStatus,
	approvedAt *time.Time, approvedBy, notes string,
) error {
	a, err := r.assertTx(tx)
	if err != nil {
		return err
	}
	var approvedByUUID *uuid.UUID
	if approvedBy != "" {
		u, perr := uuid.Parse(approvedBy)
		if perr != nil {
			return fmt.Errorf("invalid approved_by: %w", perr)
		}
		approvedByUUID = &u
	}
	const q = `
UPDATE settlements
SET status = $2,
    approved_at = COALESCE($3, approved_at),
    approved_by = COALESCE($4, approved_by),
    notes = COALESCE(NULLIF($5, ''), notes)
WHERE id = $1
`
	_, err = a.pgxTx.Exec(ctx, q, id, string(status), approvedAt, approvedByUUID, notes)
	if err != nil {
		return fmt.Errorf("update settlement status: %w", err)
	}
	return nil
}

// =============================================================================
// DTO → domain helpers
// =============================================================================

func dtoToCollectionRoute(dto collectionRouteDTO) collection.CollectionRoute {
	return collection.CollectionRoute{
		ID:                  dto.ID.String(),
		TenantID:            dto.TenantID.String(),
		SalesRepID:          dto.SalesRepID.String(),
		RouteDate:           dto.RouteDate,
		Status:              collection.RouteStatus(dto.Status),
		TotalPlannedMinor:   dto.TotalPlannedMinor,
		TotalCollectedMinor: dto.TotalCollectedMinor,
		CreatedAt:           dto.CreatedAt,
		StartedAt:           dto.StartedAt,
		CompletedAt:         dto.CompletedAt,
		SettledAt:           dto.SettledAt,
		Metadata:            parseMetadata(dto.Metadata),
	}
}

func dtoToRouteStop(dto routeStopDTO) collection.RouteStop {
	invoiceIDs := make([]string, len(dto.PlannedInvoiceIDs))
	for i, u := range dto.PlannedInvoiceIDs {
		invoiceIDs[i] = u.String()
	}
	return collection.RouteStop{
		ID:                    dto.ID.String(),
		RouteID:               dto.RouteID.String(),
		CustomerID:            dto.CustomerID.String(),
		Sequence:              dto.Sequence,
		PlannedInvoiceIDs:     invoiceIDs,
		ActualCollectionMinor: dto.ActualCollectionMinor,
		Status:                collection.StopStatus(dto.Status),
		VisitedAt:             dto.VisitedAt,
		ClosedAt:              dto.ClosedAt,
		Notes:                 dto.Notes,
		Latitude:              dto.Latitude,
		Longitude:             dto.Longitude,
	}
}

func dtoToCollectionEvent(dto collectionEventDTO) collection.CollectionEvent {
	return collection.CollectionEvent{
		ID:            dto.ID.String(),
		StopID:        dto.StopID.String(),
		AmountMinor:   dto.AmountMinor,
		PaymentMethod: collection.PaymentMethod(dto.PaymentMethod),
		Reference:     dto.Reference,
		CollectedAt:   dto.CollectedAt,
		Notes:         dto.Notes,
		RecordedBy:    dto.RecordedBy.String(),
	}
}

func dtoToSettlement(dto settlementDTO) collection.Settlement {
	var approvedBy string
	if dto.ApprovedBy != nil {
		approvedBy = dto.ApprovedBy.String()
	}
	return collection.Settlement{
		ID:                  dto.ID.String(),
		RouteID:             dto.RouteID.String(),
		ExpectedAmountMinor: dto.ExpectedAmountMinor,
		SettledAmountMinor:  dto.SettledAmountMinor,
		DiscrepancyMinor:    dto.DiscrepancyMinor,
		Status:              collection.SettlementStatus(dto.Status),
		SubmittedAt:         dto.SubmittedAt,
		ApprovedAt:          dto.ApprovedAt,
		ApprovedBy:          approvedBy,
		Notes:               dto.Notes,
	}
}

// Compile-time guard.
var _ collection.Repository = (*CollectionRepository)(nil)
