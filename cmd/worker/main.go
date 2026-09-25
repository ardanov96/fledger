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

	"github.com/shopspring/decimal"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/domain/reconciler"
	"github.com/runut/fmcg-wallet/internal/infra"
	"github.com/runut/fmcg-wallet/internal/infra/fxprovider"
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
	// Active workers (Sprint 10 + Sprint 24)
	// -------------------------------------------------------------------------

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wire ReconcilerWorker (Sprint 10 — Fase 1B).
	// Same wiring as cmd/api/main.go line 113-127, but without the API
	// surface — worker process is dedicated to background jobs.
	if err := wireReconcilerWorker(ctx, cfg, log); err != nil {
		return fmt.Errorf("wire reconciler worker: %w", err)
	}

	// Wire OutboxPublisherWorker (Sprint 24 — Fase 4A).
	// Polls outbox_events, publishes to NATS, marks rows as published.
	if err := wireOutboxWorker(ctx, cfg, log); err != nil {
		return fmt.Errorf("wire outbox worker: %w", err)
	}

	// Wire AgingWorker (Sprint 25 — Fase 4D).
	// Nightly recalculation of aging_snapshots table from live v_invoice_aging.
	if err := wireAgingWorker(ctx, cfg, log); err != nil {
		return fmt.Errorf("wire aging worker: %w", err)
	}

	// Wire FxRateWorker (Sprint 26 — Fase 1D follow-up).
	// Periodically refreshes FX rates from configured provider.
	if err := wireFxRateWorker(ctx, cfg, log); err != nil {
		return fmt.Errorf("wire fx rate worker: %w", err)
	}

	// -------------------------------------------------------------------------
	// Future workers (placeholders)
	// -------------------------------------------------------------------------
	// TODO Fase 8: notification_dispatcher (subscribe to NATS outbox events)
	// TODO Fase 8: fraud_flag_scanner (subscribe to NATS outbox events)

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

// wireOutboxWorker connects to DB + NATS and starts the OutboxPublisherWorker.
// Sprint 24 / Fase 4A.
func wireOutboxWorker(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	pool, err := infra.NewPGXPool(ctx, &cfg.DB)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	db := postgres.NewDB(pool)
	outboxRepo := postgres.NewOutboxRepository(db)

	// Interval configurable via env, default 1 second.
	interval := 1 * time.Second
	if d := os.Getenv("OUTBOX_PUBLISHER_INTERVAL"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil && parsed > 0 {
			interval = parsed
		}
	}

	// Batch size configurable via env, default 50.
	batchSize := 50
	if s := os.Getenv("OUTBOX_PUBLISHER_BATCH"); s != "" {
		fmt.Sscanf(s, "%d", &batchSize)
	}

	natsClient, err := infra.NewNATSClient(ctx, cfg.NATS, log)
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}

	publisher := usecase.NewOutboxPublisher(usecase.OutboxPublisherDeps{
		Repo:      outboxRepo,
		Broker:    natsClient,
		Logger:    log,
		BatchSize: batchSize,
	})

	outboxWorker := worker.NewOutboxPublisherWorker(worker.OutboxPublisherWorkerDeps{
		Publisher: publisher,
		Logger:    log,
		Interval:  interval,
	})
	outboxWorker.Start(ctx)
	log.Info("outbox publisher worker started",
		"interval", interval,
		"batch_size", batchSize,
		"nats_url", cfg.NATS.URL,
	)
	return nil
}

// wireAgingWorker starts the nightly AgingWorker (Sprint 25 / Fase 4D).
func wireAgingWorker(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	pool, err := infra.NewPGXPool(ctx, &cfg.DB)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	db := postgres.NewDB(pool)
	invoiceRepo := postgres.NewInvoiceRepository(db)
	agingSnapRepo := postgres.NewAgingSnapshotRepository(db)

	// Interval configurable via env, default 24h.
	interval := 24 * time.Hour
	if d := os.Getenv("AGING_RECALC_INTERVAL"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil && parsed > 0 {
			interval = parsed
		}
	}

	recalc := usecase.NewAgingRecalculator(usecase.AgingRecalculatorDeps{
		CustomersRepo: agingSnapRepo,
		LiveAging:     invoiceRepo,
		Logger:        log,
	})

	agingWorker := worker.NewAgingWorker(worker.AgingWorkerDeps{
		Service:  recalc,
		Logger:   log,
		Interval: interval,
	})
	agingWorker.Start(ctx)
	log.Info("aging worker started", "interval", interval)
	return nil
}

// wireFxRateWorker starts the FxRateWorker (Sprint 26 / Fase 1D follow-up).
//
// Provider selection:
//   - If cfg.FX.ProviderURL is non-empty → HTTPProvider
//   - Otherwise → StubProvider with hardcoded USD/IDR=15800, EUR/IDR=17000, SGD/IDR=11800
//     (deterministic; useful for dev/demo without network access)
func wireFxRateWorker(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	pool, err := infra.NewPGXPool(ctx, &cfg.DB)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	currencyRepo := postgres.NewCurrencyRepository(postgres.NewDB(pool))

	var provider currency.FxRateProvider
	if cfg.FX.ProviderURL != "" {
		log.Info("fx rate worker: using HTTPProvider", "url", cfg.FX.ProviderURL)
		provider = fxprovider.NewHTTPProvider(fxprovider.HTTPProviderConfig{
			URL:     cfg.FX.ProviderURL,
			APIKey:  cfg.FX.ProviderAPIKey,
			Timeout: cfg.FX.ProviderTimeout,
		})
	} else {
		log.Info("fx rate worker: using StubProvider (no FX_PROVIDER_URL configured)")
		provider = fxprovider.NewStubProvider(map[string]decimal.Decimal{
			"USD/IDR": decimal.NewFromInt(15800),
			"EUR/IDR": decimal.NewFromInt(17000),
			"SGD/IDR": decimal.NewFromInt(11800),
		})
	}

	// Parse pairs from config
	pairs := make([]currency.CurrencyPair, 0, len(cfg.FX.Pairs))
	for _, s := range cfg.FX.Pairs {
		p, err := currency.ParsePair(s)
		if err != nil {
			log.Warn("fx rate worker: invalid pair, skipping", "pair", s, "error", err)
			continue
		}
		pairs = append(pairs, p)
	}
	if len(pairs) == 0 {
		log.Warn("fx rate worker: no valid pairs configured — worker will skip cycles")
	}

	refresher := usecase.NewFxRateRefresher(usecase.FxRateRefresherDeps{
		Provider: provider,
		Repo:     currencyRepo,
		Logger:   log,
	})

	fxWorker := worker.NewFxRateWorker(worker.FxRateWorkerDeps{
		Refresher: refresher,
		Pairs:     pairs,
		Logger:    log,
		Interval:  cfg.FX.RefreshInterval,
	})
	fxWorker.Start(ctx)
	log.Info("fx rate worker started",
		"interval", cfg.FX.RefreshInterval,
		"pairs", len(pairs),
	)
	return nil
}
