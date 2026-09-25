// Package usecase — AgingRecalculator populates the aging snapshot table.
//
// Algorithm (Sprint 25 / Fase 4D):
//  1. Start a run (write row in aging_snapshot_runs).
//  2. List distinct (tenant_id, customer_id) pairs with outstanding invoices.
//  3. For each customer:
//     a. Query v_invoice_aging for the 6 buckets (current, d_1_7, ..., d_90_plus).
//     b. Build AgingSnapshot rows.
//  4. Bulk-upsert all snapshots under the run ID.
//  5. Finish the run with counts + duration + status.
//
// Failure handling:
//   - Run is marked 'failed' on any error; partial upserts are rolled back
//     by the single tx in UpsertAgingSnapshots.
//   - Each customer is processed sequentially for now (MVP). For large
//     tenants, parallelism can be added later with per-customer tx.
//
// Frequency:
//   - Worker (cmd/worker) calls RunForAllTenants on a nightly ticker
//     (AGING_RECALC_INTERVAL, default 24h).
//   - API reads from aging_snapshots; falls back to live view if snapshot
//     is missing (worker hasn't run yet).
package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/invoice"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
)

// AgingRepoLookup abstracts the live aging query (used during recalc).
// Implementation: wraps InvoiceRepository.GetAging (live view).
type AgingRepoLookup interface {
	GetAging(ctx context.Context, tenantID, customerID string) ([]invoice.AgingSummary, error)
}

// AgingRecalculator populates aging_snapshots from the live aging view.
type AgingRecalculator struct {
	customersRepo invoice.AgingSnapshotRepository
	liveAging     AgingRepoLookup
	log           *slog.Logger
	now           func() time.Time // injectable for tests
}

// AgingRecalculatorDeps bundles dependencies.
type AgingRecalculatorDeps struct {
	CustomersRepo invoice.AgingSnapshotRepository // List + Upsert + Run lifecycle
	LiveAging     AgingRepoLookup                 // GetAging (live view) for per-customer read
	Logger        *slog.Logger
	NowFunc       func() time.Time // optional; defaults to time.Now UTC
}

// NewAgingRecalculator constructs an AgingRecalculator.
func NewAgingRecalculator(deps AgingRecalculatorDeps) *AgingRecalculator {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	now := deps.NowFunc
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &AgingRecalculator{
		customersRepo: deps.CustomersRepo,
		liveAging:     deps.LiveAging,
		log:           log,
		now:           now,
	}
}

// RecalcResult summarizes one run.
type RecalcResult struct {
	RunID              string
	TenantsProcessed   int
	CustomersProcessed int
	RowsWritten        int
	DurationMs         int64
}

// RunForAllTenants executes one full recalculation cycle.
func (r *AgingRecalculator) RunForAllTenants(ctx context.Context) (RecalcResult, error) {
	start := r.now()
	runID, err := r.customersRepo.StartRun(ctx)
	if err != nil {
		return RecalcResult{}, fmt.Errorf("start run: %w", err)
	}

	res := RecalcResult{RunID: runID}

	customers, err := r.customersRepo.ListCustomersWithOutstanding(ctx)
	if err != nil {
		r.markFailed(ctx, runID, res, err)
		return res, fmt.Errorf("list customers: %w", err)
	}

	// Track distinct tenants we touched (for the result stat).
	tenantSet := make(map[string]struct{}, 8)

	snapshots := make([]invoice.AgingSnapshot, 0, len(customers)*6)
	snapshotAt := start

	for _, c := range customers {
		// Per-customer lookup (live view)
		buckets, err := r.liveAging.GetAging(ctx, c.TenantID, c.CustomerID)
		if err != nil {
			r.markFailed(ctx, runID, res, err)
			return res, fmt.Errorf("live aging for %s/%s: %w", c.TenantID, c.CustomerID, err)
		}

		tenantSet[c.TenantID] = struct{}{}
		res.CustomersProcessed++

		for _, b := range buckets {
			snapshots = append(snapshots, invoice.AgingSnapshot{
				TenantID:         c.TenantID,
				CustomerID:       c.CustomerID,
				Bucket:           b.Bucket,
				Count:            b.Count,
				OutstandingMinor: b.OutstandingMinor,
				SnapshotAt:       snapshotAt,
				SnapshotRunID:    runID,
			})
			res.RowsWritten++
		}
	}

	res.TenantsProcessed = len(tenantSet)

	if err := r.customersRepo.UpsertAgingSnapshots(ctx, runID, snapshots); err != nil {
		r.markFailed(ctx, runID, res, err)
		return res, fmt.Errorf("upsert snapshots: %w", err)
	}

	res.DurationMs = time.Since(start).Milliseconds()
	if err := r.customersRepo.FinishRun(ctx, runID, invoice.RunStatusOK,
		res.TenantsProcessed, res.CustomersProcessed, res.RowsWritten,
		res.DurationMs, "",
	); err != nil {
		r.log.Warn("aging recalc: finish run failed (run may show as 'running' in audit)",
			"run_id", runID, "error", err)
	}

	r.log.Info("aging recalc complete",
		"run_id", runID,
		"tenants", res.TenantsProcessed,
		"customers", res.CustomersProcessed,
		"rows", res.RowsWritten,
		"duration_ms", res.DurationMs,
	)
	return res, nil
}

// markFailed records the failure on the run row. Best-effort: log on error
// but return the original error to the caller.
func (r *AgingRecalculator) markFailed(ctx context.Context, runID string, res RecalcResult, origErr error) {
	durMs := time.Since(r.now().Add(-time.Millisecond)).Milliseconds()
	errMsg := origErr.Error()
	if len(errMsg) > 500 {
		errMsg = errMsg[:500]
	}
	if err := r.customersRepo.FinishRun(ctx, runID, invoice.RunStatusFailed,
		res.TenantsProcessed, res.CustomersProcessed, res.RowsWritten,
		durMs, errMsg,
	); err != nil {
		r.log.Error("aging recalc: mark failed errored",
			"run_id", runID,
			"original_error", origErr,
			"mark_error", err,
		)
	}
}

// =============================================================================
// TenantSnapshotService — read-through cache for the API
// =============================================================================
//
// Reads from aging_snapshots first; falls back to the live view if the
// snapshot is empty (worker hasn't run yet) or fails.
//
// Used by handler.GetCustomerAging in cmd/api.
type TenantSnapshotService struct {
	customersRepo invoice.AgingSnapshotRepository
	liveAging     AgingRepoLookup
	log           *slog.Logger
}

// NewTenantSnapshotService constructs the read-through service.
func NewTenantSnapshotService(repo invoice.AgingSnapshotRepository, live AgingRepoLookup, log *slog.Logger) *TenantSnapshotService {
	if log == nil {
		log = slog.Default()
	}
	return &TenantSnapshotService{customersRepo: repo, liveAging: live, log: log}
}

// GetAgingSnapshot reads from snapshot first, falls back to live view.
func (s *TenantSnapshotService) GetAgingSnapshot(ctx context.Context, tenantID, customerID string) ([]invoice.AgingSummary, bool, error) {
	snap, err := s.customersRepo.GetAgingSnapshot(ctx, tenantID, customerID)
	if err != nil && !isAgingNotFound(err) {
		s.log.Warn("snapshot read failed, falling back to live view",
			"tenant_id", tenantID, "customer_id", customerID, "error", err)
	}
	if len(snap) > 0 {
		return snap, true, nil
	}

	// Fallback to live view (no snapshot yet or empty result)
	live, lerr := s.liveAging.GetAging(ctx, tenantID, customerID)
	if lerr != nil {
		if lerr == apperrors.ErrNotFound || isAgingNotFound(lerr) {
			return nil, false, nil
		}
		return nil, false, lerr
	}
	return live, false, nil
}

func isAgingNotFound(err error) bool {
	if err == nil {
		return false
	}
	// Tenant might not have any outstanding invoices → empty result, not error.
	// Treat "no rows" as not-found for our purposes.
	return err == apperrors.ErrNotFound || err.Error() == "no rows in result set"
}

// guard for uuid import in case Go's unused-import check fires
var _ = uuid.Nil
