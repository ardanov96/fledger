//go:build !windows
// +build !windows

// CollectionService tests — Sprint 11 (Collection & Route module).
//
// Validates PlanRoute, StartRoute, RecordVisit, CloseStop, CompleteRoute,
// SettleRoute, ApproveSettlement using in-memory fakes (no real DB).
//
// Run with: go test ./internal/usecase/...
package usecase

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"

	"github.com/runut/fmcg-wallet/internal/domain/collection"
)

// =============================================================================
// In-memory repo fake
// =============================================================================

type fakeCollectionRepo struct {
	mu          sync.Mutex
	routes      map[string]*collection.CollectionRoute
	stops       map[string]*collection.RouteStop
	events      map[string][]collection.CollectionEvent
	settlements map[string]*collection.Settlement
}

func newFakeCollectionRepo() *fakeCollectionRepo {
	return &fakeCollectionRepo{
		routes:      map[string]*collection.CollectionRoute{},
		stops:       map[string]*collection.RouteStop{},
		events:      map[string][]collection.CollectionEvent{},
		settlements: map[string]*collection.Settlement{},
	}
}

func (r *fakeCollectionRepo) assertTx(tx collection.Tx) (*fakeCollTx, error) {
	a, ok := tx.(*fakeCollTx)
	if !ok {
		return nil, errors.New("expected *fakeCollTx")
	}
	return a, nil
}

func (r *fakeCollectionRepo) CreateRoute(_ context.Context, tx collection.Tx, route collection.CollectionRoute) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Idempotent: check unique (tenant, sales_rep, date)
	for _, existing := range r.routes {
		if existing.TenantID == route.TenantID &&
			existing.SalesRepID == route.SalesRepID &&
			sameDate(existing.RouteDate, route.RouteDate) {
			return apperrors.ErrAlreadyExists
		}
	}
	cp := route
	cp.TotalPlannedMinor = 0 // will be recomputed by trigger in prod; we set manually
	cp.TotalCollectedMinor = 0
	r.routes[route.ID] = &cp
	return nil
}

func (r *fakeCollectionRepo) GetRoute(_ context.Context, id string) (collection.CollectionRoute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rt, ok := r.routes[id]
	if !ok {
		return collection.CollectionRoute{}, apperrors.ErrNotFound
	}
	return *rt, nil
}

func (r *fakeCollectionRepo) LockRoute(_ context.Context, tx collection.Tx, id string) (collection.CollectionRoute, error) {
	if _, err := r.assertTx(tx); err != nil {
		return collection.CollectionRoute{}, err
	}
	return r.GetRoute(context.Background(), id)
}

func (r *fakeCollectionRepo) UpdateRouteStatus(_ context.Context, tx collection.Tx, id string, status collection.RouteStatus, startedAt, completedAt, settledAt *time.Time) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rt, ok := r.routes[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	rt.Status = status
	if startedAt != nil {
		rt.StartedAt = startedAt
	}
	if completedAt != nil {
		rt.CompletedAt = completedAt
	}
	if settledAt != nil {
		rt.SettledAt = settledAt
	}
	return nil
}

func (r *fakeCollectionRepo) ListRoutesByDate(_ context.Context, tenantID string, date time.Time) ([]collection.CollectionRoute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []collection.CollectionRoute{}
	for _, rt := range r.routes {
		if rt.TenantID == tenantID && sameDate(rt.RouteDate, date) {
			out = append(out, *rt)
		}
	}
	return out, nil
}

func (r *fakeCollectionRepo) ListRoutesBySalesRep(_ context.Context, tenantID, salesRepID string, _ int) ([]collection.CollectionRoute, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []collection.CollectionRoute{}
	for _, rt := range r.routes {
		if rt.TenantID == tenantID && rt.SalesRepID == salesRepID {
			out = append(out, *rt)
		}
	}
	return out, nil
}

func (r *fakeCollectionRepo) InsertStop(_ context.Context, tx collection.Tx, stop collection.RouteStop) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := stop
	cp.ActualCollectionMinor = 0
	r.stops[stop.ID] = &cp
	// Update parent route total_planned_minor (trigger equivalent).
	if parent, ok := r.routes[stop.RouteID]; ok {
		var sum int64
		for _, s := range r.stops {
			if s.RouteID == stop.RouteID {
				// sum from PlannedInvoiceIDs (parse amount — fake assumes stored in metadata)
				// For simplicity: each stop's planned amount is encoded in stop ID's first 8 chars.
				// Instead, just count the planned invoice IDs as a proxy (1 unit each).
				// Better: skip — we don't track planned amount per stop in fake.
				_ = sum
			}
		}
		_ = parent
	}
	return nil
}

func (r *fakeCollectionRepo) GetStop(_ context.Context, id string) (collection.RouteStop, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.stops[id]
	if !ok {
		return collection.RouteStop{}, apperrors.ErrNotFound
	}
	return *st, nil
}

func (r *fakeCollectionRepo) LockStop(_ context.Context, tx collection.Tx, id string) (collection.RouteStop, error) {
	if _, err := r.assertTx(tx); err != nil {
		return collection.RouteStop{}, err
	}
	return r.GetStop(context.Background(), id)
}

func (r *fakeCollectionRepo) ListStopsByRoute(_ context.Context, routeID string) ([]collection.RouteStop, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []collection.RouteStop{}
	for _, st := range r.stops {
		if st.RouteID == routeID {
			out = append(out, *st)
		}
	}
	return out, nil
}

func (r *fakeCollectionRepo) MarkStopVisited(_ context.Context, tx collection.Tx, id string, visitedAt time.Time) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.stops[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	st.Status = collection.StopStatusVisited
	st.VisitedAt = &visitedAt
	return nil
}

func (r *fakeCollectionRepo) MarkStopClosed(_ context.Context, tx collection.Tx, id string, closedAt time.Time, notes string) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.stops[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	st.Status = collection.StopStatusClosed
	st.ClosedAt = &closedAt
	if notes != "" {
		st.Notes = notes
	}
	return nil
}

func (r *fakeCollectionRepo) InsertEvent(_ context.Context, tx collection.Tx, event collection.CollectionEvent) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[event.StopID] = append(r.events[event.StopID], event)
	// Simulate trigger: update stop's actual_collection_minor.
	if st, ok := r.stops[event.StopID]; ok {
		st.ActualCollectionMinor += event.AmountMinor
	}
	// Simulate trigger: update route's total_collected_minor.
	if event.AmountMinor > 0 {
		if st, ok := r.stops[event.StopID]; ok {
			if parent, ok := r.routes[st.RouteID]; ok {
				parent.TotalCollectedMinor += event.AmountMinor
			}
		}
	}
	return nil
}

func (r *fakeCollectionRepo) ListEventsByStop(_ context.Context, stopID string) ([]collection.CollectionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	events := r.events[stopID]
	out := make([]collection.CollectionEvent, len(events))
	copy(out, events)
	return out, nil
}

func (r *fakeCollectionRepo) CreateSettlement(_ context.Context, tx collection.Tx, s collection.Settlement) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.settlements {
		if existing.RouteID == s.RouteID {
			return apperrors.ErrAlreadyExists
		}
	}
	cp := s
	r.settlements[s.ID] = &cp
	return nil
}

func (r *fakeCollectionRepo) GetSettlement(_ context.Context, id string) (collection.Settlement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.settlements[id]
	if !ok {
		return collection.Settlement{}, apperrors.ErrNotFound
	}
	return *s, nil
}

func (r *fakeCollectionRepo) GetSettlementByRoute(_ context.Context, routeID string) (collection.Settlement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.settlements {
		if s.RouteID == routeID {
			return *s, nil
		}
	}
	return collection.Settlement{}, apperrors.ErrNotFound
}

func (r *fakeCollectionRepo) UpdateSettlementStatus(_ context.Context, tx collection.Tx, id string, status collection.SettlementStatus, approvedAt *time.Time, approvedBy, notes string) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.settlements[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	s.Status = status
	if approvedAt != nil {
		s.ApprovedAt = approvedAt
	}
	if approvedBy != "" {
		s.ApprovedBy = approvedBy
	}
	if notes != "" {
		s.Notes = notes
	}
	return nil
}

// =============================================================================
// Fake tx + tx runner
// =============================================================================

type fakeCollTx struct{}

func (t *fakeCollTx) Exec(_ context.Context, _ string, _ ...any) (collection.CommandTag, error) {
	return fakeCollTag{}, nil
}
func (t *fakeCollTx) Query(_ context.Context, _ string, _ ...any) (collection.Rows, error) {
	return &fakeCollRows{}, nil
}
func (t *fakeCollTx) QueryRow(_ context.Context, _ string, _ ...any) collection.Row {
	return &fakeCollRow{}
}

type fakeCollTag struct{}

func (fakeCollTag) RowsAffected() int64 { return 1 }

type fakeCollRows struct{}

func (r *fakeCollRows) Next() bool          { return false }
func (r *fakeCollRows) Scan(_ ...any) error { return nil }
func (r *fakeCollRows) Err() error          { return nil }
func (r *fakeCollRows) Close()              {}

type fakeCollRow struct{}

func (r *fakeCollRow) Scan(_ ...any) error { return nil }

type fakeCollTxRunner struct{}

func (r *fakeCollTxRunner) ExecuteTx(_ context.Context, fn func(collection.Tx) error) error {
	return fn(&fakeCollTx{})
}

// =============================================================================
// Fake invoice lookup
// =============================================================================

type fakeInvoiceLookup struct {
	mu       sync.Mutex
	invoices map[string][]collection.OutstandingInvoiceRef // customer_id → invoices
}

func (l *fakeInvoiceLookup) OutstandingByCustomer(_ context.Context, _ string, customerIDs []string) ([]collection.OutstandingInvoiceRef, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(customerIDs) == 0 {
		// Return all (used by auto-populate).
		out := []collection.OutstandingInvoiceRef{}
		for _, invs := range l.invoices {
			out = append(out, invs...)
		}
		return out, nil
	}
	out := []collection.OutstandingInvoiceRef{}
	for _, cid := range customerIDs {
		out = append(out, l.invoices[cid]...)
	}
	return out, nil
}

// =============================================================================
// Test fixtures
// =============================================================================

const (
	colTenant1   = "00000000-0000-0000-0000-000000000001"
	colSalesRep  = "00000000-0000-0000-0000-000000000002"
	colCustomer1 = "00000000-0000-0000-0000-000000000010"
	colCustomer2 = "00000000-0000-0000-0000-000000000011"
	colApprover  = "00000000-0000-0000-0000-000000000099"
)

func newCollectionSvc(t *testing.T, lookup collection.InvoiceLookup) (*CollectionService, *fakeCollectionRepo) {
	t.Helper()
	repo := newFakeCollectionRepo()
	svc := NewCollectionService(CollectionServiceDeps{
		Repo:    repo,
		Lookup:  lookup,
		DB:      &fakeCollTxRunner{},
		Logger:  slog.Default(),
		NowFunc: func() time.Time { return time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC) },
	})
	return svc, repo
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// =============================================================================
// Tests
// =============================================================================

// 1. PlanRoute success with manual customer list.
func TestCollectionService_PlanRoute_ManualStops(t *testing.T) {
	t.Parallel()
	lookup := &fakeInvoiceLookup{invoices: map[string][]collection.OutstandingInvoiceRef{}}
	svc, _ := newCollectionSvc(t, lookup)

	route, stops, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		AutoPopulate: false,
		CustomerIDs: []string{colCustomer1, colCustomer2},
	})
	require.NoError(t, err)
	assert.Equal(t, collection.RouteStatusPlanned, route.Status)
	assert.Len(t, stops, 2)
	assert.Equal(t, 1, stops[0].Sequence)
	assert.Equal(t, 2, stops[1].Sequence)
	assert.Equal(t, colCustomer1, stops[0].CustomerID)
	assert.Equal(t, collection.StopStatusPending, stops[0].Status)
}

// 2. PlanRoute auto-populate from outstanding invoices.
func TestCollectionService_PlanRoute_AutoPopulate(t *testing.T) {
	t.Parallel()
	lookup := &fakeInvoiceLookup{invoices: map[string][]collection.OutstandingInvoiceRef{
		colCustomer1: {{ID: uuid.NewString(), CustomerID: colCustomer1, AmountMinor: 50000, DueDate: time.Now()}},
		colCustomer2: {{ID: uuid.NewString(), CustomerID: colCustomer2, AmountMinor: 30000, DueDate: time.Now()}},
	}}
	svc, _ := newCollectionSvc(t, lookup)

	route, stops, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		AutoPopulate: true,
	})
	require.NoError(t, err)
	assert.Len(t, stops, 2)
	// Each stop should have its invoice planned.
	for _, st := range stops {
		assert.Len(t, st.PlannedInvoiceIDs, 1)
	}
}

// 3. PlanRoute idempotent: duplicate (tenant, rep, date) returns ErrAlreadyExists.
func TestCollectionService_PlanRoute_DuplicateSameDay(t *testing.T) {
	t.Parallel()
	lookup := &fakeInvoiceLookup{invoices: map[string][]collection.OutstandingInvoiceRef{}}
	svc, _ := newCollectionSvc(t, lookup)
	in := collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1},
	}
	_, _, err := svc.PlanRoute(context.Background(), in)
	require.NoError(t, err)
	_, _, err = svc.PlanRoute(context.Background(), in)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrAlreadyExists))
}

// 4. PlanRoute invalid input fails.
func TestCollectionService_PlanRoute_InvalidInput(t *testing.T) {
	t.Parallel()
	svc, _ := newCollectionSvc(t, nil)
	_, _, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:   "not-a-uuid",
		SalesRepID: colSalesRep,
		RouteDate:  time.Now(),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}

// 5. StartRoute flips planned → in_progress.
func TestCollectionService_StartRoute_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newCollectionSvc(t, nil)
	route, _, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1},
	})
	require.NoError(t, err)

	started, err := svc.StartRoute(context.Background(), route.ID)
	require.NoError(t, err)
	assert.Equal(t, collection.RouteStatusInProgress, started.Status)
	assert.NotNil(t, started.StartedAt)
}

// 6. StartRoute fails if not planned.
func TestCollectionService_StartRoute_InvalidState(t *testing.T) {
	t.Parallel()
	svc, _ := newCollectionSvc(t, nil)
	route, _, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1},
	})
	require.NoError(t, err)

	_, err = svc.StartRoute(context.Background(), route.ID)
	require.NoError(t, err)
	_, err = svc.StartRoute(context.Background(), route.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidState))
}

// 7. RecordVisit appends event + updates stop total + flips status.
func TestCollectionService_RecordVisit_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newCollectionSvc(t, nil)
	route, stops, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1},
	})
	require.NoError(t, err)
	_, err = svc.StartRoute(context.Background(), route.ID)
	require.NoError(t, err)

	event, stop, err := svc.RecordVisit(context.Background(), collection.RecordVisitInput{
		StopID:        stops[0].ID,
		AmountMinor:   25000,
		PaymentMethod: collection.PaymentCash,
		RecordedBy:    colSalesRep,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(25000), event.AmountMinor)
	assert.Equal(t, collection.StopStatusVisited, stop.Status)
	assert.Equal(t, int64(25000), stop.ActualCollectionMinor)
}

// 8. RecordVisit rejects invalid amount.
func TestCollectionService_RecordVisit_InvalidAmount(t *testing.T) {
	t.Parallel()
	svc, _ := newCollectionSvc(t, nil)
	_, _, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1},
	})
	require.NoError(t, err)

	_, _, err = svc.RecordVisit(context.Background(), collection.RecordVisitInput{
		StopID:        colCustomer1, // not a real stop
		AmountMinor:   0,
		PaymentMethod: collection.PaymentCash,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}

// 9. CloseStop marks stop closed.
func TestCollectionService_CloseStop_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newCollectionSvc(t, nil)
	_, stops, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1},
	})
	require.NoError(t, err)
	closed, err := svc.CloseStop(context.Background(), collection.CloseStopInput{StopID: stops[0].ID})
	require.NoError(t, err)
	assert.Equal(t, collection.StopStatusClosed, closed.Status)
	assert.NotNil(t, closed.ClosedAt)
}

// 10. CompleteRoute fails if any stop is not closed.
func TestCollectionService_CompleteRoute_RequiresAllClosed(t *testing.T) {
	t.Parallel()
	svc, _ := newCollectionSvc(t, nil)
	route, stops, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1, colCustomer2},
	})
	require.NoError(t, err)
	_, err = svc.StartRoute(context.Background(), route.ID)
	require.NoError(t, err)
	_, err = svc.CloseStop(context.Background(), collection.CloseStopInput{StopID: stops[0].ID})
	require.NoError(t, err)
	// stops[1] is still pending.
	_, err = svc.CompleteRoute(context.Background(), route.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidState))
}

// 11. CompleteRoute succeeds when all stops closed.
func TestCollectionService_CompleteRoute_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newCollectionSvc(t, nil)
	route, stops, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1, colCustomer2},
	})
	require.NoError(t, err)
	_, err = svc.StartRoute(context.Background(), route.ID)
	require.NoError(t, err)
	for _, st := range stops {
		_, err = svc.CloseStop(context.Background(), collection.CloseStopInput{StopID: st.ID})
		require.NoError(t, err)
	}
	completed, err := svc.CompleteRoute(context.Background(), route.ID)
	require.NoError(t, err)
	assert.Equal(t, collection.RouteStatusCompleted, completed.Status)
	assert.NotNil(t, completed.CompletedAt)
}

// 12. SettleRoute balanced → auto-approved, route flipped to settled.
func TestCollectionService_SettleRoute_Balanced_AutoApprove(t *testing.T) {
	t.Parallel()
	svc, repo := newCollectionSvc(t, nil)
	route, stops, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1},
	})
	require.NoError(t, err)
	_, err = svc.StartRoute(context.Background(), route.ID)
	require.NoError(t, err)
	_, _, err = svc.RecordVisit(context.Background(), collection.RecordVisitInput{
		StopID:        stops[0].ID,
		AmountMinor:   50000,
		PaymentMethod: collection.PaymentCash,
		RecordedBy:    colSalesRep,
	})
	require.NoError(t, err)
	_, err = svc.CloseStop(context.Background(), collection.CloseStopInput{StopID: stops[0].ID})
	require.NoError(t, err)
	_, err = svc.CompleteRoute(context.Background(), route.ID)
	require.NoError(t, err)

	// Pre-set route.total_collected_minor to 50000 (trigger-equivalent in fake).
	repo.mu.Lock()
	repo.routes[route.ID].TotalCollectedMinor = 50000
	repo.mu.Unlock()

	settlement, settledRoute, err := svc.SettleRoute(context.Background(), collection.SettleRouteInput{
		RouteID:            route.ID,
		SettledAmountMinor: 50000, // exact match
		RecordedBy:         colSalesRep,
	})
	require.NoError(t, err)
	assert.Equal(t, collection.SettlementApproved, settlement.Status)
	assert.Equal(t, int64(0), settlement.DiscrepancyMinor)
	assert.Equal(t, collection.RouteStatusSettled, settledRoute.Status)
}

// 13. SettleRoute discrepancy (under) → status pending.
func TestCollectionService_SettleRoute_Discrepancy_Pending(t *testing.T) {
	t.Parallel()
	svc, repo := newCollectionSvc(t, nil)
	route, stops, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1},
	})
	require.NoError(t, err)
	_, err = svc.StartRoute(context.Background(), route.ID)
	require.NoError(t, err)
	_, _, err = svc.RecordVisit(context.Background(), collection.RecordVisitInput{
		StopID:        stops[0].ID,
		AmountMinor:   50000,
		PaymentMethod: collection.PaymentCash,
		RecordedBy:    colSalesRep,
	})
	require.NoError(t, err)
	_, err = svc.CloseStop(context.Background(), collection.CloseStopInput{StopID: stops[0].ID})
	require.NoError(t, err)
	_, err = svc.CompleteRoute(context.Background(), route.ID)
	require.NoError(t, err)

	repo.mu.Lock()
	repo.routes[route.ID].TotalCollectedMinor = 50000
	repo.mu.Unlock()

	// Sales rep deposited only 48000 (20000 short).
	settlement, _, err := svc.SettleRoute(context.Background(), collection.SettleRouteInput{
		RouteID:            route.ID,
		SettledAmountMinor: 48000,
		RecordedBy:         colSalesRep,
		Notes:              "customer paid short, will follow up",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(-2000), settlement.DiscrepancyMinor)
	assert.Equal(t, collection.SettlementPending, settlement.Status)
}

// 14. ApproveSettlement: pending → approved.
func TestCollectionService_ApproveSettlement_Approve(t *testing.T) {
	t.Parallel()
	svc, repo := newCollectionSvc(t, nil)
	route, stops, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1},
	})
	require.NoError(t, err)
	_, err = svc.StartRoute(context.Background(), route.ID)
	require.NoError(t, err)
	_, _, err = svc.RecordVisit(context.Background(), collection.RecordVisitInput{
		StopID:        stops[0].ID,
		AmountMinor:   50000,
		PaymentMethod: collection.PaymentCash,
		RecordedBy:    colSalesRep,
	})
	require.NoError(t, err)
	_, err = svc.CloseStop(context.Background(), collection.CloseStopInput{StopID: stops[0].ID})
	require.NoError(t, err)
	_, err = svc.CompleteRoute(context.Background(), route.ID)
	require.NoError(t, err)
	repo.mu.Lock()
	repo.routes[route.ID].TotalCollectedMinor = 50000
	repo.mu.Unlock()
	settlement, _, err := svc.SettleRoute(context.Background(), collection.SettleRouteInput{
		RouteID:            route.ID,
		SettledAmountMinor: 48000,
		RecordedBy:         colSalesRep,
	})
	require.NoError(t, err)
	require.Equal(t, collection.SettlementPending, settlement.Status)

	approved, err := svc.ApproveSettlement(context.Background(), collection.ApproveSettlementInput{
		SettlementID: settlement.ID,
		ApproverID:   colApprover,
		Approve:      true,
		Notes:        "investigated, customer will pay remainder tomorrow",
	})
	require.NoError(t, err)
	assert.Equal(t, collection.SettlementApproved, approved.Status)
	assert.Equal(t, colApprover, approved.ApprovedBy)
}

// 15. ApproveSettlement: reject.
func TestCollectionService_ApproveSettlement_Reject(t *testing.T) {
	t.Parallel()
	svc, repo := newCollectionSvc(t, nil)
	route, stops, err := svc.PlanRoute(context.Background(), collection.PlanRouteInput{
		TenantID:    colTenant1,
		SalesRepID:  colSalesRep,
		RouteDate:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
		CustomerIDs: []string{colCustomer1},
	})
	require.NoError(t, err)
	_, err = svc.StartRoute(context.Background(), route.ID)
	require.NoError(t, err)
	_, _, err = svc.RecordVisit(context.Background(), collection.RecordVisitInput{
		StopID:        stops[0].ID,
		AmountMinor:   50000,
		PaymentMethod: collection.PaymentCash,
		RecordedBy:    colSalesRep,
	})
	require.NoError(t, err)
	_, err = svc.CloseStop(context.Background(), collection.CloseStopInput{StopID: stops[0].ID})
	require.NoError(t, err)
	_, err = svc.CompleteRoute(context.Background(), route.ID)
	require.NoError(t, err)
	repo.mu.Lock()
	repo.routes[route.ID].TotalCollectedMinor = 50000
	repo.mu.Unlock()
	settlement, _, err := svc.SettleRoute(context.Background(), collection.SettleRouteInput{
		RouteID:            route.ID,
		SettledAmountMinor: 30000, // big discrepancy
		RecordedBy:         colSalesRep,
	})
	require.NoError(t, err)
	require.Equal(t, collection.SettlementPending, settlement.Status)

	rejected, err := svc.ApproveSettlement(context.Background(), collection.ApproveSettlementInput{
		SettlementID: settlement.ID,
		ApproverID:   colApprover,
		Approve:      false,
		Notes:        "need further investigation",
	})
	require.NoError(t, err)
	assert.Equal(t, collection.SettlementRejected, rejected.Status)
}

// 16. GetRoute returns NotFound for unknown id.
func TestCollectionService_GetRoute_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newCollectionSvc(t, nil)
	_, err := svc.GetRoute(context.Background(), "00000000-0000-0000-0000-000000000xxx")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}
