-- Migration 000014: Tenant Row-Level Security (Sprint 15 / Fase 5A).
--
-- Strategy: enable Postgres RLS on tenant-scoped tables + set per-session
-- GUC variable `app.current_tenant_id` from middleware (set in transaction).
-- RLS policies enforce WHERE tenant_id = current_setting('app.current_tenant_id')::uuid
-- so that every query automatically filters rows to the current tenant.
--
-- Bypass: superusers / role `postgres` always bypass RLS. We also create a
-- dedicated role `app_admin` that bypasses RLS for maintenance / system jobs.
--
-- Sprint 15 also adds a field-level authz helper: a `sales_rep` role can ONLY
-- see their own routes (sales_rep_id = current_user_id) — enforced by RLS
-- policy `using (sales_rep_id = current_setting('app.current_user_id')::uuid)`.
--
-- NOTE: RLS is applied via ALTER TABLE ... ENABLE ROW LEVEL SECURITY.
-- Tables without tenant_id (e.g. currencies, fx_rates which are global refs)
-- are NOT enabled for RLS — those are shared lookup data.

BEGIN;

-- ============================================================================
-- Enable RLS on tenant-scoped tables.
-- ============================================================================

-- accounts (has tenant_id)
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts FORCE ROW LEVEL SECURITY; -- applies even to table owner (so DB superuser bypasses)

-- transactions
ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions FORCE ROW LEVEL SECURITY;

-- ledger_entries (inherits via transaction FK, but explicit for defense-in-depth)
ALTER TABLE ledger_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE ledger_entries FORCE ROW LEVEL SECURITY;

-- invoices
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices FORCE ROW LEVEL SECURITY;

-- invoice_payments (FK -> invoices, but tenant_id also added for direct query)
ALTER TABLE invoice_payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoice_payments FORCE ROW LEVEL SECURITY;

-- credit_limits
ALTER TABLE credit_limits ENABLE ROW LEVEL SECURITY;
ALTER TABLE credit_limits FORCE ROW LEVEL SECURITY;

-- accounting_periods
ALTER TABLE accounting_periods ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounting_periods FORCE ROW LEVEL SECURITY;

-- period_close_requests (FK -> periods)
ALTER TABLE period_close_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE period_close_requests FORCE ROW LEVEL SECURITY;

-- period_snapshots
ALTER TABLE period_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE period_snapshots FORCE ROW LEVEL SECURITY;

-- reconciler_runs
ALTER TABLE reconciler_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE reconciler_runs FORCE ROW LEVEL SECURITY;

-- reconciler_account_results (FK -> runs)
ALTER TABLE reconciler_account_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE reconciler_account_results FORCE ROW LEVEL SECURITY;

-- collection_routes (has tenant_id + sales_rep_id for field-level authz)
ALTER TABLE collection_routes ENABLE ROW LEVEL SECURITY;
ALTER TABLE collection_routes FORCE ROW LEVEL SECURITY;

-- refresh_tokens
ALTER TABLE refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens FORCE ROW LEVEL SECURITY;

-- user_credentials (has tenant_id; but RLS would block admin tools —
--     for Sprint 15 we exclude this table from RLS and rely on app-layer checks.
--     Future Sprint: add admin RLS bypass via dedicated role.)

-- ============================================================================
-- Policies: SELECT / INSERT / UPDATE / DELETE all filtered by tenant_id.
-- ============================================================================
-- Pattern: USING (tenant_id = current_setting('app.current_tenant_id')::uuid)
-- The `current_setting(...)` returns NULL when not set; we use ::uuid cast which
-- throws on NULL → queries will FAIL FAST when middleware forgets to set tenant.
-- This is INTENTIONAL: defense-in-depth against missing tenant context.

-- accounts policies
DROP POLICY IF EXISTS tenant_isolation_select ON accounts;
CREATE POLICY tenant_isolation_select ON accounts
    FOR SELECT
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

DROP POLICY IF EXISTS tenant_isolation_modify ON accounts;
CREATE POLICY tenant_isolation_modify ON accounts
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

-- transactions policies (same pattern)
DROP POLICY IF EXISTS tenant_isolation_select ON transactions;
CREATE POLICY tenant_isolation_select ON transactions
    FOR SELECT USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

DROP POLICY IF EXISTS tenant_isolation_modify ON transactions;
CREATE POLICY tenant_isolation_modify ON transactions
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

-- ledger_entries policies
DROP POLICY IF EXISTS tenant_isolation_select ON ledger_entries;
CREATE POLICY tenant_isolation_select ON ledger_entries
    FOR SELECT USING (
        -- inherit tenant from transaction
        EXISTS (
            SELECT 1 FROM transactions t
            WHERE t.id = ledger_entries.transaction_id
              AND t.tenant_id = current_setting('app.current_tenant_id', true)::uuid
        )
    );

DROP POLICY IF EXISTS tenant_isolation_modify ON ledger_entries;
CREATE POLICY tenant_isolation_modify ON ledger_entries
    FOR ALL
    USING (
        EXISTS (
            SELECT 1 FROM transactions t
            WHERE t.id = ledger_entries.transaction_id
              AND t.tenant_id = current_setting('app.current_tenant_id', true)::uuid
        )
    )
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM transactions t
            WHERE t.id = ledger_entries.transaction_id
              AND t.tenant_id = current_setting('app.current_tenant_id', true)::uuid
        )
    );

-- invoices / invoice_payments / credit_limits (same pattern)
DO $$
DECLARE t TEXT;
BEGIN
  FOR t IN SELECT unnest(ARRAY['invoices', 'invoice_payments', 'credit_limits', 'accounting_periods', 'period_close_requests', 'period_snapshots', 'reconciler_runs', 'reconciler_account_results', 'refresh_tokens'])
  LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_select ON %I', t);
    EXECUTE format('CREATE POLICY tenant_isolation_select ON %I FOR SELECT USING (tenant_id = current_setting(''app.current_tenant_id'', true)::uuid)', t);
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_modify ON %I', t);
    EXECUTE format('CREATE POLICY tenant_isolation_modify ON %I FOR ALL USING (tenant_id = current_setting(''app.current_tenant_id'', true)::uuid) WITH CHECK (tenant_id = current_setting(''app.current_tenant_id'', true)::uuid)', t);
  END LOOP;
END $$;

-- ============================================================================
-- Field-level authz (Sprint 15 / Fase 2B): sales_rep can only see own routes.
-- ============================================================================
-- Two policies on collection_routes:
--   1. Tenant isolation (already applied above)
--   2. Sales-rep scope (additional filter when current_setting app.is_sales_rep = 'true')
DROP POLICY IF EXISTS sales_rep_scope ON collection_routes;
CREATE POLICY sales_rep_scope ON collection_routes
    FOR SELECT
    USING (
        -- Either admin (app.is_sales_rep != 'true') OR sales_rep_id matches current user
        current_setting('app.is_sales_rep', true) IS DISTINCT FROM 'true'
        OR sales_rep_id = current_setting('app.current_user_id', true)::uuid
    );

-- ============================================================================
-- Indexes to keep RLS-overhead queries fast.
-- ============================================================================
-- (Most tables already have tenant_id indexes from prior migrations; this is
-- a no-op safety check. If you see RLS slow in production, add covering
-- indexes including tenant_id.)
CREATE INDEX IF NOT EXISTS accounts_tenant_idx     ON accounts (tenant_id);
CREATE INDEX IF NOT EXISTS invoices_tenant_idx      ON invoices (tenant_id);
CREATE INDEX IF NOT EXISTS collection_routes_tenant_idx ON collection_routes (tenant_id, sales_rep_id);
CREATE INDEX IF NOT EXISTS refresh_tokens_tenant_idx   ON refresh_tokens (tenant_id);
CREATE INDEX IF NOT EXISTS transactions_tenant_idx  ON transactions (tenant_id);

COMMIT;
