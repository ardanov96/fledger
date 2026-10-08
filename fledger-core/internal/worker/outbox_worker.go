// Package worker — OutboxPublisherWorker runs the outbox publisher on a
// ticker-based schedule (Sprint 24 / Fase 4A).
//
// Pattern mirrors ReconcilerWorker (same ticker + immediate first run + graceful
// shutdown via context). Polls the outbox table, publishes events to NATS via
// usecase.OutboxPublisher, retries on failure.
//
// Why ticker-based:
//   - Same trade-offs as ReconcilerWorker
//   - Sub-second poll interval is fine for low-volume demo
//   - Easy to swap for SELECT ... FOR UPDATE SKIP LOCKED if multiple workers
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/runut/fmcg-wallet/internal/usecase"
)

// =============================================================================
// OutboxPublisherWorker
// =============================================================================

// OutboxPublisherWorker runs the outbox publisher periodically.
type OutboxPublisherWorker struct {
	publisher *usecase.OutboxPublisher
	log       *slog.Logger
	interval  time.Duration

	mu       sync.Mutex
	running  bool
	lastRun  time.Time
	lastErr  error
	lastPub  int
	cyclesOk int
	cyclesEr int
}

// OutboxPublisherWorkerDeps bundles dependencies.
type OutboxPublisherWorkerDeps struct {
	Publisher *usecase.OutboxPublisher
	Logger    *slog.Logger
	Interval  time.Duration // default 1s if zero
}

// NewOutboxPublisherWorker constructs the worker.
func NewOutboxPublisherWorker(deps OutboxPublisherWorkerDeps) *OutboxPublisherWorker {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	interval := deps.Interval
	if interval <= 0 {
		interval = 1 * time.Second
	}
	return &OutboxPublisherWorker{
		publisher: deps.Publisher,
		log:       log,
		interval:  interval,
	}
}

// Start launches the background loop. Cancel ctx to stop.
func (w *OutboxPublisherWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		w.log.Warn("outbox publisher worker already running; ignoring Start")
		return
	}
	w.running = true
	w.mu.Unlock()

	w.log.Info("outbox publisher worker starting", "interval", w.interval)
	go w.loop(ctx)
}

// RunOnce runs one publisher cycle immediately (for tests / manual trigger).
func (w *OutboxPublisherWorker) RunOnce(ctx context.Context) (int, error) {
	n, err := w.publisher.RunOnce(ctx)
	w.mu.Lock()
	w.lastRun = time.Now().UTC()
	w.lastPub = n
	if err != nil {
		w.lastErr = err
		w.cyclesEr++
	} else {
		w.cyclesOk++
	}
	w.mu.Unlock()
	return n, err
}

// Status returns the worker's last run state (for diagnostics).
func (w *OutboxPublisherWorker) Status() OutboxPublisherStatus {
	w.mu.Lock()
	defer w.mu.Unlock()
	return OutboxPublisherStatus{
		Running:   w.running,
		Interval:  w.interval,
		LastRun:   w.lastRun,
		LastErr:   w.lastErr,
		LastPub:   w.lastPub,
		CyclesOK:  w.cyclesOk,
		CyclesErr: w.cyclesEr,
	}
}

// OutboxPublisherStatus is a snapshot of the worker state.
type OutboxPublisherStatus struct {
	Running   bool
	Interval  time.Duration
	LastRun   time.Time
	LastErr   error
	LastPub   int
	CyclesOK  int
	CyclesErr int
}

func (w *OutboxPublisherWorker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			w.log.Info("outbox publisher worker stopped")
			return
		case <-ticker.C:
			if _, err := w.RunOnce(ctx); err != nil {
				w.log.Warn("outbox publisher cycle failed",
					"error", err.Error(),
				)
			}
		}
	}
}
