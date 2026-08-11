// Package main is the background worker entrypoint for FMCG Wallet.
//
// The worker hosts long-running jobs:
//   - asynq scheduled tasks (aging recalculation, notifications, fraud scan)
//   - NATS JetStream consumers (write-side fanout, projections)
//   - Outbox publisher (transactional outbox pattern)
//
// In Fase 0, this is a placeholder that just logs and waits for shutdown.
// Real workers are added starting Fase 1 (trial balance reconciler) onward.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/runut/fmcg-wallet/internal/platform/config"
	"github.com/runut/fmcg-wallet/internal/platform/logger"
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
	// Placeholder workers (added in later phases)
	// -------------------------------------------------------------------------
	// TODO Fase 1: trial_balance_reconciler (scheduled hourly)
	// TODO Fase 1: hash_chain_verifier (scheduled daily)
	// TODO Fase 2: aging_recalculator (scheduled nightly)
	// TODO Fase 4: outbox_publisher (continuous)
	// TODO Fase 8: notification_dispatcher
	// TODO Fase 8: fraud_flag_scanner

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Heartbeat so the process doesn't exit
	go heartbeat(ctx, log)

	// Wait for shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-shutdownCh
	log.Info("worker shutdown signal received", "signal", sig.String())
	cancel()

	// Give in-flight work a moment to drain
	time.Sleep(2 * time.Second)
	log.Info("worker stopped")
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

var startedAt = time.Now()
