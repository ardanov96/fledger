// Package collection defines the daily collection route workflow (Sprint 11,
// Portfolio Sprint 4 / Fase 8 partial).
//
// Domain model:
//
//	collection_route (1) ──< route_stop (N) ──< collection_event (N)
//	                       └── (1) ──> settlement (1)
//
// Lifecycle:
//
//	route: planned → in_progress → completed → settled
//	                     ↓
//	                  cancelled (anytime before settled)
//
//	stop:  pending → visited → closed (or skipped)
//
// Invariants:
//   - 1 route per sales rep per day per tenant (DB unique).
//   - All mutations of route+stops happen in one tx (atomic).
//   - Collection events are immutable (append-only).
//   - Settlement is 1-per-route (DB unique).
//   - Any settlement with discrepancy ≠ 0 requires supervisor approval.
package collection

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Status enums (mirror SQL CHECK constraints)
// =============================================================================

// RouteStatus mirrors collection_routes.status.
type RouteStatus string

const (
	RouteStatusPlanned    RouteStatus = "planned"
	RouteStatusInProgress RouteStatus = "in_progress"
	RouteStatusCompleted  RouteStatus = "completed"
	RouteStatusSettled    RouteStatus = "settled"
	RouteStatusCancelled  RouteStatus = "cancelled"
)

// StopStatus mirrors route_stops.status.
type StopStatus string

const (
	StopStatusPending StopStatus = "pending"
	StopStatusVisited StopStatus = "visited"
	StopStatusSkipped StopStatus = "skipped"
	StopStatusClosed  StopStatus = "closed"
)

// PaymentMethod mirrors collection_events.payment_method.
type PaymentMethod string

const (
	PaymentCash     PaymentMethod = "cash"
	PaymentQRIS     PaymentMethod = "qris"
	PaymentTransfer PaymentMethod = "transfer"
	PaymentCheque   PaymentMethod = "cheque"
)

// SettlementStatus mirrors settlements.status.
type SettlementStatus string

const (
	SettlementPending  SettlementStatus = "pending"
	SettlementApproved SettlementStatus = "approved"
	SettlementDisputed SettlementStatus = "disputed"
	SettlementRejected SettlementStatus = "rejected"
)

// =============================================================================
// Entities
// =============================================================================

// CollectionRoute is one daily route plan.
type CollectionRoute struct {
	ID                  string
	TenantID            string
	SalesRepID          string
	RouteDate           time.Time // truncated to date
	Status              RouteStatus
	TotalPlannedMinor   int64
	TotalCollectedMinor int64
	CreatedAt           time.Time
	StartedAt           *time.Time
	CompletedAt         *time.Time
	SettledAt           *time.Time
	Metadata            map[string]any
}

// RouteStop is one customer visit within a route.
type RouteStop struct {
	ID                    string
	RouteID               string
	CustomerID            string
	Sequence              int
	PlannedInvoiceIDs     []string // invoice UUIDs expected to be collected
	ActualCollectionMinor int64
	Status                StopStatus
	VisitedAt             *time.Time
	ClosedAt              *time.Time
	Notes                 string
	Latitude              *float64
	Longitude             *float64
}

// CollectionEvent is one append-only collection at a stop.
type CollectionEvent struct {
	ID            string
	StopID        string
	AmountMinor   int64
	PaymentMethod PaymentMethod
	Reference     string
	CollectedAt   time.Time
	Notes         string
	RecordedBy    string
}

// Settlement is the end-of-day deposit for a route.
type Settlement struct {
	ID                  string
	RouteID             string
	ExpectedAmountMinor int64
	SettledAmountMinor  int64
	DiscrepancyMinor    int64 // settled - expected
	Status              SettlementStatus
	SubmittedAt         *time.Time
	ApprovedAt          *time.Time
	ApprovedBy          string
	Notes               string
}

// =============================================================================
// Inputs
// =============================================================================

// PlanRouteInput is the input for planning a new route.
type PlanRouteInput struct {
	TenantID      string
	SalesRepID    string
	RouteDate     time.Time
	AutoPopulate  bool     // if true, auto-suggest stops from outstanding invoices
	CustomerIDs   []string // if provided (and AutoPopulate=true), override with these
	Metadata      map[string]any
}

// RecordVisitInput is the input for recording a collection visit at a stop.
type RecordVisitInput struct {
	StopID        string
	AmountMinor   int64
	PaymentMethod PaymentMethod
	Reference     string
	RecordedBy    string
	Notes         string
}

// CloseStopInput marks a stop as closed (operator confirmation).
type CloseStopInput struct {
	StopID string
	Notes  string
}

// SettleRouteInput is the input for sales rep's end-of-day deposit.
type SettleRouteInput struct {
	RouteID           string
	SettledAmountMinor int64
	RecordedBy        string
	Notes             string
}

// ApproveSettlementInput is the supervisor approval.
type ApproveSettlementInput struct {
	SettlementID string
	ApproverID   string
	Approve      bool // false → reject (status=rejected)
	Notes        string
}

// =============================================================================
// Outstanding invoice helper (used for auto-populating stops)
// =============================================================================

// OutstandingInvoiceRef is a minimal invoice reference used to plan a route.
// Returned by the InvoiceRepository.OutstandingByCustomer(ctx, tenantID, customerIDs).
type OutstandingInvoiceRef struct {
	ID         string
	CustomerID string
	AmountMinor int64
	DueDate    time.Time
}

// InvoiceLookup is the narrow interface used by the use case to auto-populate
// stops. Implemented by an adapter that wraps the invoice repository.
type InvoiceLookup interface {
	// OutstandingByCustomer returns outstanding (open/partial) invoices
	// for the given customer IDs within a tenant, sorted by due_date ASC.
	OutstandingByCustomer(ctx context.Context, tenantID string, customerIDs []string) ([]OutstandingInvoiceRef, error)
}

// =============================================================================
// Repository interface
// =============================================================================

// Repository defines persistence operations for the collection module.
type Repository interface {
	// ----- Routes -----

	// CreateRoute inserts a new route (status=planned). Caller's tx.
	CreateRoute(ctx context.Context, tx Tx, route CollectionRoute) error

	// GetRoute reads a route by id (no lock).
	GetRoute(ctx context.Context, id string) (CollectionRoute, error)

	// LockRoute reads a route with SELECT FOR UPDATE. Caller's tx.
	LockRoute(ctx context.Context, tx Tx, id string) (CollectionRoute, error)

	// UpdateRouteStatus mutates status + lifecycle timestamps. Caller's tx.
	UpdateRouteStatus(ctx context.Context, tx Tx, id string, status RouteStatus,
		startedAt, completedAt, settledAt *time.Time) error

	// ListRoutesByDate returns routes for a tenant on a given date.
	ListRoutesByDate(ctx context.Context, tenantID string, date time.Time) ([]CollectionRoute, error)

	// ListRoutesBySalesRep returns recent routes for a sales rep.
	ListRoutesBySalesRep(ctx context.Context, tenantID, salesRepID string, limit int) ([]CollectionRoute, error)

	// ----- Stops -----

	// InsertStop inserts a new stop. Caller's tx.
	InsertStop(ctx context.Context, tx Tx, stop RouteStop) error

	// GetStop reads a stop by id (no lock).
	GetStop(ctx context.Context, id string) (RouteStop, error)

	// LockStop reads a stop with SELECT FOR UPDATE. Caller's tx.
	LockStop(ctx context.Context, tx Tx, id string) (RouteStop, error)

	// ListStopsByRoute returns all stops for a route, ordered by sequence.
	ListStopsByRoute(ctx context.Context, routeID string) ([]RouteStop, error)

	// MarkStopVisited flips stop status to 'visited' and sets visited_at. Caller's tx.
	MarkStopVisited(ctx context.Context, tx Tx, id string, visitedAt time.Time) error

	// MarkStopClosed flips stop status to 'closed' and sets closed_at. Caller's tx.
	MarkStopClosed(ctx context.Context, tx Tx, id string, closedAt time.Time, notes string) error

	// ----- Events -----

	// InsertEvent inserts a new collection event. Caller's tx.
	InsertEvent(ctx context.Context, tx Tx, event CollectionEvent) error

	// ListEventsByStop returns all events for a stop (chronological).
	ListEventsByStop(ctx context.Context, stopID string) ([]CollectionEvent, error)

	// ----- Settlements -----

	// CreateSettlement inserts a new settlement. Caller's tx.
	CreateSettlement(ctx context.Context, tx Tx, settlement Settlement) error

	// GetSettlement reads a settlement by id (no lock).
	GetSettlement(ctx context.Context, id string) (Settlement, error)

	// GetSettlementByRoute reads the (unique) settlement for a route.
	GetSettlementByRoute(ctx context.Context, routeID string) (Settlement, error)

	// UpdateSettlementStatus flips settlement status. Caller's tx.
	UpdateSettlementStatus(ctx context.Context, tx Tx, id string, status SettlementStatus,
		approvedAt *time.Time, approvedBy, notes string) error
}

// =============================================================================
// Transaction abstraction (mirrors reconciler pattern)
// =============================================================================

// Tx is the transaction abstraction used by the collection module.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// CommandTag is the result of an Exec.
type CommandTag interface {
	RowsAffected() int64
}

// Row is a single-row query result.
type Row interface {
	Scan(dest ...any) error
}

// Rows is a multi-row query result.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// =============================================================================
// Convenience: generate ID (mirrors invoice/reconciler style)
// =============================================================================

// NewID is a convenience wrapper around uuid.NewString (exported for callers).
func NewID() string {
	return uuid.NewString()
}
