-- Migration 000016: GUC Bind Audit Trail (Sprint 23 / 22B.5, closes
-- ADR-0006 open follow-up #2).
--
-- Strategy: every SetTenantContext call now INSERTs a row to
-- guc_bind_audit BEFORE the user closure runs. This creates an
-- append-only audit log of every tenant GUC binding — forensics
-- trail for cross-tenant investigation.
--
-- RLS: regular `fmcg` role sees only own tenant's binds (defense
-- in depth). Admin role `app_admin` sees ALL binds via the
-- migration 00015 policy pattern.

BEGIN;

-- ============================================================================
-- guc_bind_audit table.
-- ============================================================================
CREATE TABLE IF NOT EXISTS guc_bind_audit (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    is_sales_rep BOOLEAN    NOT NULL,
    request_id  TEXT,                           -- W3C trace correlation
    bound_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Hot-path: list binds for one tenant, newest first
CREATE INDEX IF NOT EXISTS guc_bind_audit_tenant_time_idx
    ON guc_bind_audit (tenant_id, bound_at DESC);

-- Forensic: who-did-what across tenants
CREATE INDEX IF NOT EXISTS guc_bind_audit_user_time_idx
    ON guc_bind_audit (user_id, bound_at DESC);

-- Optional: time-window queries across all binds
CREATE INDEX IF NOT EXISTS guc_bind_audit_bound_at_idx
    ON guc_bind_audit (bound_at DESC);

-- ============================================================================
-- RLS: same pattern as migration 00014 + app_admin bypass (migration 00015).
-- ============================================================================

ALTER TABLE guc_bind_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE guc_bind_audit FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS guc_bind_audit_tenant_select ON guc_bind_audit;
CREATE POLICY guc_bind_audit_tenant_select ON guc_bind_audit
    FOR SELECT USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

DROP POLICY IF EXISTS guc_bind_audit_tenant_modify ON guc_bind_audit;
CREATE POLICY guc_bind_audit_tenant_modify ON guc_bind_audit
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

-- Admin bypass: app_admin sees ALL binds (cross-tenant investigations).
DROP POLICY IF EXISTS guc_bind_audit_admin_bypass ON guc_bind_audit;
CREATE POLICY guc_bind_audit_admin_bypass ON guc_bind_audit
    TO app_admin USING (true) WITH CHECK (true);

-- ============================================================================
-- Operational note (ADR-0008 open follow-up #1):
-- This table will grow unbounded. Plan for monthly partitioning
-- (similar to `login_attempts` once volume grows). Index on
-- bound_at already makes partition pruning efficient.
-- ============================================================================

COMMIT;
