// tenant_rate_limit_tier_repo.go - Postgres impl of TenantRateLimitTierRepository
// (Sprint 62).
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
)

// =============================================================================
// Repository
// =============================================================================

type TenantRateLimitTierRepository struct {
	db *DB
}

func NewTenantRateLimitTierRepository(db *DB) *TenantRateLimitTierRepository {
	return &TenantRateLimitTierRepository{db: db}
}

// GetByTenant reads one row. Returns apperrors.ErrNotFound if missing —
// caller should fall back to defaults.
//
// Sprint 62: uses RunInReadTx so RLS evaluates correctly.
func (r *TenantRateLimitTierRepository) GetByTenant(ctx context.Context, tenantID uuid.UUID) (domain.TenantRateLimitTier, error) {
	const q = `
SELECT tenant_id, tier_name,
       custom_ip_burst, custom_ip_rps,
       custom_user_burst, custom_user_rps,
       custom_tenant_burst, custom_tenant_rps,
       notes, created_at, updated_at
FROM tenant_rate_limit_tiers
WHERE tenant_id = $1
`
	var t domain.TenantRateLimitTier
	var notes *string
	err := r.db.RunInReadTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, q, tenantID).Scan(
			&t.TenantID, &t.TierName,
			&t.CustomIPBurst, &t.CustomIPRPS,
			&t.CustomUserBurst, &t.CustomUserRPS,
			&t.CustomTenantBurst, &t.CustomTenantRPS,
			&notes,
			&t.CreatedAt, &t.UpdatedAt,
		)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TenantRateLimitTier{}, apperrors.ErrNotFound
		}
		return domain.TenantRateLimitTier{}, fmt.Errorf("get tenant rate limit tier: %w", err)
	}
	t.Notes = notes
	return t, nil
}

// Upsert inserts or updates one row. Used by ops tools / API endpoints to
// adjust tier limits. Sprint 62: uses RunInTx so RLS WITH CHECK passes.
func (r *TenantRateLimitTierRepository) Upsert(ctx context.Context, t domain.TenantRateLimitTier) error {
	const q = `
INSERT INTO tenant_rate_limit_tiers (
    tenant_id, tier_name,
    custom_ip_burst, custom_ip_rps,
    custom_user_burst, custom_user_rps,
    custom_tenant_burst, custom_tenant_rps,
    notes, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, now()
)
ON CONFLICT (tenant_id) DO UPDATE SET
    tier_name = EXCLUDED.tier_name,
    custom_ip_burst = EXCLUDED.custom_ip_burst,
    custom_ip_rps = EXCLUDED.custom_ip_rps,
    custom_user_burst = EXCLUDED.custom_user_burst,
    custom_user_rps = EXCLUDED.custom_user_rps,
    custom_tenant_burst = EXCLUDED.custom_tenant_burst,
    custom_tenant_rps = EXCLUDED.custom_tenant_rps,
    notes = EXCLUDED.notes,
    updated_at = now()
`
	var notes any
	if t.Notes != nil {
		notes = *t.Notes
	} else {
		notes = nil
	}
	return r.db.RunInTx(ctx, func(pgxTx pgx.Tx) error {
		_, err := pgxTx.Exec(ctx, q,
			t.TenantID, string(t.TierName),
			t.CustomIPBurst, t.CustomIPRPS,
			t.CustomUserBurst, t.CustomUserRPS,
			t.CustomTenantBurst, t.CustomTenantRPS,
			notes,
		)
		if err != nil {
			return fmt.Errorf("upsert tenant rate limit tier: %w", err)
		}
		return nil
	})
}

// Delete removes one row. Returns ErrNotFound if no row existed.
func (r *TenantRateLimitTierRepository) Delete(ctx context.Context, tenantID uuid.UUID) error {
	const q = `DELETE FROM tenant_rate_limit_tiers WHERE tenant_id = $1`
	return r.db.RunInTx(ctx, func(pgxTx pgx.Tx) error {
		tag, err := pgxTx.Exec(ctx, q, tenantID)
		if err != nil {
			return fmt.Errorf("delete tenant rate limit tier: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.ErrNotFound
		}
		return nil
	})
}