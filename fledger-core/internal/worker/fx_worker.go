// Package worker — FxRateWorker runs the FxRateRefresher on a ticker.
//
// Pattern mirrors OutboxPublisherWorker / AgingWorker: ticker-based,
// graceful shutdown via context, immediate first run on Start.
//
// Default interval is 1h (configurable via FX_REFRESH_INTERVAL env).
// Pairs come from cfg.FX.Pairs (parsed from FX_PAIRS env at startup).
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// =============================================================================
// FxRateWorker
// =============================================================================

// FxRateWorker runs the FX rate refresher periodically.
type FxRateWorker struct {
	refresher *usecase.FxRateRefresher
	pairs     []currency.CurrencyPair
	log       *slog.Logger
	interval  time.Duration

	mu       sync.Mutex
	running  bool
	lastRun  time.Time
	lastRes  usecase.FxRefreshResult
	lastErr  error
	cyclesOk int
	cyclesEr int
}

// FxRateWorkerDeps bundles dependencies.
type FxRateWorkerDeps struct {
	Refresher *usecase.FxRateRefresher
	Pairs     []currency.CurrencyPair // parsed at startup
	Logger    *slog.Logger
	Interval  time.Duration // default 1h if zero
}

// NewFxRateWorker constructs the worker.
func NewFxRateWorker(deps FxRateWorkerDeps) *FxRateWorker {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	interval := deps.Interval
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	return &FxRateWorker{
		refresher: deps.Refresher,
		pairs:     deps.Pairs,
		log:       log,
		interval:  interval,
	}
}

// Start launches the background loop. Cancel ctx to stop.
func (w *FxRateWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		w.log.Warn("fx rate worker already running; ignoring Start")
		return
	}
	w.running = true
	w.mu.Unlock()

	w.log.Info("fx rate worker starting",
		"interval", w.interval,
		"pairs", len(w.pairs),
	)
	go w.loop(ctx)
}

// RunNow executes one refresh cycle immediately (for tests / manual trigger).
// Pass nil tenantIDs to let the refresher fetch its own list.
func (w *FxRateWorker) RunNow(ctx context.Context) (usecase.FxRefreshResult, error) {
	res, err := w.refresher.RefreshOnce(ctx, nil, w.pairs)
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

// Status returns the worker's last run state.
func (w *FxRateWorker) Status() FxRateWorkerStatus {
	w.mu.Lock()
	defer w.mu.Unlock()
	return FxRateWorkerStatus{
		Running:   w.running,
		Interval:  w.interval,
		LastRun:   w.lastRun,
		LastRes:   w.lastRes,
		LastErr:   w.lastErr,
		CyclesOK:  w.cyclesOk,
		CyclesErr: w.cyclesEr,
	}
}

// FxRateWorkerStatus is a snapshot of the worker state.
type FxRateWorkerStatus struct {
	Running   bool
	Interval  time.Duration
	LastRun   time.Time
	LastRes   usecase.FxRefreshResult
	LastErr   error
	CyclesOK  int
	CyclesErr int
}

func (w *FxRateWorker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			w.log.Info("fx rate worker stopped")
			return
		case <-ticker.C:
			if _, err := w.RunNow(ctx); err != nil {
				w.log.Warn("fx scheduled refresh failed", "error", err.Error())
			}
		}
	}
}
