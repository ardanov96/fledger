// Package main is the Fledger Dunning API binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/fledger/fledger-dunning/internal/auth/jwt"
	"github.com/fledger/fledger-dunning/internal/config"
	dunninghttp "github.com/fledger/fledger-dunning/internal/delivery/http"
	"github.com/fledger/fledger-dunning/internal/delivery/http/handler"
	"github.com/fledger/fledger-dunning/internal/platform/log"
	"github.com/fledger/fledger-dunning/internal/platform/whatsapp"
	"github.com/fledger/fledger-dunning/internal/repository/postgres"
	"github.com/fledger/fledger-dunning/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fledger-dunning fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := log.New(levelFor(cfg.AppEnv))
	logger.Info("starting fledger-dunning", "cfg", cfg.String())

	rootCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := postgres.NewPool(rootCtx, cfg)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()
	logger.Info("postgres pool ready")

	// Repositories.
	configs := postgres.NewConfigRepo(pool)
	sessions := postgres.NewSessionRepo(pool)
	contacts := postgres.NewStoreContactRepo(pool)
	queues := postgres.NewQueueRepo(pool)
	stats := postgres.NewStatementRepo(pool)
	logs := postgres.NewMessageLogRepo(pool)
	audits := postgres.NewAuditRepo(pool)

	// WhatsApp provider (mock by default).
	provider := whatsapp.NewMockProvider(logger)

	signer := jwt.NewSigner(jwt.StaticSecret{Value: []byte(cfg.JWTSecret)})
	verifier := jwt.NewVerifier(jwt.StaticSecret{Value: []byte(cfg.JWTSecret)})

	services := usecase.NewServices(usecase.Deps{
		Configs: configs, Sessions: sessions, Contacts: contacts, Queues: queues,
		Stats: stats, Logs: logs, Audits: audits, Provider: provider,
		JitterMinSeconds: cfg.WAJitterMinSeconds, JitterMaxSeconds: cfg.WAJitterMaxSeconds,
	})
	services.Dunning.SetLogger(logger)
	services.Statement.SetLogger(logger)

	bgCtx, bgCancel := context.WithCancel(rootCtx)
	defer bgCancel()
	go services.Dunning.DrainLoop(bgCtx, cfg.OutboxPollInterval, 16)

	v := validator.New(validator.WithRequiredStructEnabled())
	handlers := handler.NewHandlers(v, services, func() error { return pool.Ping(rootCtx) })

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           dunninghttp.NewRouter(cfg, handlers, *verifier),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("http listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// Mint a service JWT for downstream calls.
	if err := mintServiceJWT(signer, cfg); err != nil {
		logger.Warn("mint service JWT failed", "err", err.Error())
	}

	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		return err
	}

	shutdownCtx, sCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer sCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err.Error())
	}
	bgCancel()
	logger.Info("fledger-dunning stopped cleanly")
	return nil
}

func mintServiceJWT(s *jwt.Signer, cfg *config.Config) error {
	_, err := s.Sign(jwt.Claims{
		UserID: "service:fledger-dunning",
		Tenant: cfg.FledgerTenantID,
		Role:   "service",
		Scopes: []string{"dunning:write"},
	}, 1*time.Hour)
	return err
}

func levelFor(env string) string {
	if env == "development" {
		return "debug"
	}
	return "info"
}
