// Package usecase — CollectionService orchestrates the daily collection
// route workflow (Sprint 11, Portfolio Sprint 4 / Fase 8 partial).
//
// Workflow:
//
//	PlanRoute         → create route + auto-suggest stops (or manual list)
//	StartRoute        → flip planned → in_progress
//	RecordVisit       → append collection_event + trigger updates stop total
//	CloseStop         → mark stop as closed (operator confirmation)
//	CompleteRoute     → all stops closed → route status completed
//	SettleRoute       → sales rep deposits → create settlement with discrepancy
//	ApproveSettlement → supervisor approves (or rejects) the settlement
//
// All mutations happen in one tx for atomicity. Reads are pool-based.
package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/domain/collection"
)

// CollectionTxRunner is the transaction runner used by CollectionService.
type CollectionTxRunner interface {
	ExecuteTx(ctx context.Context, fn func(collection.Tx) error) error
}

// CollectionService orchestrates daily collection routes.
type CollectionService struct {
	repo   collection.Repository
	lookup collection.InvoiceLookup
	db     CollectionTxRunner
	log    *slog.Logger
	now    func() time.Time
}

// CollectionServiceDeps bundles dependencies.
type CollectionServiceDeps struct {
	Repo    collection.Repository
	Lookup  collection.InvoiceLookup
	DB      CollectionTxRunner
	Logger  *slog.Logger
	NowFunc func() time.Time
}

// NewCollectionService constructs a CollectionService.
func NewCollectionService(deps CollectionServiceDeps) *CollectionService {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	nowFn := deps.NowFunc
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	return &CollectionService{
		repo:   deps.Repo,
		lookup: deps.Lookup,
		db:     deps.DB,
		log:    log,
		now:    nowFn,
	}
}

// PlanRoute creates a new route with auto-suggested (or manual) stops.
// All in 1 tx: route insert + N stop inserts are atomic.
// Idempotent: if route already exists for (tenant, sales_rep, date), returns ErrAlreadyExists.
func (s *CollectionService) PlanRoute(ctx context.Context, in collection.PlanRouteInput) (collection.CollectionRoute, []collection.RouteStop, error) {
	if _, err := uuid.Parse(in.TenantID); err != nil {
		return collection.CollectionRoute{}, nil, fmt.Errorf("%w: invalid tenant_id", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(in.SalesRepID); err != nil {
		return collection.CollectionRoute{}, nil, fmt.Errorf("%w: invalid sales_rep_id", apperrors.ErrInvalidInput)
	}
	if in.RouteDate.IsZero() {
		return collection.CollectionRoute{}, nil, fmt.Errorf("%w: route_date required", apperrors.ErrInvalidInput)
	}

	now := s.now()
	routeID := uuid.NewString()
	route := collection.CollectionRoute{
		ID:         routeID,
		TenantID:   in.TenantID,
		SalesRepID: in.SalesRepID,
		RouteDate:  in.RouteDate,
		Status:     collection.RouteStatusPlanned,
		CreatedAt:  now,
		Metadata:   in.Metadata,
	}

	var stops []collection.RouteStop

	err := s.db.ExecuteTx(ctx, func(tx collection.Tx) error {
		if err := s.repo.CreateRoute(ctx, tx, route); err != nil {
			return fmt.Errorf("create route: %w", err)
		}

		customerIDs := in.CustomerIDs
		if in.AutoPopulate && len(customerIDs) == 0 && s.lookup != nil {
			invs, lerr := s.lookup.OutstandingByCustomer(ctx, in.TenantID, nil)
			if lerr != nil {
				return fmt.Errorf("auto-populate lookup: %w", lerr)
			}
			seen := map[string]bool{}
			for _, inv := range invs {
				if !seen[inv.CustomerID] {
					seen[inv.CustomerID] = true
					customerIDs = append(customerIDs, inv.CustomerID)
				}
			}
		}

		for i, customerID := range customerIDs {
			if _, perr := uuid.Parse(customerID); perr != nil {
				return fmt.Errorf("invalid customer_id[%d]: %w", i, perr)
			}
			stop := collection.RouteStop{
				ID:                uuid.NewString(),
				RouteID:           routeID,
				CustomerID:        customerID,
				Sequence:          i + 1,
				PlannedInvoiceIDs: []string{},
				Status:            collection.StopStatusPending,
			}
			if in.AutoPopulate && s.lookup != nil {
				invs, lerr := s.lookup.OutstandingByCustomer(ctx, in.TenantID, []string{customerID})
				if lerr != nil {
					return fmt.Errorf("lookup invoices for %s: %w", customerID, lerr)
				}
				for _, inv := range invs {
					stop.PlannedInvoiceIDs = append(stop.PlannedInvoiceIDs, inv.ID)
					route.TotalPlannedMinor += inv.AmountMinor
				}
			}
			if err := s.repo.InsertStop(ctx, tx, stop); err != nil {
				return fmt.Errorf("insert stop[%d]: %w", i, err)
			}
			stops = append(stops, stop)
		}
		return nil
	})

	if err != nil {
		s.log.Warn("plan route failed", "sales_rep_id", in.SalesRepID, "error", err.Error())
		return collection.CollectionRoute{}, nil, err
	}

	s.log.Info("route planned",
		"route_id", routeID, "sales_rep_id", in.SalesRepID,
		"stops", len(stops), "total_planned_minor", route.TotalPlannedMinor,
	)
	route, _ = s.repo.GetRoute(ctx, routeID)
	return route, stops, nil
}

// StartRoute flips planned → in_progress and stamps started_at.
func (s *CollectionService) StartRoute(ctx context.Context, routeID string) (collection.CollectionRoute, error) {
	if _, err := uuid.Parse(routeID); err != nil {
		return collection.CollectionRoute{}, fmt.Errorf("%w: invalid route_id", apperrors.ErrInvalidInput)
	}
	var route collection.CollectionRoute
	err := s.db.ExecuteTx(ctx, func(tx collection.Tx) error {
		r, err := s.repo.LockRoute(ctx, tx, routeID)
		if err != nil {
			return err
		}
		if r.Status != collection.RouteStatusPlanned {
			return fmt.Errorf("%w: route must be planned (current=%s)", apperrors.ErrInvalidState, r.Status)
		}
		now := s.now()
		if err := s.repo.UpdateRouteStatus(ctx, tx, routeID, collection.RouteStatusInProgress, &now, nil, nil); err != nil {
			return err
		}
		route = r
		route.Status = collection.RouteStatusInProgress
		route.StartedAt = &now
		return nil
	})
	if err != nil {
		return collection.CollectionRoute{}, err
	}
	s.log.Info("route started", "route_id", routeID)
	return route, nil
}

// RecordVisit appends a collection_event at a stop, marks stop visited if not yet.
func (s *CollectionService) RecordVisit(ctx context.Context, in collection.RecordVisitInput) (collection.CollectionEvent, collection.RouteStop, error) {
	if _, err := uuid.Parse(in.StopID); err != nil {
		return collection.CollectionEvent{}, collection.RouteStop{}, fmt.Errorf("%w: invalid stop_id", apperrors.ErrInvalidInput)
	}
	if in.AmountMinor <= 0 {
		return collection.CollectionEvent{}, collection.RouteStop{}, fmt.Errorf("%w: amount_minor must be > 0", apperrors.ErrInvalidInput)
	}
	if in.PaymentMethod == "" {
		return collection.CollectionEvent{}, collection.RouteStop{}, fmt.Errorf("%w: payment_method required", apperrors.ErrInvalidInput)
	}
	switch in.PaymentMethod {
	case collection.PaymentCash, collection.PaymentQRIS, collection.PaymentTransfer, collection.PaymentCheque:
		// OK
	default:
		return collection.CollectionEvent{}, collection.RouteStop{}, fmt.Errorf("%w: invalid payment_method", apperrors.ErrInvalidInput)
	}

	now := s.now()
	event := collection.CollectionEvent{
		ID:            uuid.NewString(),
		StopID:        in.StopID,
		AmountMinor:   in.AmountMinor,
		PaymentMethod: in.PaymentMethod,
		Reference:     in.Reference,
		CollectedAt:   now,
		Notes:         in.Notes,
		RecordedBy:    in.RecordedBy,
	}

	var stop collection.RouteStop

	err := s.db.ExecuteTx(ctx, func(tx collection.Tx) error {
		st, err := s.repo.LockStop(ctx, tx, in.StopID)
		if err != nil {
			return err
		}
		if st.Status == collection.StopStatusClosed || st.Status == collection.StopStatusSkipped {
			return fmt.Errorf("%w: stop is %s (cannot record visit)", apperrors.ErrInvalidState, st.Status)
		}
		stop = st

		if err := s.repo.InsertEvent(ctx, tx, event); err != nil {
			return err
		}

		if st.Status == collection.StopStatusPending {
			if err := s.repo.MarkStopVisited(ctx, tx, in.StopID, now); err != nil {
				return err
			}
			stop.Status = collection.StopStatusVisited
			stop.VisitedAt = &now
		}
		stop.ActualCollectionMinor += in.AmountMinor
		return nil
	})

	if err != nil {
		return collection.CollectionEvent{}, collection.RouteStop{}, err
	}
	s.log.Info("visit recorded",
		"stop_id", in.StopID, "amount_minor", in.AmountMinor,
		"payment_method", in.PaymentMethod,
	)
	return event, stop, nil
}

// CloseStop marks a stop as closed (operator confirmation).
func (s *CollectionService) CloseStop(ctx context.Context, in collection.CloseStopInput) (collection.RouteStop, error) {
	if _, err := uuid.Parse(in.StopID); err != nil {
		return collection.RouteStop{}, fmt.Errorf("%w: invalid stop_id", apperrors.ErrInvalidInput)
	}

	var stop collection.RouteStop
	err := s.db.ExecuteTx(ctx, func(tx collection.Tx) error {
		st, err := s.repo.LockStop(ctx, tx, in.StopID)
		if err != nil {
			return err
		}
		if st.Status == collection.StopStatusClosed {
			return fmt.Errorf("%w: stop already closed", apperrors.ErrInvalidState)
		}
		now := s.now()
		if err := s.repo.MarkStopClosed(ctx, tx, in.StopID, now, in.Notes); err != nil {
			return err
		}
		st.Status = collection.StopStatusClosed
		st.ClosedAt = &now
		stop = st
		return nil
	})

	if err != nil {
		return collection.RouteStop{}, err
	}
	s.log.Info("stop closed", "stop_id", in.StopID)
	return stop, nil
}

// CompleteRoute flips route → completed if all stops are closed/skipped.
func (s *CollectionService) CompleteRoute(ctx context.Context, routeID string) (collection.CollectionRoute, error) {
	if _, err := uuid.Parse(routeID); err != nil {
		return collection.CollectionRoute{}, fmt.Errorf("%w: invalid route_id", apperrors.ErrInvalidInput)
	}
	var route collection.CollectionRoute
	err := s.db.ExecuteTx(ctx, func(tx collection.Tx) error {
		r, err := s.repo.LockRoute(ctx, tx, routeID)
		if err != nil {
			return err
		}
		if r.Status != collection.RouteStatusInProgress {
			return fmt.Errorf("%w: route must be in_progress (current=%s)", apperrors.ErrInvalidState, r.Status)
		}
		stops, err := s.repo.ListStopsByRoute(ctx, routeID)
		if err != nil {
			return err
		}
		for _, st := range stops {
			if st.Status != collection.StopStatusClosed && st.Status != collection.StopStatusSkipped {
				return fmt.Errorf("%w: stop %s is %s (must be closed/skipped)",
					apperrors.ErrInvalidState, st.ID, st.Status)
			}
		}
		now := s.now()
		if err := s.repo.UpdateRouteStatus(ctx, tx, routeID, collection.RouteStatusCompleted, nil, &now, nil); err != nil {
			return err
		}
		route = r
		route.Status = collection.RouteStatusCompleted
		route.CompletedAt = &now
		return nil
	})

	if err != nil {
		return collection.CollectionRoute{}, err
	}
	s.log.Info("route completed", "route_id", routeID)
	return route, nil
}

// SettleRoute creates a settlement for a completed route.
// If discrepancy = 0 → status='approved'. Otherwise → status='pending'.
func (s *CollectionService) SettleRoute(ctx context.Context, in collection.SettleRouteInput) (collection.Settlement, collection.CollectionRoute, error) {
	if _, err := uuid.Parse(in.RouteID); err != nil {
		return collection.Settlement{}, collection.CollectionRoute{}, fmt.Errorf("%w: invalid route_id", apperrors.ErrInvalidInput)
	}
	if in.SettledAmountMinor < 0 {
		return collection.Settlement{}, collection.CollectionRoute{}, fmt.Errorf("%w: settled_amount must be >= 0", apperrors.ErrInvalidInput)
	}

	var settlement collection.Settlement
	var route collection.CollectionRoute

	err := s.db.ExecuteTx(ctx, func(tx collection.Tx) error {
		r, err := s.repo.LockRoute(ctx, tx, in.RouteID)
		if err != nil {
			return err
		}
		if r.Status != collection.RouteStatusCompleted {
			return fmt.Errorf("%w: route must be completed (current=%s)", apperrors.ErrInvalidState, r.Status)
		}
		now := s.now()
		expected := r.TotalCollectedMinor
		discrepancy := in.SettledAmountMinor - expected
		status := collection.SettlementPending
		if discrepancy == 0 {
			status = collection.SettlementApproved
		}
		settlement = collection.Settlement{
			ID:                  uuid.NewString(),
			RouteID:             in.RouteID,
			ExpectedAmountMinor: expected,
			SettledAmountMinor:  in.SettledAmountMinor,
			DiscrepancyMinor:    discrepancy,
			Status:              status,
			SubmittedAt:         &now,
			Notes:               in.Notes,
		}
		if err := s.repo.CreateSettlement(ctx, tx, settlement); err != nil {
			return err
		}
		if err := s.repo.UpdateRouteStatus(ctx, tx, in.RouteID, collection.RouteStatusSettled, nil, nil, &now); err != nil {
			return err
		}
		route = r
		route.Status = collection.RouteStatusSettled
		route.SettledAt = &now
		return nil
	})

	if err != nil {
		return collection.Settlement{}, collection.CollectionRoute{}, err
	}
	s.log.Info("route settled",
		"route_id", in.RouteID,
		"expected", settlement.ExpectedAmountMinor,
		"settled", settlement.SettledAmountMinor,
		"discrepancy", settlement.DiscrepancyMinor,
		"status", settlement.Status,
	)
	return settlement, route, nil
}

// ApproveSettlement supervisor approves or rejects a pending settlement.
func (s *CollectionService) ApproveSettlement(ctx context.Context, in collection.ApproveSettlementInput) (collection.Settlement, error) {
	if _, err := uuid.Parse(in.SettlementID); err != nil {
		return collection.Settlement{}, fmt.Errorf("%w: invalid settlement_id", apperrors.ErrInvalidInput)
	}

	var settlement collection.Settlement
	err := s.db.ExecuteTx(ctx, func(tx collection.Tx) error {
		current, err := s.repo.GetSettlement(ctx, in.SettlementID)
		if err != nil {
			return err
		}
		if current.Status != collection.SettlementPending {
			return fmt.Errorf("%w: settlement must be pending (current=%s)", apperrors.ErrInvalidState, current.Status)
		}
		now := s.now()
		newStatus := collection.SettlementRejected
		if in.Approve {
			newStatus = collection.SettlementApproved
		}
		if err := s.repo.UpdateSettlementStatus(ctx, tx, in.SettlementID, newStatus, &now, in.ApproverID, in.Notes); err != nil {
			return err
		}
		current.Status = newStatus
		current.ApprovedAt = &now
		current.ApprovedBy = in.ApproverID
		if in.Notes != "" {
			current.Notes = in.Notes
		}
		settlement = current
		return nil
	})

	if err != nil {
		return collection.Settlement{}, err
	}
	s.log.Info("settlement decided",
		"settlement_id", in.SettlementID,
		"status", settlement.Status,
		"approver", in.ApproverID,
	)
	return settlement, nil
}

// GetRoute reads a route by id.
func (s *CollectionService) GetRoute(ctx context.Context, id string) (collection.CollectionRoute, error) {
	return s.repo.GetRoute(ctx, id)
}

// ListStopsByRoute returns stops for a route (ordered by sequence).
func (s *CollectionService) ListStopsByRoute(ctx context.Context, routeID string) ([]collection.RouteStop, error) {
	return s.repo.ListStopsByRoute(ctx, routeID)
}

// ListEventsByStop returns events for a stop (chronological).
func (s *CollectionService) ListEventsByStop(ctx context.Context, stopID string) ([]collection.CollectionEvent, error) {
	return s.repo.ListEventsByStop(ctx, stopID)
}

// GetSettlementByRoute returns the settlement for a route.
func (s *CollectionService) GetSettlementByRoute(ctx context.Context, routeID string) (collection.Settlement, error) {
	return s.repo.GetSettlementByRoute(ctx, routeID)
}

// ListRoutesBySalesRep returns recent routes for a sales rep.
func (s *CollectionService) ListRoutesBySalesRep(ctx context.Context, tenantID, salesRepID string, limit int) ([]collection.CollectionRoute, error) {
	return s.repo.ListRoutesBySalesRep(ctx, tenantID, salesRepID, limit)
}

// ListRoutesByDate returns routes for a tenant on a given date.
func (s *CollectionService) ListRoutesByDate(ctx context.Context, tenantID string, date time.Time) ([]collection.CollectionRoute, error) {
	return s.repo.ListRoutesByDate(ctx, tenantID, date)
}
