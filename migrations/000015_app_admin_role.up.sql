-- =============================================================================
-- Migration 000015: app_admin Postgres role for RLS bypass
-- =============================================================================
--
-- Purpose: Create a separate Postgres role (`app_admin`) that bypasses the
-- tenant-isolation RLS policies defined in migration 000014. This enables
-- legitimate administrative operations (cross-tenant reports, manual data
-- fixes, debugging) without weakening security for the regular app user.
--
-- Design:
--   1. `fmcg` role = regular app user, RLS-enforced (default).
--   2. `app_admin` role = NOINHERIT, separate role.
--   3. `fmcg` is granted `app_admin` so it can `SET ROLE app_admin` per-tx.
--   4. RLS policies on tenant-scoped tables allow `app_admin` (USING (true))
--      while still blocking `fmcg` (USING tenant_id = current_setting(...)).
--   5. Production usage: `BEGIN; SET LOCAL ROLE app_admin; <queries>; COMMIT;`
--      — `SET LOCAL ROLE` is tx-scoped, no leak across connections.
--
-- Deployment order:
--   1. Apply this migration (creates role + policies).
--   2. Deploy app code that uses `SET LOCAL ROLE app_admin` when needed
--      (gated by env var `ADMIN_MODE_ENABLED=true`).
--
-- Affected tables (11 tenant-scoped): accounts, transactions, ledger_entries,
-- accounting_periods, invoices, invoice_payments, credit_limits,
-- period_close_requests, period_snapshots, reconciler_runs, fx_rates.
--
-- Tables NOT affected (public/global, no tenant_id):
--   currencies, tenants, user_credentials, refresh_tokens, audit_logs,
--   collection_routes, route_stops, collection_events, settlements.
-- =============================================================================

BEGIN;

-- 1. Create app_admin role (NOINHERIT so fmcg must explicitly SET ROLE)
CREATE ROLE app_admin NOINHERIT;

-- 2. Grant fmcg permission to switch to app_admin
GRANT app_admin TO fmcg;

-- 3. Grant basic table privileges (app_admin needs SELECT/INSERT/UPDATE/DELETE)
GRANT SELECT, INSERT, UPDATE, DELETE
ON ALL TABLES IN SCHEMA public
TO app_admin;

-- 4. Grant sequence usage (for INSERTs with auto-generated IDs)
GRANT USAGE, SELECT
ON ALL SEQUENCES IN SCHEMA public
TO app_admin;

-- 5. For each RLS-enabled table, create admin bypass policy.
-- These run AFTER tenant_isolation policies from migration 000014.
-- The app_admin policy USING (true) means admin sees ALL rows.
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
    -- Drop pre-existing admin policy if any (idempotent)
    EXECUTE format('DROP POLICY IF EXISTS %I_admin_bypass ON %I', t, t);
    -- Create new admin bypass policy
    EXECUTE format(
      'CREATE POLICY %I_admin_bypass ON %I TO app_admin USING (true) WITH CHECK (true)',
      t, t
    );
  END LOOP;
END
$$;

COMMIT;

-- =============================================================================
-- Verification queries (run manually after applying):
--
-- 1. Verify role exists:
--    SELECT rolname FROM pg_roles WHERE rolname = 'app_admin';
--    -- Expected: 1 row
--
-- 2. Verify fmcg can SET ROLE app_admin:
--    SET ROLE app_admin;
--    SELECT current_user;  -- Expected: app_admin
--    RESET ROLE;
--
-- 3. Verify admin sees all tenants (bypasses RLS):
--    SET ROLE app_admin;
--    SELECT tenant_id, COUNT(*) FROM accounts GROUP BY tenant_id;
--    RESET ROLE;
--    -- Expected: counts for ALL tenants (not just one)
--
-- 4. Verify regular fmcg role STILL scoped to its tenant:
--    RESET ROLE;  -- back to fmcg
--    SET app.current_tenant_id = '<tenant-a-uuid>';
--    SELECT * FROM accounts;  -- Expected: only tenant A's accounts
-- =============================================================================
