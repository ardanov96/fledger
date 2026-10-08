// aging_recalculator_test.go — Sprint 25 / Fase 4D unit tests.
package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/invoice"
)

// =============================================================================
// Mocks
// =============================================================================

type fakeAgingSnapshotRepo struct {
	mu          sync.Mutex
	snapshots   map[string][]invoice.AgingSnapshot // runID → rows
	runs        map[string]*invoice.SnapshotRun
	upsertErr   error
	startErr    error
	listErr     error
	finishErr   error
	upsertCalls int
}

func newFakeAgingRepo() *fakeAgingSnapshotRepo {
	return &fakeAgingSnapshotRepo{
		snapshots: make(map[string][]invoice.AgingSnapshot),
		runs:      make(map[string]*invoice.SnapshotRun),
	}
}

func (r *fakeAgingSnapshotRepo) UpsertAgingSnapshots(_ context.Context, runID string, snapshots []invoice.AgingSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.upsertCalls++
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.snapshots[runID] = snapshots
	return nil
}

func (r *fakeAgingSnapshotRepo) GetAgingSnapshot(_ context.Context, tenantID, customerID string) ([]invoice.AgingSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Look up snapshots across all runs for this (tenant, customer).
	var out []invoice.AgingSummary
	for _, rows := range r.snapshots {
		for _, row := range rows {
			if row.TenantID == tenantID && row.CustomerID == customerID {
				out = append(out, invoice.AgingSummary{
					TenantID:         row.TenantID,
					CustomerID:       row.CustomerID,
					Bucket:           row.Bucket,
					Count:            row.Count,
					OutstandingMinor: row.OutstandingMinor,
				})
			}
		}
	}
	return out, nil
}

func (r *fakeAgingSnapshotRepo) ListCustomersWithOutstanding(_ context.Context) ([]invoice.CustomerRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listErr != nil {
		return nil, r.listErr
	}
	// Default: 2 distinct customers in 1 tenant
	return []invoice.CustomerRef{
		{TenantID: "11111111-1111-1111-1111-111111111111", CustomerID: "22222222-2222-2222-2222-222222222222"},
		{TenantID: "11111111-1111-1111-1111-111111111111", CustomerID: "33333333-3333-3333-3333-333333333333"},
	}, nil
}

func (r *fakeAgingSnapshotRepo) StartRun(_ context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.startErr != nil {
		return "", r.startErr
	}
	id := uuid.NewString()
	r.runs[id] = &invoice.SnapshotRun{ID: id, StartedAt: time.Now().UTC(), Status: invoice.RunStatusRunning}
	return id, nil
}

func (r *fakeAgingSnapshotRepo) FinishRun(_ context.Context, runID string, status invoice.RunStatus,
	tenants, customers, rows int, durationMs int64, errMsg string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finishErr != nil {
		return r.finishErr
	}
	run, ok := r.runs[runID]
	if !ok {
		return errors.New("run not found")
	}
	now := time.Now().UTC()
	run.FinishedAt = &now
	run.Status = status
	run.TenantsProcessed = tenants
	run.CustomersProcessed = customers
	run.RowsWritten = rows
	run.DurationMs = durationMs
	run.Error = errMsg
	return nil
}

func (r *fakeAgingSnapshotRepo) LatestRun(_ context.Context) (*invoice.SnapshotRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, run := range r.runs {
		return run, nil
	}
	return nil, nil
}

type fakeLiveAging struct {
	mu     sync.Mutex
	byCustomer map[string][]invoice.AgingSummary // "tenantID|customerID" → buckets
	err    error
	calls  int
}

func (f *fakeLiveAging) GetAging(_ context.Context, tenantID, customerID string) ([]invoice.AgingSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	key := tenantID + "|" + customerID
	if v, ok := f.byCustomer[key]; ok {
		return v, nil
	}
	// Default: 3 buckets for unknown customer
	return []invoice.AgingSummary{
		{TenantID: tenantID, CustomerID: customerID, Bucket: invoice.BucketCurrent, Count: 1, OutstandingMinor: 100_000},
		{TenantID: tenantID, CustomerID: customerID, Bucket: invoice.BucketD1To7, Count: 2, OutstandingMinor: 50_000},
		{TenantID: tenantID, CustomerID: customerID, Bucket: invoice.BucketD90Plus, Count: 1, OutstandingMinor: 25_000},
	}, nil
}

func newTestAgingRecalculator(repo *fakeAgingSnapshotRepo, live *fakeLiveAging) *AgingRecalculator {
	return NewAgingRecalculator(AgingRecalculatorDeps{
		CustomersRepo: repo,
		LiveAging:     live,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// =============================================================================
// Tests
// =============================================================================

func TestAgingRecalculator_RunForAllTenants_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeAgingRepo()
	live := &fakeLiveAging{
		byCustomer: map[string][]invoice.AgingSummary{
			"11111111-1111-1111-1111-111111111111|22222222-2222-2222-2222-222222222222": {
				{TenantID: "11111111-1111-1111-1111-111111111111", CustomerID: "22222222-2222-2222-2222-222222222222", Bucket: invoice.BucketCurrent, Count: 5, OutstandingMinor: 500_000},
				{TenantID: "11111111-1111-1111-1111-111111111111", CustomerID: "22222222-2222-2222-2222-222222222222", Bucket: invoice.BucketD8To30, Count: 1, OutstandingMinor: 100_000},
			},
		},
	}
	r := newTestAgingRecalculator(repo, live)

	res, err := r.RunForAllTenants(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, res.RunID)
	assert.Equal(t, 2, res.CustomersProcessed, "both customers processed")
	assert.Equal(t, 1, res.TenantsProcessed, "both customers share one tenant")
	assert.Greater(t, res.RowsWritten, 0, "rows written")
	assert.Greater(t, res.DurationMs, int64(-1))
	assert.Equal(t, 1, repo.upsertCalls, "UpsertAgingSnapshots called once")
	assert.Equal(t, 2, live.calls, "live view queried per customer")
}

func TestAgingRecalculator_RunForAllTenants_StartRunFails(t *testing.T) {
	t.Parallel()
	repo := newFakeAgingRepo()
	repo.startErr = errors.New("start failed")
	r := newTestAgingRecalculator(repo, &fakeLiveAging{})

	_, err := r.RunForAllTenants(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start run")
	assert.Equal(t, 0, repo.upsertCalls, "no upsert when start fails")
}

func TestAgingRecalculator_RunForAllTenants_ListCustomersFails(t *testing.T) {
	t.Parallel()
	repo := newFakeAgingRepo()
	repo.listErr = errors.New("list failed")
	r := newTestAgingRecalculator(repo, &fakeLiveAging{})

	_, err := r.RunForAllTenants(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list customers")
}

func TestAgingRecalculator_RunForAllTenants_LiveAgingFails(t *testing.T) {
	t.Parallel()
	repo := newFakeAgingRepo()
	live := &fakeLiveAging{}
	live.err = errors.New("live view failed")
	r := newTestAgingRecalculator(repo, live)

	_, err := r.RunForAllTenants(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "live aging")
}

func TestAgingRecalculator_RunForAllTenants_UpsertFails(t *testing.T) {
	t.Parallel()
	repo := newFakeAgingRepo()
	repo.upsertErr = errors.New("upsert failed")
	r := newTestAgingRecalculator(repo, &fakeLiveAging{})

	_, err := r.RunForAllTenants(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert snapshots")
}

func TestAgingRecalculator_RunForAllTenants_SnapshotHasCorrectRunID(t *testing.T) {
	t.Parallel()
	repo := newFakeAgingRepo()
	r := newTestAgingRecalculator(repo, &fakeLiveAging{})

	res, err := r.RunForAllTenants(context.Background())
	require.NoError(t, err)

	rows, ok := repo.snapshots[res.RunID]
	require.True(t, ok, "snapshot rows stored under the run id")
	require.NotEmpty(t, rows)
	for _, row := range rows {
		assert.Equal(t, res.RunID, row.SnapshotRunID, "all rows tagged with run id")
	}
}

// =============================================================================
// TenantSnapshotService — read-through cache
// =============================================================================

func TestTenantSnapshotService_GetAgingSnapshot_FromSnapshot(t *testing.T) {
	t.Parallel()
	repo := newFakeAgingRepo()
	// Pre-populate one snapshot row via Upsert (so the fake's GetAgingSnapshot finds it).
	tid, cid := uuid.New().String(), uuid.New().String()
	runID := uuid.NewString()
	require.NoError(t, repo.UpsertAgingSnapshots(context.Background(), runID, []invoice.AgingSnapshot{
		{TenantID: tid, CustomerID: cid, Bucket: invoice.BucketCurrent, Count: 7, OutstandingMinor: 700_000, SnapshotRunID: runID},
	}))

	// Live view would return different data — verify the snapshot wins.
	live := &fakeLiveAging{byCustomer: map[string][]invoice.AgingSummary{
		tid + "|" + cid: {{Bucket: invoice.BucketCurrent, Count: 99, OutstandingMinor: 999}},
	}}
	svc := NewTenantSnapshotService(repo, live, slog.New(slog.NewTextHandler(io.Discard, nil)))

	out, fromSnap, err := svc.GetAgingSnapshot(context.Background(), tid, cid)
	require.NoError(t, err)
	assert.True(t, fromSnap, "data should come from snapshot")
	assert.Len(t, out, 1)
	assert.Equal(t, invoice.BucketCurrent, out[0].Bucket)
	assert.Equal(t, int(7), out[0].Count, "snapshot value wins over live")
}

func TestTenantSnapshotService_GetAgingSnapshot_FallbackToLive(t *testing.T) {
	t.Parallel()
	repo := newFakeAgingRepo() // empty
	live := &fakeLiveAging{
		byCustomer: map[string][]invoice.AgingSummary{
			"t1|c1": {
				{TenantID: "t1", CustomerID: "c1", Bucket: invoice.BucketD8To30, Count: 2, OutstandingMinor: 200_000},
			},
		},
	}
	svc := NewTenantSnapshotService(repo, live, slog.New(slog.NewTextHandler(io.Discard, nil)))

	out, fromSnap, err := svc.GetAgingSnapshot(context.Background(), "t1", "c1")
	require.NoError(t, err)
	assert.False(t, fromSnap, "data should fall back to live view")
	assert.Len(t, out, 1)
	assert.Equal(t, invoice.BucketD8To30, out[0].Bucket)
}

func TestTenantSnapshotService_GetAgingSnapshot_NoDataAnywhere(t *testing.T) {
	t.Parallel()
	repo := newFakeAgingRepo()
	// Empty live: configure byCustomer with empty slice so live returns empty (not default 3 buckets).
	live := &fakeLiveAging{byCustomer: map[string][]invoice.AgingSummary{
		"t1|c1": {}, // explicitly empty
	}}
	svc := NewTenantSnapshotService(repo, live, slog.New(slog.NewTextHandler(io.Discard, nil)))

	out, fromSnap, err := svc.GetAgingSnapshot(context.Background(), "t1", "c1")
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.False(t, fromSnap)
}
