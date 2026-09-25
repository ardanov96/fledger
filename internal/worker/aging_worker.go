// Package worker — AgingWorker runs the AgingRecalculator on a ticker.
//
// Pattern mirrors OutboxPublisherWorker and ReconcilerWorker: ticker-based,
// graceful shutdown via context, immediate first run on Start.
//
// Default interval is 24h (nightly) — aging is bounded-fresh by design.
// Override via AGING_RECALC_INTERVAL env (e.g. "1h" for demo, "5m" for tests).
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/runut/fmcg-wallet/internal/usecase"
)

// =============================================================================
// AgingWorker
// =============================================================================

// AgingWorker runs the aging recalculator periodically.
type AgingWorker struct {
	svc      *usecase.AgingRecalculator
	log      *slog.Logger
	interval time.Duration

	mu       sync.Mutex
	running  bool
	lastRun  time.Time
	lastRes  usecase.RecalcResult
	lastErr  error
	cyclesOk int
	cyclesEr int
}

// AgingWorkerDeps bundles dependencies.
type AgingWorkerDeps struct {
	Service  *usecase.AgingRecalculator
	Logger   *slog.Logger
	Interval time.Duration // default 24h if zero
}

// NewAgingWorker constructs the worker.
func NewAgingWorker(deps AgingWorkerDeps) *AgingWorker {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	interval := deps.Interval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &AgingWorker{
		svc:      deps.Service,
		log:      log,
		interval: interval,
	}
}

// Start launches the background loop. Cancel ctx to stop.
func (w *AgingWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		w.log.Warn("aging worker already running; ignoring Start")
		return
	}
	w.running = true
	w.mu.Unlock()

	w.log.Info("aging worker starting", "interval", w.interval)
	go w.loop(ctx)
}

// RunNow executes one recalc cycle immediately (for tests / manual trigger).
func (w *AgingWorker) RunNow(ctx context.Context) (usecase.RecalcResult, error) {
	res, err := w.svc.RunForAllTenants(ctx)
	w.mu.Lock()
	w.lastRun = time.Now().UTC()
	w.lastRes = res
	if err != nil {
		w.lastErr = err
		w.cyclesEr++
	} else {
		w.cyclesOk++
	}
	w.mu.Unlock()
	return res, err
}

// Status returns the worker's last run state (for diagnostics).
func (w *AgingWorker) Status() AgingStatus {
	w.mu.Lock()
	defer w.mu.Unlock()
	return AgingStatus{
		Running:   w.running,
		Interval:  w.interval,
		LastRun:   w.lastRun,
		LastRes:   w.lastRes,
		LastErr:   w.lastErr,
		CyclesOK:  w.cyclesOk,
		CyclesErr: w.cyclesEr,
	}
}

// AgingStatus is a snapshot of the worker state.
type AgingStatus struct {
	Running   bool
	Interval  time.Duration
	LastRun   time.Time
	LastRes   usecase.RecalcResult
	LastErr   error
	CyclesOK  int
	CyclesErr int
}

func (w *AgingWorker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			w.log.Info("aging worker stopped")
			return
		case <-ticker.C:
			if _, err := w.RunNow(ctx); err != nil {
				w.log.Warn("aging scheduled run failed", "error", err.Error())
			}
		}
	}
}
