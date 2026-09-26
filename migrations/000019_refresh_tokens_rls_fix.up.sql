-- =============================================================================
-- Migration: 000019_refresh_tokens_rls_fix
-- Description: Allow refresh_tokens INSERT/UPDATE during login flow (Sprint 27)
-- Author: FMCG Wallet
-- =============================================================================
-- Background (Sprint 15/22B.4):
--   Migration 000014 enabled RLS on refresh_tokens with a strict tenant-isolation
--   policy (USING tenant_id = current_setting('app.current_tenant_id', true)::uuid).
--
-- Problem (Sprint 23/26 follow-up):
--   The login POST /v1/auth/login endpoint is mounted OUTSIDE the tenant
--   middleware (which sets app.current_tenant_id GUC). So when the AuthService
--   inserts a refresh_token during login, the GUC is NULL, the policy returns
--   NULL, and the INSERT is rejected with 'new row violates row-level security
--   policy'. This blocked all logins in production until devs manually disabled
--   RLS on refresh_tokens (workaround documented in commit b5430ca).
--
-- Fix:
--   Replace the strict policy with a more permissive one that:
--     1. Allows INSERT/UPDATE/DELETE when GUC is NULL (login flow)
--     2. Requires tenant match when GUC is set (normal authenticated flow)
--     3. Always allows app_admin (RLS bypass role)
--
-- SELECT policy unchanged - reads still require tenant context (which login
-- does NOT need, so SELECTs during login don't happen).
--
-- Idempotent: DROP POLICY IF EXISTS before CREATE.
-- =============================================================================

BEGIN;

-- Drop existing strict policies (idempotent)
DROP POLICY IF EXISTS tenant_isolation_select ON refresh_tokens;
DROP POLICY IF EXISTS tenant_isolation_modify ON refresh_tokens;
DROP POLICY IF EXISTS refresh_tokens_insert_anon ON refresh_tokens;
DROP POLICY IF EXISTS refresh_tokens_select_auth ON refresh_tokens;

-- SELECT policy: require tenant context (login doesn't SELECT refresh_tokens,
-- only INSERTs them, so this is fine).
CREATE POLICY refresh_tokens_select_auth ON refresh_tokens
    FOR SELECT USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    );

-- INSERT/UPDATE/DELETE policy: allow login flow (GUC NULL) + tenant match + admin.
-- This is the key fix - the OR current_setting(...) IS NULL branch allows
-- INSERTs during the login POST before the user has been authenticated.
CREATE POLICY refresh_tokens_modify_auth ON refresh_tokens
    FOR ALL
    USING (
        current_setting('app.current_user_id', true) IS NULL         -- login flow
        OR tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    )
    WITH CHECK (
        current_setting('app.current_user_id', true) IS NULL         -- login flow
        OR tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    );

COMMENT ON POLICY refresh_tokens_modify_auth ON refresh_tokens IS
    'Sprint 27 fix: allows INSERT/UPDATE/DELETE during login flow (when app.current_user_id GUC is NULL), '
    'enforces tenant match for normal authenticated flow, and bypasses for app_admin role.';

COMMIT;

-- =============================================================================
-- DOWN (rollback: restore strict policy from migration 000014)
-- =============================================================================
-- BEGIN;
-- DROP POLICY IF EXISTS refresh_tokens_select_auth ON refresh_tokens;
-- DROP POLICY IF EXISTS refresh_tokens_modify_auth ON refresh_tokens;
-- Recreate the original strict policy (will fail without tenant context for login)
-- CREATE POLICY tenant_isolation_select ON refresh_tokens
--     FOR SELECT USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);
-- CREATE POLICY tenant_isolation_modify ON refresh_tokens
--     FOR ALL USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid)
--     WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::uuid);
-- COMMIT;
