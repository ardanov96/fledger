// Package worker — ReconcilerWorker runs the trial-balance reconciler on a
// ticker-based schedule.
//
// Why ticker-based instead of cron / River / Temporal:
//   - No new dependencies (no River, no Temporal, no cron lib needed)
//   - Trivial to test
//   - Production-ready for low-volume periodic jobs (every N minutes)
//   - Easy graceful shutdown via context
//
// For higher scale, swap with River (Postgres-native durable scheduler).
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/runut/fmcg-wallet/internal/usecase"
)

// =============================================================================
// ReconcilerWorker
// =============================================================================

// ReconcilerWorker runs the trial-balance reconciler periodically.
type ReconcilerWorker struct {
	svc        *usecase.ReconcilerService
	log        *slog.Logger
	interval   time.Duration
	runHashChk bool

	mu       sync.Mutex
	running  bool
	lastRun  time.Time
	lastErr  error
	lastRuns map[string][]string // tenant → run IDs
}

// ReconcilerWorkerDeps bundles dependencies.
type ReconcilerWorkerDeps struct {
	Service            *usecase.ReconcilerService
	Logger             *slog.Logger
	Interval           time.Duration // default 1h if zero
	RunHashCheckOnCron bool          // whether to also verify hash chain on scheduled runs
}

// NewReconcilerWorker constructs a ReconcilerWorker.
func NewReconcilerWorker(deps ReconcilerWorkerDeps) *ReconcilerWorker {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	interval := deps.Interval
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	return &ReconcilerWorker{
		svc:        deps.Service,
		log:        log,
		interval:   interval,
		runHashChk: deps.RunHashCheckOnCron,
	}
}

// Start launches the background loop. Returns immediately. Cancel ctx to stop.
func (w *ReconcilerWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		w.log.Warn("reconciler worker already running; ignoring Start")
		return
	}
	w.running = true
	w.mu.Unlock()

	w.log.Info("reconciler worker starting",
		"interval", w.interval,
		"run_hash_check", w.runHashChk,
	)

	go w.loop(ctx)
}

// RunNow executes one reconciliation pass immediately (for manual trigger).
func (w *ReconcilerWorker) RunNow(ctx context.Context) (map[string][]string, error) {
	w.log.Info("reconciler worker: RunNow triggered")
	results, err := w.svc.RunAllForAllTenants(ctx, w.runHashChk)
	if err != nil {
		w.mu.Lock()
		w.lastErr = err
		w.mu.Unlock()
		return nil, err
	}
	w.mu.Lock()
	w.lastRun = time.Now().UTC()
	w.lastRuns = results
	w.mu.Unlock()
	return results, nil
}

// Status returns the worker's last run state (for diagnostics endpoint).
func (w *ReconcilerWorker) Status() Status {
	w.mu.Lock()
	defer w.mu.Unlock()
	return Status{
		Running:  w.running,
		Interval: w.interval,
		LastRun:  w.lastRun,
		LastErr:  w.lastErr,
		LastRuns: w.lastRuns,
	}
}

// Status is a snapshot of the worker state.
type Status struct {
	Running  bool
	Interval time.Duration
	LastRun  time.Time
	LastErr  error
	LastRuns map[string][]string
}

// loop is the main ticker loop. Runs reconciliation at every `interval`,
// with first run immediately on Start (skip if just-initialized).
func (w *ReconcilerWorker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run once immediately on Start.
	if _, err := w.RunNow(ctx); err != nil {
		w.log.Warn("reconciler initial run failed", "error", err.Error())
	}

	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			w.log.Info("reconciler worker stopped")
			return
		case <-ticker.C:
			if _, err := w.RunNow(ctx); err != nil {
				w.log.Warn("reconciler scheduled run failed", "error", err.Error())
			}
		}
	}
}

// String returns a human-readable summary (useful for logs).
func (s Status) String() string {
	errStr := ""
	if s.LastErr != nil {
		errStr = fmt.Sprintf(" last_err=%q", s.LastErr.Error())
	}
	last := "never"
	if !s.LastRun.IsZero() {
		last = s.LastRun.Format(time.RFC3339)
	}
	return fmt.Sprintf("running=%t interval=%s last_run=%s runs=%d%s",
		s.Running, s.Interval, last, totalRuns(s.LastRuns), errStr)
}

func totalRuns(m map[string][]string) int {
	if m == nil {
		return 0
	}
	n := 0
	for _, v := range m {
		n += len(v)
	}
	return n
}
