// tenant_fraud_settings_repo.go - Postgres impl of TenantFraudSettingsRepository (Sprint 38).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/domain"
)

// =============================================================================
// Repository
// =============================================================================

type TenantFraudSettingsRepository struct {
	db *DB
}

func NewTenantFraudSettingsRepository(db *DB) *TenantFraudSettingsRepository {
	return &TenantFraudSettingsRepository{db: db}
}

// GetByTenant returns the settings for one tenant, or domain.ErrNotFound
// if no row exists (caller then falls back to env defaults).
//
// Sprint 38 — uses RunInReadTx so RLS evaluates correctly.
func (r *TenantFraudSettingsRepository) GetByTenant(ctx context.Context, tenantID uuid.UUID) (domain.TenantFraudSettings, error) {
	const q = `
SELECT tenant_id, large_amount_threshold_minor, velocity_max_count,
       velocity_window_seconds, off_hours_start_hour, off_hours_end_hour,
       disabled_rules, created_at, updated_at, updated_by
FROM tenant_fraud_settings
WHERE tenant_id = $1
`
	var s domain.TenantFraudSettings
	var disabledJSON []byte
	var updatedBy *uuid.UUID
	err := r.db.RunInReadTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, q, tenantID).Scan(
			&s.TenantID,
			&s.LargeAmountThresholdMinor,
			&s.VelocityMaxCount,
			&s.VelocityWindowSeconds,
			&s.OffHoursStartHour,
			&s.OffHoursEndHour,
			&disabledJSON,
			&s.CreatedAt,
			&s.UpdatedAt,
			&updatedBy,
		)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Treat as "not configured" — caller uses defaults.
			return domain.TenantFraudSettings{}, apperrors.ErrNotFound
		}
		return domain.TenantFraudSettings{}, fmt.Errorf("get tenant fraud settings: %w", err)
	}
	if len(disabledJSON) > 0 {
		_ = json.Unmarshal(disabledJSON, &s.DisabledRules)
	}
	s.UpdatedBy = updatedBy
	return s, nil
}

// Upsert inserts or updates settings for one tenant. Sprint 38 — uses
// RunInTx so RLS WITH CHECK passes.
func (r *TenantFraudSettingsRepository) Upsert(ctx context.Context, s domain.TenantFraudSettings) error {
	disabledJSON, err := json.Marshal(s.DisabledRules)
	if err != nil {
		return fmt.Errorf("marshal disabled rules: %w", err)
	}
	const q = `
INSERT INTO tenant_fraud_settings (
    tenant_id, large_amount_threshold_minor, velocity_max_count,
    velocity_window_seconds, off_hours_start_hour, off_hours_end_hour,
    disabled_rules, updated_at, updated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, now(), $8
)
ON CONFLICT (tenant_id) DO UPDATE SET
    large_amount_threshold_minor = EXCLUDED.large_amount_threshold_minor,
    velocity_max_count            = EXCLUDED.velocity_max_count,
    velocity_window_seconds        = EXCLUDED.velocity_window_seconds,
    off_hours_start_hour           = EXCLUDED.off_hours_start_hour,
    off_hours_end_hour             = EXCLUDED.off_hours_end_hour,
    disabled_rules                 = EXCLUDED.disabled_rules,
    updated_at                     = now(),
    updated_by                     = EXCLUDED.updated_by
`
	return r.db.RunInTx(ctx, func(pgxTx pgx.Tx) error {
		_, err := pgxTx.Exec(ctx, q,
			s.TenantID,
			s.LargeAmountThresholdMinor,
			s.VelocityMaxCount,
			s.VelocityWindowSeconds,
			s.OffHoursStartHour,
			s.OffHoursEndHour,
			disabledJSON,
			s.UpdatedBy,
		)
		if err != nil {
			return fmt.Errorf("upsert tenant fraud settings: %w", err)
		}
		return nil
	})
}

// Delete removes settings for one tenant. Sprint 38 — uses RunInTx.
func (r *TenantFraudSettingsRepository) Delete(ctx context.Context, tenantID uuid.UUID) error {
	const q = `DELETE FROM tenant_fraud_settings WHERE tenant_id = $1`
	return r.db.RunInTx(ctx, func(pgxTx pgx.Tx) error {
		tag, err := pgxTx.Exec(ctx, q, tenantID)
		if err != nil {
			return fmt.Errorf("delete tenant fraud settings: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.ErrNotFound
		}
		return nil
	})
}