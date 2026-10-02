-- =============================================================================
-- Migration: 000026_user_credentials_rls
-- Description: Enable RLS on user_credentials (Sprint 35 / Fase 5A)
-- Author: FMCG Wallet
-- =============================================================================
-- Background (Sprint 15 / ADR-0006):
--   Migration 000014 enabled RLS on 11 tenant-scoped tables but EXPLICITLY
--   excluded user_credentials:
--
--     > user_credentials (has tenant_id; but RLS would block admin tools —
--     >     for Sprint 15 we exclude this table from RLS and rely on app-layer
--     >     checks. Future Sprint: add admin RLS bypass via dedicated role.)
--
-- The "dedicated role" arrived in Sprint 14 (migration 000015):
--   `app_admin` with `USING (true) WITH CHECK (true)` policies on every
--   RLS-enabled table. This Sprint 35 closes the loop.
--
-- Strategy:
--   1. Enable RLS + FORCE on user_credentials
--   2. Add tenant_isolation_select policy (require tenant match)
--   3. Add tenant_isolation_modify policy (require tenant match)
--   4. Add admin_bypass policy for app_admin (USING (true))
--   5. Grant table-level access to fmcg + app_admin (auto via migration 000023
--      ALTER DEFAULT PRIVILEGES for tables created AFTER 000023, but this
--      table was created in 000013 — explicit GRANT needed)
--   6. Idempotent: safe to re-run (DROP POLICY IF EXISTS before CREATE)
--
-- App-layer checks must still verify tenant before operations, but RLS now
-- provides defense-in-depth: even if app-layer filter is forgotten, RLS
-- will reject cross-tenant reads/writes for the fmcg role.
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Grant table-level access (table was created in migration 000013,
--    BEFORE 000023's ALTER DEFAULT PRIVILEGES, so explicit GRANTs needed).
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fmcg') THEN
        -- fmcg needs SELECT/INSERT/UPDATE/DELETE for the auth flow
        GRANT SELECT, INSERT, UPDATE, DELETE ON user_credentials TO fmcg;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_admin') THEN
        -- app_admin gets the same so it can act during RLS bypass
        GRANT SELECT, INSERT, UPDATE, DELETE ON user_credentials TO app_admin;
    END IF;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

-- ---------------------------------------------------------------------------
-- 2. Enable RLS
-- ---------------------------------------------------------------------------
ALTER TABLE user_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_credentials FORCE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 3. Tenant isolation policies
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS user_credentials_tenant_isolation_select ON user_credentials;
CREATE POLICY user_credentials_tenant_isolation_select ON user_credentials
    FOR SELECT
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

DROP POLICY IF EXISTS user_credentials_tenant_isolation_modify ON user_credentials;
CREATE POLICY user_credentials_tenant_isolation_modify ON user_credentials
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

-- ---------------------------------------------------------------------------
-- 4. Admin bypass (USING (true)) for app_admin — runs ops/maintenance tasks.
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS user_credentials_admin_bypass ON user_credentials;
CREATE POLICY user_credentials_admin_bypass ON user_credentials
    TO app_admin
    USING (true) WITH CHECK (true);

COMMENT ON TABLE user_credentials IS
    'Sprint 35: tenant-isolated credentials table with admin_bypass policy for app_admin role. '
    'App-layer checks still required; RLS provides defense-in-depth.';

COMMIT;

-- =============================================================================
-- DOWN (rollback)
-- =============================================================================
-- BEGIN;
-- DROP POLICY IF EXISTS user_credentials_admin_bypass ON user_credentials;
-- DROP POLICY IF EXISTS user_credentials_tenant_isolation_modify ON user_credentials;
-- DROP POLICY IF EXISTS user_credentials_tenant_isolation_select ON user_credentials;
-- ALTER TABLE user_credentials DISABLE ROW LEVEL SECURITY;
-- COMMIT;