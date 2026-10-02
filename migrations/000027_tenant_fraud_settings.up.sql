-- =============================================================================
-- Migration: 000027_tenant_fraud_settings
-- Description: Per-tenant configurable fraud detection thresholds (Sprint 38)
-- Author: FMCG Wallet
-- =============================================================================
-- Sprint 38 follow-up to Sprint 31 (Fraud Flag Scanner). Current thresholds
-- are env-only (FRAUD_LARGE_AMOUNT_THRESHOLD_MINOR, FRAUD_VELOCITY_*,
-- FRAUD_OFF_HOURS_*) — one set per deployment, no per-tenant customization.
--
-- In production, each tenant may want different risk appetite:
--   - High-volume enterprise customer wants a higher large_amount
--     threshold (say 500M IDR vs default 50M) to reduce noise.
--   - New/small customer wants tighter velocity limits (5 transfers in
--     10min vs default 10 in 5min).
--
-- Strategy:
--   1. New table `tenant_fraud_settings` — one row per tenant. Defaults
--   2. Same RLS pattern as other tenant tables (FORCE RLS + admin_bypass)
--   3. Index on tenant_id (PK) for fast lookup
--   4. Optional `disabled_rules JSONB` — operators can disable specific
--   rules per tenant (e.g., turn off OffHoursRule for a 24/7 business)
--   5. created_at + updated_at for audit
--   6. updated_by for change tracking
--
-- Read path:
--   - TenantFraudSettingsService.Get(tenantID) → cached or DB lookup
--   - Falls back to env defaults (FraudConfig) if no row
--   - In-memory cache with TTL (60s) so we don't hit DB on every flag
--
-- Default fallback behavior:
--   - Missing row → use env FraudConfig values
--   - NULL column in row → use that column's env default
--   - Allows partial per-tenant overrides
-- =============================================================================

BEGIN;

CREATE TABLE IF NOT EXISTS tenant_fraud_settings (
    tenant_id         UUID         PRIMARY KEY,

    -- Per-rule thresholds (NULL = fall back to env FraudConfig)
    large_amount_threshold_minor BIGINT  NULL,
    velocity_max_count           INT     NULL,
    velocity_window_seconds       INT     NULL,  -- stored as seconds for portability
    off_hours_start_hour          INT     NULL,
    off_hours_end_hour            INT     NULL,

    -- Optional: rules to disable per-tenant (array of rule names)
    disabled_rules               JSONB   NULL DEFAULT '[]'::jsonb,

    -- Audit
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by         UUID         NULL,

    -- Sanity: thresholds must be positive when set
    CONSTRAINT tenant_fraud_settings_large_amount_positive
        CHECK (large_amount_threshold_minor IS NULL OR large_amount_threshold_minor > 0),
    CONSTRAINT tenant_fraud_settings_velocity_max_positive
        CHECK (velocity_max_count IS NULL OR velocity_max_count > 0),
    CONSTRAINT tenant_fraud_settings_velocity_window_positive
        CHECK (velocity_window_seconds IS NULL OR velocity_window_seconds > 0),
    CONSTRAINT tenant_fraud_settings_off_hours_start_valid
        CHECK (off_hours_start_hour IS NULL OR (off_hours_start_hour >= 0 AND off_hours_start_hour <= 23)),
    CONSTRAINT tenant_fraud_settings_off_hours_end_valid
        CHECK (off_hours_end_hour IS NULL OR (off_hours_end_hour >= 0 AND off_hours_end_hour <= 23))
);

-- Grant + RLS (table created after 000023, so ALTER DEFAULT PRIVILEGES
-- auto-applies for app_admin. fmcg gets explicit GRANT).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fmcg') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON tenant_fraud_settings TO fmcg;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO fmcg;
    END IF;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

-- RLS + admin bypass
ALTER TABLE tenant_fraud_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_fraud_settings FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_fraud_settings_isolation_select ON tenant_fraud_settings;
CREATE POLICY tenant_fraud_settings_isolation_select ON tenant_fraud_settings
    FOR SELECT USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    );

DROP POLICY IF EXISTS tenant_fraud_settings_isolation_modify ON tenant_fraud_settings;
CREATE POLICY tenant_fraud_settings_isolation_modify ON tenant_fraud_settings
    FOR ALL USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    )
    WITH CHECK (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    );

DROP POLICY IF EXISTS tenant_fraud_settings_admin_bypass ON tenant_fraud_settings;
CREATE POLICY tenant_fraud_settings_admin_bypass ON tenant_fraud_settings
    TO app_admin
    USING (true) WITH CHECK (true);

COMMENT ON TABLE tenant_fraud_settings IS
    'Sprint 38: per-tenant overrides for fraud detection thresholds. '
    'NULL columns fall back to the deployment-wide FraudConfig defaults. '
    'Used by TenantFraudSettingsService (with 60s in-memory cache) to resolve '
    'rule thresholds per-transfer before evaluation.';

COMMIT;

-- =============================================================================
-- DOWN (rollback)
-- =============================================================================
-- BEGIN;
-- DROP POLICY IF EXISTS tenant_fraud_settings_admin_bypass ON tenant_fraud_settings;
-- DROP POLICY IF EXISTS tenant_fraud_settings_isolation_modify ON tenant_fraud_settings;
-- DROP POLICY IF EXISTS tenant_fraud_settings_isolation_select ON tenant_fraud_settings;
-- DROP TABLE IF EXISTS tenant_fraud_settings;
-- COMMIT;