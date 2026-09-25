// Package usecase — FxRateRefresher (Sprint 26 / Fase 1D follow-up).
//
// Fetches FX rates from a configured provider and inserts new rows into the
// fx_rates table with source='api'. Designed to be invoked periodically by
// FxRateWorker.
//
// Algorithm (one refresh cycle):
//  1. For each configured (base, target) pair:
//     a. Call provider.FetchRate(ctx, base, target).
//     b. If provider returns ErrFxProviderUnavailable → log + skip (transient).
//        Don't mark the run as failed; we'll retry next tick.
//     c. If provider returns ErrFxProviderRateMissing → log + skip (pair not supported).
//     d. Otherwise → build FxRate entity with source='api' + effective_at=now,
//        expires_at=now+24h (default), CreatedBy=system-uuid.
//     e. Insert into fx_rates table (via CurrencyRepository.CreateFxRate in a tx).
//  2. Return summary: pairs_attempted, pairs_succeeded, pairs_skipped, error.
//
// Future: publish `fx_rate.refreshed` event via outbox for downstream
// consumers (notification, dashboard cache invalidation). Skipped for MVP.
package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
)

// CurrencyRepo abstracts the currency repository operations needed by the
// refresher. Defined here to keep the use case decoupled from the postgres
// package and easy to mock in tests.
type CurrencyRepo interface {
	ListTenants(ctx context.Context) ([]uuid.UUID, error)

	// CreateFxRateNoTx inserts without a tx (uses the pool directly).
	// Sprint 26 refresh is fire-and-forget — no business tx wraps the
	// refresh cycle, so each insert is its own atomic write.
	CreateFxRateNoTx(ctx context.Context, r currency.FxRate) error
}

// FxRateRefresher fetches FX rates from the provider and stores them.
type FxRateRefresher struct {
	provider currency.FxRateProvider
	repo     CurrencyRepo
	log      *slog.Logger
	now      func() time.Time
	ttl      time.Duration // rate validity window — defaults to 24h

	systemUserID uuid.UUID // CreatedBy for refresh-sourced rates
}

// FxRateRefresherDeps bundles dependencies.
type FxRateRefresherDeps struct {
	Provider     currency.FxRateProvider
	Repo         CurrencyRepo
	Logger       *slog.Logger
	NowFunc      func() time.Time      // optional; defaults to time.Now UTC
	ValidityTTL  time.Duration         // optional; default 24h
	SystemUserID uuid.UUID             // optional; auto-generated if zero
}

// NewFxRateRefresher constructs an FxRateRefresher.
func NewFxRateRefresher(deps FxRateRefresherDeps) *FxRateRefresher {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	now := deps.NowFunc
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	ttl := deps.ValidityTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	sysUser := deps.SystemUserID
	if sysUser == uuid.Nil {
		sysUser = uuid.New()
	}
	return &FxRateRefresher{
		provider:     deps.Provider,
		repo:         deps.Repo,
		log:          log,
		now:          now,
		ttl:          ttl,
		systemUserID: sysUser,
	}
}

// FxRefreshResult summarizes one refresh cycle.
type FxRefreshResult struct {
	TenantsProcessed int
	PairsAttempted   int
	PairsSucceeded   int
	PairsSkipped     int
	DurationMs       int64
}

// RefreshOnce refreshes the configured pairs for all tenants.
//
// tenantIDs: list of tenant IDs to refresh for. If empty, the refresher
//           fetches the list from the repo via ListTenants. Passing an
//           explicit list is useful for tests.
func (r *FxRateRefresher) RefreshOnce(ctx context.Context, tenantIDs []uuid.UUID, pairs []currency.CurrencyPair) (FxRefreshResult, error) {
	start := r.now()

	if len(tenantIDs) == 0 {
		var err error
		tenantIDs, err = r.repo.ListTenants(ctx)
		if err != nil {
			return FxRefreshResult{}, fmt.Errorf("list tenants: %w", err)
		}
	}

	res := FxRefreshResult{TenantsProcessed: len(tenantIDs)}

	for _, pair := range pairs {
		res.PairsAttempted++

		rate, err := r.provider.FetchRate(ctx, pair.Base, pair.Target)
		if err != nil {
			// Transient (unavailable) → log + skip. Permanent (missing) → log + skip.
			// Either way we don't fail the whole run; we just don't update this pair.
			r.log.Warn("fx refresher: provider fetch failed",
				"pair", pair.Base+"/"+pair.Target,
				"error", err,
			)
			res.PairsSkipped++
			continue
		}

		// Insert one row per tenant.
		effectiveAt := r.now()
		expiresAt := effectiveAt.Add(r.ttl)
		for _, tid := range tenantIDs {
			fx := currency.FxRate{
				ID:           uuid.New(),
				TenantID:     tid,
				FromCurrency: pair.Base,
				ToCurrency:   pair.Target,
				Rate:         rate,
				EffectiveAt:  effectiveAt,
				ExpiresAt:    expiresAt,
				Source:       currency.FxRateSourceAPI,
				CreatedBy:    r.systemUserID,
				CreatedAt:    effectiveAt,
			}
			if err := r.repo.CreateFxRateNoTx(ctx, fx); err != nil {
				r.log.Warn("fx refresher: create failed",
					"tenant_id", tid,
					"pair", pair.Base+"/"+pair.Target,
					"error", err,
				)
				continue
			}
			res.PairsSucceeded++
		}
	}

	res.DurationMs = time.Since(start).Milliseconds()
	r.log.Info("fx refresher complete",
		"tenants", res.TenantsProcessed,
		"pairs_attempted", res.PairsAttempted,
		"pairs_succeeded", res.PairsSucceeded,
		"pairs_skipped", res.PairsSkipped,
		"duration_ms", res.DurationMs,
	)
	return res, nil
}

// helper kept to silence unused imports when refactoring
var _ = decimal.NewFromInt
