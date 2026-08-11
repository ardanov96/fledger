// Package main is the API server entrypoint for FMCG Wallet.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/runut/fmcg-wallet/internal/auth/jwt"
	"github.com/runut/fmcg-wallet/internal/auth/rbac"
	"github.com/runut/fmcg-wallet/internal/domain/invoice"
	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/handler"
	"github.com/runut/fmcg-wallet/internal/infra"
	"github.com/runut/fmcg-wallet/internal/middleware"
	"github.com/runut/fmcg-wallet/internal/platform/config"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
	"github.com/runut/fmcg-wallet/internal/platform/logger"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

var (
	version   = "dev"
	buildTime = "unknown"
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
	log.Info("starting fmcg-wallet api",
		"version", version, "build_time", buildTime,
		"env", cfg.App.Env, "go_version", runtime.Version(),
		"port", cfg.App.Port,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := infra.NewPGXPool(ctx, &cfg.DB)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	log.Info("database connected",
		"host", cfg.DB.Host, "port", cfg.DB.Port, "database", cfg.DB.Name,
		"max_conns", cfg.DB.MaxConns,
	)

	db := postgres.NewDB(pool)
	accountRepo := postgres.NewAccountRepository(db)
	transactionRepo := postgres.NewTransactionRepository(db)
	entryRepo := postgres.NewEntryRepository(db)
	invoiceRepo := postgres.NewInvoiceRepository(db)
	creditLimitRepo := postgres.NewCreditLimitRepository(db)

	txAdapter := &dbTxAdapter{db: db}
	invoiceTx := &invoiceTxAdapter{db: db}

	transferService := usecase.NewTransferService(usecase.TransferServiceDeps{
		Accounts:     accountRepo,
		Transactions: transactionRepo,
		Entries:      entryRepo,
		DB:           txAdapter,
		Logger:       log,
	})
	accountService := usecase.NewAccountService(accountRepo, entryRepo)
	invoiceService := usecase.NewInvoiceService(usecase.InvoiceServiceDeps{
		Invoices:     invoiceRepo,
		CreditLimits: creditLimitRepo,
		DB:           invoiceTx,
		Logger:       log,
	})

	verifier := jwt.NewVerifier(jwt.StaticSecret{Value: []byte(cfg.JWT.Secret)})

	modelPath, policyPath := resolveRBACPaths()
	rbacEnforcer, err := rbac.New(modelPath, policyPath)
	if err != nil {
		return fmt.Errorf("init RBAC enforcer: %w", err)
	}
	log.Info("RBAC enforcer loaded", "policy", rbacEnforcer.Source())

	h := handler.New(transferService, accountService, invoiceService)

	auditRepo := postgres.NewAuditRepository(db)
	auditHandlers := &handler.AuditHandlers{Repo: auditRepo}

	router := buildRouter(cfg, log, pool, h, auditHandlers, *verifier, rbacEnforcer)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App.Port),
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	serverErrCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	select {
	case err := <-serverErrCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdownCh:
		log.Info("shutdown signal received", "signal", sig.String())
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
		return err
	}
	log.Info("graceful shutdown complete")
	return nil
}

func resolveRBACPaths() (string, string) {
	const defaultDir = "internal/auth/rbac/policies"
	const modelFile = "rbac_model.conf"
	const policyFile = "rbac_policy.csv"

	dir := os.Getenv("RBAC_POLICY_DIR")
	if dir == "" {
		dir = defaultDir
	}
	abs, _ := filepath.Abs(dir)
	return filepath.Join(abs, modelFile), filepath.Join(abs, policyFile)
}

func buildRouter(
	cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool,
	h *handler.Handlers,
	auditHandlers *handler.AuditHandlers,
	verifier jwt.Verifier, rbacEnforcer *rbac.Enforcer,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RealIP)
	r.Use(httpx.RequestIDMiddleware)
	r.Use(chimw.RequestID)
	r.Use(structuredLogger(log))
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(corsMiddleware(cfg.Web.Origin))

	r.Get("/healthz", livenessHandler)
	r.Get("/readyz", readinessHandler(pool))
	r.Get("/version", versionHandler())

	if cfg.Telemetry.MetricsEnabled {
		r.Method(http.MethodGet, cfg.Telemetry.MetricsPath, prometheusHandler())
	}

	r.Route("/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
			httpx.JSON(w, http.StatusOK, map[string]string{"message": "pong"})
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(verifier))

			r.Post("/accounts", h.CreateAccount)
			r.Get("/accounts", h.ListAccounts)
			r.Get("/accounts/{id}", h.GetAccount)
			r.Get("/accounts/{id}/entries", h.ListAccountEntries)

			r.With(middleware.RequirePermission(verifier, rbacEnforcer,
				rbac.ActionCreate, rbac.ObjectTransfer)).Post("/transfers", h.CreateTransfer)

			r.Post("/invoices", h.CreateInvoice)
			r.Get("/invoices/{id}", h.GetInvoice)
			r.Get("/invoices", h.ListInvoices)
			r.Post("/customers/{id}/payments", h.RecordPayment)
			r.Get("/customers/{id}/aging", h.GetCustomerAging)
			r.Post("/customers/{id}/credit-limit", h.SetCreditLimit)
		})

		r.With(middleware.RequirePermission(verifier, rbacEnforcer,
			rbac.ActionRead, rbac.ObjectAuditLog)).Get("/audit", auditHandlers.ListAudit)
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, errors.New("not found"))
	})

	return r
}

func structuredLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", httpx.GetRequestID(r.Context()),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

func corsMiddleware(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Idempotency-Key, X-Tenant-ID")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "600")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func livenessHandler(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{
		"status": "alive",
		"uptime": time.Since(startedAt).String(),
	})
}

func readinessHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status := "ready"
		httpStatus := http.StatusOK
		checks := map[string]string{}

		if err := pool.Ping(ctx); err != nil {
			status = "not_ready"
			httpStatus = http.StatusServiceUnavailable
			checks["postgres"] = "DOWN: " + err.Error()
		} else {
			checks["postgres"] = "up"
		}
		checks["redis"] = "skipped (not yet wired)"
		checks["nats"] = "skipped (not yet wired)"

		httpx.JSON(w, httpStatus, map[string]any{
			"status": status,
			"checks": checks,
		})
	}
}

func versionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{
			"version":    version,
			"build_time": buildTime,
			"go_version": runtime.Version(),
		})
	}
}

var startedAt = time.Now()

type dbTxAdapter struct {
	db *postgres.DB
}

func (a *dbTxAdapter) ExecuteTx(ctx context.Context, fn func(ledger.Tx) error) error {
	return a.db.RunInTxDomain(ctx, fn)
}

type invoiceTxAdapter struct {
	db *postgres.DB
}

func (a *invoiceTxAdapter) ExecuteTx(ctx context.Context, fn func(invoice.Tx) error) error {
	return a.db.RunInTxInvoiceDomain(ctx, fn)
}
