-- =============================================================================
-- Migration: 000028_tenant_rate_limit_tiers
-- Description: Per-tenant rate-limit tier overrides (Sprint 62)
-- Author: FMCG Wallet
-- =============================================================================
-- Sprint 62 follow-up to Sprint 61. Until this sprint, all tenants shared
-- the same rate-limit configuration (defined via env vars at process
-- startup). For production, each tenant may want different SLAs:
--   - Free tier:    100 req/s per user, 500 req/s per tenant
--   - Standard tier: 200 req/s per user, 2000 req/s per tenant
--   - Premium tier:  1000 req/s per user, 10000 req/s per tenant
--
-- Strategy:
--   1. New table tenant_rate_limit_tiers (one row per tenant).
--      NULL columns fall back to deployment-wide env defaults.
--   2. Same RLS pattern as other tenant tables (FORCE + admin_bypass
--      via migration 000023 default privileges + explicit GRANTs since
--      this table was created BEFORE 000023 follow-up? Actually 000023
--      came after 000013 which made user_credentials etc. 000028 is
--      after 000023, so default privileges auto-apply).
--   3. TenantRateLimitTierService caches for 60s; same pattern as
--      Sprint 38 tenant_fraud_settings.
--   4. MultiTierLimiter enhanced to look up per-tenant bucket on
--      each request; cache miss → DB → reuse.
--
-- Migration plan:
--   1. Create table
--   2. Idempotent: safe to re-run (PRIMARY KEY is tenant_id)
-- =============================================================================

BEGIN;

CREATE TABLE IF NOT EXISTS tenant_rate_limit_tiers (
    tenant_id        UUID         PRIMARY KEY,
    tier_name        TEXT         NOT NULL DEFAULT 'standard',
    -- NULL = use env default (RATE_LIMIT_GLOBAL_BURST/RPS).
    -- Non-NULL = override per tenant.
    custom_ip_burst       INT     NULL,
    custom_ip_rps         INT     NULL,
    custom_user_burst     INT     NULL,
    custom_user_rps       INT     NULL,
    custom_tenant_burst   INT     NULL,
    custom_tenant_rps     INT     NULL,
    notes           TEXT         NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT tenant_rate_limit_tiers_tier_name_valid CHECK (
        tier_name IN ('free', 'standard', 'premium', 'enterprise')
    ),
    CONSTRAINT tenant_rate_limit_tiers_positive_burst CHECK (
        custom_ip_burst IS NULL OR custom_ip_burst > 0
    ),
    CONSTRAINT tenant_rate_limit_tiers_positive_rps CHECK (
        custom_ip_rps IS NULL OR custom_ip_rps > 0
    )
);

-- Grant table for RLS — app_admin needs admin_bypass (via migration 000023
-- default privileges which auto-applies since 000028 is after 000023).
-- fmcg gets SELECT/INSERT/UPDATE/DELETE for normal CRUD.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fmcg') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON tenant_rate_limit_tiers TO fmcg;
    END IF;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

-- RLS + admin bypass.
ALTER TABLE tenant_rate_limit_tiers ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_rate_limit_tiers FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_rate_limit_tiers_isolation_select ON tenant_rate_limit_tiers;
CREATE POLICY tenant_rate_limit_tiers_isolation_select ON tenant_rate_limit_tiers
    FOR SELECT USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    );

DROP POLICY IF EXISTS tenant_rate_limit_tiers_isolation_modify ON tenant_rate_limit_tiers;
CREATE POLICY tenant_rate_limit_tiers_isolation_modify ON tenant_rate_limit_tiers
    FOR ALL USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    )
    WITH CHECK (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    );

DROP POLICY IF EXISTS tenant_rate_limit_tiers_admin_bypass ON tenant_rate_limit_tiers;
CREATE POLICY tenant_rate_limit_tiers_admin_bypass ON tenant_rate_limit_tiers
    TO app_admin
    USING (true) WITH CHECK (true);

COMMENT ON TABLE tenant_rate_limit_tiers IS
    'Sprint 62: per-tenant rate-limit overrides. NULL columns fall back to '
    'deployment-wide env defaults (RATE_LIMIT_GLOBAL_BURST/RPS etc). '
    'TenantRateLimitTierService caches for 60s.';

COMMIT;

-- =============================================================================
-- DOWN (rollback)
-- =============================================================================
-- BEGIN;
-- DROP POLICY IF EXISTS tenant_rate_limit_tiers_admin_bypass ON tenant_rate_limit_tiers;
-- DROP POLICY IF EXISTS tenant_rate_limit_tiers_isolation_modify ON tenant_rate_limit_tiers;
-- DROP POLICY IF EXISTS tenant_rate_limit_tiers_isolation_select ON tenant_rate_limit_tiers;
-- DROP TABLE IF EXISTS tenant_rate_limit_tiers;
-- COMMIT;