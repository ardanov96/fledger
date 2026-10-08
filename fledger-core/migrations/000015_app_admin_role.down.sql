-- Migration 000015 (down): app_admin Postgres role rollback
--
-- Inverse of 000015_app_admin_role.up.sql:
--   1. Drop admin bypass policies on all RLS tables
--   2. Revoke sequence + table grants
--   3. Revoke app_admin from fmcg
--   4. Drop the app_admin role itself
--
-- Idempotent: all DROP statements use IF EXISTS / DROP POLICY IF EXISTS.
--
-- WARNING: This migration does NOT touch the underlying tenant-isolation
-- policies defined in 000014 (those remain intact). After rollback, the
-- `app_admin` role no longer exists and cannot bypass RLS — only the
-- Postgres superuser (e.g. role `postgres`) can.

BEGIN;

-- ============================================================================
-- 1. Drop admin bypass policies on every RLS-enabled table.
-- ============================================================================
-- Must drop before the role itself, otherwise policy drops would fail with
-- "policy ... does not exist" warnings (which are harmless but noisy).
DO $$
DECLARE
  rls_tables TEXT[] := ARRAY[
    'accounts',
    'transactions',
    'ledger_entries',
    'accounting_periods',
    'invoices',
    'invoice_payments',
    'credit_limits',
    'period_close_requests',
    'period_snapshots',
    'reconciler_runs',
    'fx_rates'
  ];
  t TEXT;
BEGIN
  FOREACH t IN ARRAY rls_tables
  LOOP
    EXECUTE format('DROP POLICY IF EXISTS %I_admin_bypass ON %I', t, t);
  END LOOP;
END
$$;

-- ============================================================================
-- 2. Revoke grants (sequence + table).
-- ============================================================================
REVOKE USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public FROM app_admin;
REVOKE SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public FROM app_admin;

-- ============================================================================
-- 3. Revoke fmcg's ability to SET ROLE app_admin.
-- ============================================================================
REVOKE app_admin FROM fmcg;

-- ============================================================================
-- 4. Drop the app_admin role.
-- ============================================================================
-- DROP ROLE would fail if any other role has been granted app_admin. To make
-- this idempotent in environments where the role may already be missing
-- (e.g. partial rollback), we use IF EXISTS.
DROP ROLE IF EXISTS app_admin;

COMMIT;
