// Package main is the Fledger Fleet API binary. It wires the config loader,
// JWT verifier, repositories, use-case services, and HTTP routes.
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
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"

	"github.com/fledger/fledger-fleet/internal/auth/jwt"
	"github.com/fledger/fledger-fleet/internal/config"
	"github.com/fledger/fledger-fleet/internal/handler"
	"github.com/fledger/fledger-fleet/internal/integration/coreclient"
	fleetmw "github.com/fledger/fledger-fleet/internal/middleware"
	"github.com/fledger/fledger-fleet/internal/platform/httpx"
	"github.com/fledger/fledger-fleet/internal/platform/log"
	"github.com/fledger/fledger-fleet/internal/repository/postgres"
	"github.com/fledger/fledger-fleet/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fledger-fleet fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	loggerLevel := "info"
	if cfg.AppEnv == "development" {
		loggerLevel = "debug"
	}
	logger := log.New(loggerLevel)
	logger.Info("starting fledger-fleet", "cfg", cfg.String())

	rootCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := postgres.NewPool(rootCtx, cfg)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()
	logger.Info("postgres pool ready")

	vehicleRepo := postgres.NewVehicleRepo(pool)
	driverRepo := postgres.NewDriverRepo(pool)
	tripRepo := postgres.NewTripRepo(pool)
	doRepo := postgres.NewDORepo(pool)
	podRepo := postgres.NewPODRepo(pool)
	outboxRepo := postgres.NewOutboxRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)

	// JWT signer/verifier — single-secret (multi-key supported by the lib).
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
		Pool:       pool,
		Vehicles:   vehicleRepo,
		Drivers:    driverRepo,
		Trips:      tripRepo,
		DOs:        doRepo,
		PODs:       podRepo,
		Outbox:     outboxRepo,
		Audit:      auditRepo,
		CoreClient: coreHTTP,
	})
	services.Outbox.SetLogger(logger)

	// Background worker: drain the outbox to Core.
	bgCtx, bgCancel := context.WithCancel(rootCtx)
	defer bgCancel()
	go services.Outbox.DrainLoop(bgCtx, cfg.OutboxPollInterval, 16, cfg.OutboxMaxAttempts)

	// HTTP routes.
	v := validator.New(validator.WithRequiredStructEnabled())
	handlers := handler.NewHandlers(v, services)

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)
	r.Use(httpx.Logger(logger))

	// CORS middleware for Web Portal & external apps (e.g. ekspedisi-dashboard)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Tenant-ID")
			w.Header().Set("Access-Control-Expose-Headers", "Link")
			w.Header().Set("Access-Control-Max-Age", "300")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	// Web Portal static files
	webDirs := []string{
		os.Getenv("WEB_DIR"),
		"web",
		"../web",
		"../../web",
		"fledger-fleet/web",
	}
	for _, dir := range webDirs {
		if dir == "" {
			continue
		}
		if stat, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !stat.IsDir() {
			fs := http.FileServer(http.Dir(dir))
			r.Handle("/web/*", http.StripPrefix("/web", fs))
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			})
			r.Get("/style.css", func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, filepath.Join(dir, "style.css"))
			})
			r.Get("/fleet-client.js", func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, filepath.Join(dir, "fleet-client.js"))
			})
			r.Get("/app.js", func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, filepath.Join(dir, "app.js"))
			})
			logger.Info("serving web portal", "dir", dir)
			break
		}
	}

	r.Route("/v1", func(r chi.Router) {
		r.Get("/ping", handlers.Ping)
		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(fleetmw.RequireAuth(verifier))
			r.Get("/fleet/vehicles", handlers.ListVehicles)
			r.Post("/fleet/vehicles", handlers.CreateVehicle)
			r.Get("/fleet/vehicles/{id}", handlers.GetVehicle)
			r.Get("/fleet/drivers", handlers.ListDrivers)
			r.Post("/fleet/drivers", handlers.CreateDriver)
			r.Get("/fleet/drivers/{id}", handlers.GetDriver)
			r.Get("/fleet/trips", handlers.ListTrips)
			r.Get("/fleet/trips/today", handlers.ListTodayTrips)
			r.Post("/fleet/trips", handlers.CreateTrip)
			r.Get("/fleet/trips/{id}", handlers.GetTrip)
			r.Post("/fleet/trips/{id}/dispatch", handlers.DispatchTrip)
			r.Get("/fleet/delivery-orders", handlers.ListDOs)
			r.Post("/fleet/delivery-orders", handlers.CreateDO)
			r.Get("/fleet/delivery-orders/{id}", handlers.GetDO)
			r.Post("/fleet/delivery-orders/{id}/pod", handlers.SubmitPOD)
			r.Get("/fleet/outbox/counts", handlers.OutboxCounts)
		})
	})

	// Local dev login helper — mints a service token (NOT exposed in prod).
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
				Scopes: []string{"fleet:write", "fleet:read"},
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
	logger.Info("fledger-fleet stopped cleanly")
	return nil
}

// mintServiceJWT issues a short-lived JWT for Fledger Fleet → Core calls so
// the Core API treats us as an authenticated service caller.
func mintServiceJWT(s *jwt.Signer, cfg *config.Config) string {
	tok, err := s.Sign(jwt.Claims{
		UserID: "service:fledger-fleet",
		Tenant: cfg.FledgerTenantID,
		Role:   "service",
		Scopes: []string{"invoice:create"},
	}, 1*time.Hour)
	if err != nil {
		// Fallback: empty token (Core will reject, outbox keeps retrying).
		slog.Default().Warn("mint service JWT failed", "err", err.Error())
		return ""
	}
	return tok
}