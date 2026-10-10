// Package http ??? router setup for Fledger Dunning.
package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/fledger/fledger-dunning/internal/auth/jwt"
	"github.com/fledger/fledger-dunning/internal/config"
	"github.com/fledger/fledger-dunning/internal/delivery/http/handler"
	"github.com/fledger/fledger-dunning/internal/middleware"
	"github.com/fledger/fledger-dunning/internal/platform/httpx"
	"github.com/fledger/fledger-dunning/internal/webui"
)

// NewRouter assembles the Chi router for port :8086.
func NewRouter(cfg *config.Config, h *handler.Handlers, verifier jwt.Verifier) http.Handler {
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
	r.Use(httpx.Logger(nil))

	r.Get("/healthz", h.Healthz)
	r.Get("/readyz", h.Readyz)

	// Web portal (Sprint 5) served from embedded bundle.
	wu := webui.Handler()
	r.Mount("/web", http.StripPrefix("/web", wu))
	r.Get("/", wu.ServeHTTP)

	if cfg.AppEnv == "development" {
		r.Post("/v1/dev/login", devLogin(cfg))
	}

	r.Route("/v1", func(r chi.Router) {
		r.Route("/dunning", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAuth(verifier))
				// WhatsApp
				r.Get("/whatsapp/status", h.GetWhatsAppStatus)
				r.Post("/whatsapp/qr/generate", h.GenerateQR)
				r.Post("/whatsapp/disconnect", h.DisconnectWA)
				r.Post("/whatsapp/test-send", h.TestSend)
				// Queues
				r.Get("/queues", h.ListQueues)
				r.Post("/queues/ingest-invoice", h.IngestInvoice)
				r.Post("/queues/cancel-invoice", h.CancelInvoice)
				r.Post("/queues/{id}/dispatch", h.DispatchOne)
				r.Post("/queues/cron-run", h.CronRun)
				r.Get("/queues/{id}", h.GetQueueByID)
				// Contacts
				r.Get("/contacts", h.ListContacts)
				r.Post("/contacts", h.UpsertContact)
				// Statements
				r.Get("/statements", h.ListStatements)
				r.Post("/statements/generate", h.GenerateStatement)
				r.Post("/statements/{id}/send", h.SendStatement)
				// Outbox (queue counts)
				r.Get("/outbox/counts", h.QueueCounts)
			})
			// Public webhook (signed via X-Webhook-Secret).
			r.Post("/webhooks/pay", h.PayWebhook(&handler.Config{PayWebhookSecret: cfg.PayWebhookSecret}))
		})
	})

	return r
}

// devLogin mints a dev JWT for the embedded PWA.
func devLogin(cfg *config.Config) http.HandlerFunc {
	signer := jwt.NewSigner(jwt.StaticSecret{Value: []byte(cfg.JWTSecret)})
	tenantFallback := cfg.FledgerTenantID
	if tenantFallback == "" {
		tenantFallback = middleware.DefaultTenantID
	}
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant_id")
		if tenantID == "" {
			tenantID = tenantFallback
		}
		tok, err := signer.Sign(jwt.Claims{
			UserID: "dev-user",
			Tenant: tenantID,
			Role:   "admin",
			Scopes: []string{"dunning:write", "dunning:read"},
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
	}
}
