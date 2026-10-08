-- =============================================================================
-- Migration: 000023_app_admin_grants
-- Description: Grant app_admin table-level access on tables added after 000015
--              + ALTER DEFAULT PRIVILEGES so future tables auto-inherit
-- Author: FMCG Wallet
-- =============================================================================
-- Background (Sprint 30):
--   Migration 000015 created the `app_admin` role for RLS bypass and issued
--     GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public
--     TO app_admin;
--   That GRANT applies ONLY to tables that EXIST at the time of the grant.
--   Tables created after 000015 (guc_bind_audit, outbox_events, aging_snapshots,
--   aging_snapshot_runs, notifications) did not get the GRANT automatically.
--   Workers that need cross-tenant SELECT/INSERT/UPDATE on those tables
--   (outbox publisher, aging recalculator) were silently failing or returning
--   zero rows due to permission denied on the bare pool.
--
-- This migration:
--   1. Issues explicit GRANTs to app_admin on the post-000015 tables
--   2. Sets ALTER DEFAULT PRIVILEGES so tables created BY fmcg (the migration
--      runner) in the future automatically get the same grants to app_admin
--   3. Adds an admin_bypass policy for collection_routes which had RLS
--      enabled in 000014 but was missed by the 000015 admin_bypass loop
--   4. Idempotent: safe to re-run
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Explicit GRANTs on tables created after 000015
-- ---------------------------------------------------------------------------
-- These tables already have admin_bypass RLS policies (in their own migrations
-- 000016/000017/000018/000022), but the GRANT side was missing.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_admin') THEN
        -- guc_bind_audit (000016)
        IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'guc_bind_audit') THEN
            EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON guc_bind_audit TO app_admin';
        END IF;
        -- outbox_events (000017)
        IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'outbox_events') THEN
            EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON outbox_events TO app_admin';
        END IF;
        -- aging_snapshots (000018)
        IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'aging_snapshots') THEN
            EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON aging_snapshots TO app_admin';
        END IF;
        -- aging_snapshot_runs (000018, no RLS but worker writes here)
        IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'aging_snapshot_runs') THEN
            EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON aging_snapshot_runs TO app_admin';
        END IF;
        -- notifications (000022)
        IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'notifications') THEN
            EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON notifications TO app_admin';
        END IF;
    END IF;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

-- ---------------------------------------------------------------------------
-- 2. ALTER DEFAULT PRIVILEGES for future tables created by fmcg role
-- ---------------------------------------------------------------------------
-- Migrations run as the postgres superuser (or whichever role owns the db).
-- Default privileges must be set FOR the role that will OWN future tables.
-- The migration runner typically connects as superuser (postgres) which is
-- also the table owner for everything in this DB. So we set defaults for
-- that role. If the role doesn't exist (edge case), the DO block silently
-- skips.
DO $$
BEGIN
    -- Default for tables created by the current (migration runner) role
    EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public
             GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_admin';
    EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public
             GRANT USAGE, SELECT ON SEQUENCES TO app_admin';
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

-- ---------------------------------------------------------------------------
-- 3. admin_bypass RLS policy for collection_routes (missed in 000015)
-- ---------------------------------------------------------------------------
-- collection_routes has RLS via 000014 but no admin_bypass policy was added
-- in 000015's loop (it wasn't in the explicit array). Workers querying
-- cross-tenant collection routes would see RLS filter results.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'collection_routes') THEN
        EXECUTE 'DROP POLICY IF EXISTS collection_routes_admin_bypass ON collection_routes';
        EXECUTE 'CREATE POLICY collection_routes_admin_bypass ON collection_routes
                 TO app_admin USING (true) WITH CHECK (true)';
    END IF;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

COMMENT ON ROLE app_admin IS
    'Sprint 30: cross-tenant worker role. Granted access to tables added after migration 000015 '
    '(guc_bind_audit, outbox_events, aging_snapshots, aging_snapshot_runs, notifications) plus '
    'collection_routes admin_bypass policy. Workers use this role via SET LOCAL ROLE inside a tx '
    '(see internal/repository/postgres/RunInAdminTx).';

COMMIT;

-- =============================================================================
-- DOWN (rollback)
-- =============================================================================
-- BEGIN;
-- ALTER DEFAULT PRIVILEGES IN SCHEMA public
--     REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM app_admin;
-- ALTER DEFAULT PRIVILEGES IN SCHEMA public
--     REVOKE USAGE, SELECT ON SEQUENCES FROM app_admin;
-- DROP POLICY IF EXISTS collection_routes_admin_bypass ON collection_routes;
-- REVOKE SELECT, INSERT, UPDATE, DELETE ON guc_bind_audit FROM app_admin;
-- REVOKE SELECT, INSERT, UPDATE, DELETE ON outbox_events FROM app_admin;
-- REVOKE SELECT, INSERT, UPDATE, DELETE ON aging_snapshots FROM app_admin;
-- REVOKE SELECT, INSERT, UPDATE, DELETE ON aging_snapshot_runs FROM app_admin;
-- REVOKE SELECT, INSERT, UPDATE, DELETE ON notifications FROM app_admin;
-- COMMIT;