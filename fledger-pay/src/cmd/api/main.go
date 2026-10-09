// Package main is the Fledger Pay API binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-playground/validator/v10"

	"github.com/fledger/fledger-pay/internal/auth/jwt"
	"github.com/fledger/fledger-pay/internal/config"
	"github.com/fledger/fledger-pay/internal/handler"
	"github.com/fledger/fledger-pay/internal/integration/coreclient"
	"github.com/fledger/fledger-pay/internal/middleware"
	"github.com/fledger/fledger-pay/internal/platform/httpx"
	"github.com/fledger/fledger-pay/internal/platform/log"
	"github.com/fledger/fledger-pay/internal/repository/postgres"
	"github.com/fledger/fledger-pay/internal/usecase"
	"github.com/fledger/fledger-pay/internal/webui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fledger-pay fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := log.New(levelFor(cfg.AppEnv))
	logger.Info("starting fledger-pay", "cfg", cfg.String())

	rootCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := postgres.NewPool(rootCtx, cfg)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()
	logger.Info("postgres pool ready")

	paymentRepo := postgres.NewPaymentRepo(pool)
	txRepo := postgres.NewTransactionRepo(pool)
	outboxRepo := postgres.NewOutboxRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)

	signer := jwt.NewSigner(jwt.StaticSecret{Value: []byte(cfg.JWTSecret)})
	verifier := jwt.NewVerifier(jwt.StaticSecret{Value: []byte(cfg.JWTSecret)})

	coreHTTP := coreclient.NewClient(coreclient.Config{
		BaseURL:  cfg.FledgerCoreURL,
		TenantID: cfg.FledgerTenantID,
		JWT:      mintServiceJWT(signer, cfg),
		APIKey:   cfg.FledgerCoreAPIKey,
		Timeout:  cfg.OutboxRequestTimeout,
		Log:      logger,
	})

	services := usecase.NewServices(usecase.Deps{
		Pool:         pool,
		Payments:     paymentRepo,
		Transactions: txRepo,
		Outbox:       outboxRepo,
		Audit:        auditRepo,
		Core:         coreHTTP,
	})
	services.Settlement.SetLogger(logger)

	bgCtx, bgCancel := context.WithCancel(rootCtx)
	defer bgCancel()
	go services.Settlement.DrainLoop(bgCtx, cfg.OutboxPollInterval, 16, cfg.OutboxMaxAttempts)

	v := validator.New(validator.WithRequiredStructEnabled())
	handlers := handler.NewHandlers(v, services, cfg.WebhookSecret, pool.Ping)

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Tenant-ID", "X-Actor-Id", "X-Forwarded-For"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(httpx.Logger(logger))

	// Health probes
	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz)

	// Static web simulator (Sprint 5) — served from the embedded bundle.
	// Both "/" and "/web/*" route to the same handler.
	webuiHandler := webui.Handler()
	r.Mount("/web", http.StripPrefix("/web", webuiHandler))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/index.html"
		webuiHandler.ServeHTTP(w, r2)
	})

	// Dev login (APP_ENV=development)
	if cfg.AppEnv == "development" {
		r.Post("/v1/dev/login", func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.URL.Query().Get("tenant_id")
			if tenantID == "" {
				tenantID = cfg.FledgerTenantID
			}
			tok, err := signer.Sign(jwt.Claims{
				UserID: "dev-user",
				Tenant: tenantID,
				Role:   "admin",
				Scopes: []string{"pay:write", "pay:read"},
			}, cfg.TokenTTL)
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			httpx.JSON(w, http.StatusOK, map[string]any{
				"access_token": tok,
				"expires_at":   time.Now().Add(cfg.TokenTTL).UTC(),
				"tenant_id":    tenantID,
			})
		})
	}

	// Authenticated routes
	r.Route("/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(verifier))
			r.Post("/pay/requests", handlers.CreatePayment)
			r.Get("/pay/requests", handlers.ListPayments)
			r.Get("/pay/requests/{id}", handlers.GetPayment)
			r.Post("/pay/requests/{id}/cancel", handlers.CancelPayment)
			r.Get("/pay/outbox/counts", handlers.OutboxCounts)
		})
		// Webhook + simulator are public (signature-protected).
		r.Post("/pay/webhooks/{gateway}", handlers.WebhookHandler)
		r.Post("/pay/simulator/settle", handlers.SimulatorSettle)
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("http listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

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
	logger.Info("fledger-pay stopped cleanly")
	return nil
}

func mintServiceJWT(s *jwt.Signer, cfg *config.Config) string {
	tok, err := s.Sign(jwt.Claims{
		UserID: "service:fledger-pay",
		Tenant: cfg.FledgerTenantID,
		Role:   "service",
		Scopes: []string{"transfer:create", "invoice:pay"},
	}, 1*time.Hour)
	if err != nil {
		slog.Default().Warn("mint service JWT failed", "err", err.Error())
		return ""
	}
	return tok
}

func levelFor(env string) string {
	if env == "development" {
		return "debug"
	}
	return "info"
}