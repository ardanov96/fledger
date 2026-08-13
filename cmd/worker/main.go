// Package main is the background worker entrypoint for FMCG Wallet.
//
// The worker hosts long-running jobs:
//   - ReconcilerWorker (Sprint 10 — Fase 1B) — ticker-based trial balance reconciler
//   - asynq scheduled tasks (aging recalculation, notifications, fraud scan) — Fase 4+
//   - NATS JetStream consumers (write-side fanout, projections) — Fase 4+
//   - Outbox publisher (transactional outbox pattern) — Fase 4A
//
// Workers ditambahkan bertahap seiring roadmap. Saat ini hanya ReconcilerWorker
// yang aktif (Sprint 10); worker lain menyusul di Fase 4+.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/domain/reconciler"
	"github.com/runut/fmcg-wallet/internal/infra"
	"github.com/runut/fmcg-wallet/internal/platform/config"
	"github.com/runut/fmcg-wallet/internal/platform/logger"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
	"github.com/runut/fmcg-wallet/internal/usecase"
	"github.com/runut/fmcg-wallet/internal/worker"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(logger.Config{
		Level:  cfg.App.LogLevel,
		Format: cfg.App.LogFormat,
	})
	slog.SetDefault(log)
	log.Info("starting fmcg-wallet worker", "env", cfg.App.Env)

	// -------------------------------------------------------------------------
	// Active workers (Sprint 10)
	// -------------------------------------------------------------------------

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wire ReconcilerWorker (Sprint 10 — Fase 1B).
	// Same wiring as cmd/api/main.go line 113-127, but without the API
	// surface — worker process is dedicated to background jobs.
	if err := wireReconcilerWorker(ctx, cfg, log); err != nil {
		return fmt.Errorf("wire reconciler worker: %w", err)
	}

	// -------------------------------------------------------------------------
	// Future workers (placeholders)
	// -------------------------------------------------------------------------
	// TODO Fase 4: aging_recalculator (scheduled nightly)
	// TODO Fase 4: outbox_publisher (continuous)
	// TODO Fase 8: notification_dispatcher
	// TODO Fase 8: fraud_flag_scanner

	// Heartbeat so the process doesn't exit (for debugging idle state).
	go heartbeat(ctx, log)

	// Wait for shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-shutdownCh
	log.Info("worker shutdown signal received", "signal", sig.String())
	cancel()

	// Give in-flight work a moment to drain (reconciler ticker loop will exit
	// via ctx, in-flight RunNow call finishes up to ~30s).
	time.Sleep(2 * time.Second)
	log.Info("worker stopped")
	return nil
}

// wireReconcilerWorker connects to DB and starts the ReconcilerWorker.
func wireReconcilerWorker(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	pool, err := infra.NewPGXPool(ctx, &cfg.DB)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	// Note: pool is intentionally NOT closed here — the worker runs for the
	// lifetime of the process and uses it on every tick.

	db := postgres.NewDB(pool)
	reconcilerRepo := postgres.NewReconcilerRepository(db)
	periodRepo := postgres.NewPeriodRepository(db)
	entryRepo := postgres.NewEntryRepository(db)

	recTx := &reconcilerTxAdapter{db: db}
	ledgerProbe := &ledgerProbeAdapter{
		periodRepo: periodRepo,
		entryRepo:  entryRepo,
	}
	hashChainRunner := &hashChainAdapter{verifier: usecase.NewVerifier(log)}
	reconcilerService := usecase.NewReconcilerService(usecase.ReconcilerServiceDeps{
		Repo:   reconcilerRepo,
		Ledger: ledgerProbe,
		Hasher: hashChainRunner,
		DB:     recTx,
		Logger: log,
	})

	// Hash chain check on cron is gated by env (expensive for large datasets).
	runHashCheck := os.Getenv("RECONCILER_HASH_CHECK") == "true"

	// Interval configurable via env, default 1 hour (matches cmd/api).
	interval := 1 * time.Hour
	if d := os.Getenv("RECONCILER_INTERVAL"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil && parsed > 0 {
			interval = parsed
		}
	}

	reconcilerWorker := worker.NewReconcilerWorker(worker.ReconcilerWorkerDeps{
		Service:            reconcilerService,
		Logger:             log,
		Interval:           interval,
		RunHashCheckOnCron: runHashCheck,
	})
	reconcilerWorker.Start(ctx)
	log.Info("reconciler worker started",
		"interval", interval,
		"hash_check_on_cron", runHashCheck,
	)
	return nil
}

func heartbeat(ctx context.Context, log *slog.Logger) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			log.Debug("worker heartbeat", "goroutines", 1, "uptime", time.Since(startedAt).String())
		}
	}
}

// -------------------------------------------------------------------------
// Reconciler adapter shims — duplicated from cmd/api/reconciler_adapters.go
// to keep worker binary self-contained (avoid importing cmd/api package).
// -------------------------------------------------------------------------

type reconcilerTxAdapter struct {
	db *postgres.DB
}

func (a *reconcilerTxAdapter) ExecuteTx(ctx context.Context, fn func(reconciler.Tx) error) error {
	return a.db.RunInTxReconcilerDomain(ctx, fn)
}

type ledgerProbeAdapter struct {
	periodRepo *postgres.PeriodRepository
	entryRepo  *postgres.EntryRepository
}

func (a *ledgerProbeAdapter) TrialBalance(ctx context.Context, tx reconciler.Tx, periodID string) (int64, int64, int64, error) {
	const q = `
SELECT
    COALESCE(SUM(CASE WHEN type = 'debit'  THEN amount ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN type = 'credit' THEN amount ELSE 0 END), 0)
FROM ledger_entries
WHERE period_id = $1
`
	pgxTx, err := postgres.UnwrapPgxTxFromReconciler(tx)
	if err != nil {
		return 0, 0, 0, err
	}
	var td, tc int64
	if err := pgxTx.QueryRow(ctx, q, periodID).Scan(&td, &tc); err != nil {
		return 0, 0, 0, fmt.Errorf("trial balance query: %w", err)
	}
	return td, tc, td - tc, nil
}

func (a *ledgerProbeAdapter) AccountBalanceAtPeriod(ctx context.Context, tx reconciler.Tx, accountID, periodID string) (int64, int64, int64, int, error) {
	const q = `
SELECT
    COALESCE(SUM(CASE WHEN type = 'debit'  THEN amount ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN type = 'credit' THEN amount ELSE 0 END), 0),
    COUNT(*)
FROM ledger_entries
WHERE account_id = $1 AND period_id = $2
`
	pgxTx, err := postgres.UnwrapPgxTxFromReconciler(tx)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	var d, c int64
	var cnt int
	if err := pgxTx.QueryRow(ctx, q, accountID, periodID).Scan(&d, &c, &cnt); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("account balance query: %w", err)
	}
	return d, c, d - c, cnt, nil
}

func (a *ledgerProbeAdapter) ListEntriesByPeriod(ctx context.Context, periodID string) ([]ledger.Entry, error) {
	return a.entryRepo.ListByPeriod(ctx, periodID)
}

type hashChainAdapter struct {
	verifier *usecase.Verifier
}

func (a *hashChainAdapter) VerifyEntries(ctx context.Context, entries []ledger.Entry) []error {
	return a.verifier.Verify(ctx, entries)
}

// Compile-time guards — ensures our adapters satisfy the usecase interfaces.
var (
	_ usecase.LedgerProbe        = (*ledgerProbeAdapter)(nil)
	_ usecase.ReconcilerTxRunner = (*reconcilerTxAdapter)(nil)
	_ usecase.HashChainRunner    = (*hashChainAdapter)(nil)
)

var startedAt = time.Now()
