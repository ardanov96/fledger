-- =============================================================================
-- Migration: 000018_aging_snapshot
-- Description: Materialized aging snapshot table (Sprint 25 / Fase 4D)
-- Author: FMCG Wallet
-- =============================================================================
-- The aging summary (`GET /v1/customers/{id}/aging`) currently queries the
-- live SQL view `v_invoice_aging` on every request. For large tenants with
-- many customers, that scan is expensive (full table scan + GROUP BY).
--
-- This migration adds `aging_snapshots` — a denormalized cache populated
-- by the AgingRecalculator worker (Sprint 25). The worker runs nightly
-- (configurable via AGING_RECALC_INTERVAL env), truncates + inserts in a
-- single transaction, and the API prefers reading from this table.
--
-- Trade-off:
--   - Snapshot is bounded-fresh (worker interval). Stale by up to 1 day
--     for nightly runs. For real-time data the API still has the live view.
--   - AgingRecalculator truncates + re-inserts (not incremental) for MVP
--     simplicity. Switching to incremental updates is a future optimization.
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- aging_snapshots
-- ---------------------------------------------------------------------------
CREATE TABLE aging_snapshots (
    tenant_id         UUID         NOT NULL,
    customer_id       UUID         NOT NULL,
    bucket            TEXT         NOT NULL,                    -- 'current' | 'd_1_7' | 'd_8_30' | 'd_31_60' | 'd_61_90' | 'd_90_plus'
    invoice_count     INT          NOT NULL DEFAULT 0,
    outstanding_minor BIGINT       NOT NULL DEFAULT 0,           -- signed; negative = credit balance (overpayment)
    snapshot_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),       -- when this row was computed
    snapshot_run_id   UUID         NOT NULL,                    -- ties all rows from one worker run

    CONSTRAINT aging_snapshots_pk PRIMARY KEY (tenant_id, customer_id, bucket),
    CONSTRAINT aging_snapshots_bucket_check CHECK (
        bucket IN ('current','d_1_7','d_8_30','d_31_60','d_61_90','d_90_plus')
    ),
    CONSTRAINT aging_snapshots_count_nonneg CHECK (invoice_count >= 0)
);

-- Hot path: GET /v1/customers/{id}/aging — filter by (tenant_id, customer_id)
CREATE INDEX aging_snapshots_tenant_customer_idx
    ON aging_snapshots (tenant_id, customer_id);

-- Tenant-wide report (when customer_id = ""): filter by tenant_id
CREATE INDEX aging_snapshots_tenant_idx
    ON aging_snapshots (tenant_id);

-- Cleanup helper: drop stale snapshots older than N runs (admin tool)
CREATE INDEX aging_snapshots_run_idx
    ON aging_snapshots (snapshot_run_id);

-- ---------------------------------------------------------------------------
-- aging_snapshot_runs — audit trail of worker executions
-- ---------------------------------------------------------------------------
CREATE TABLE aging_snapshot_runs (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    started_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    finished_at       TIMESTAMPTZ,
    tenants_processed INT          NOT NULL DEFAULT 0,
    customers_processed INT        NOT NULL DEFAULT 0,
    rows_written      INT          NOT NULL DEFAULT 0,
    duration_ms       INT,
    status            TEXT         NOT NULL DEFAULT 'running',  -- 'running' | 'ok' | 'failed'
    error             TEXT,

    CONSTRAINT aging_snapshot_runs_status_check CHECK (
        status IN ('running','ok','failed')
    )
);

-- Latest run lookup (admin dashboard, /readyz optional)
CREATE INDEX aging_snapshot_runs_started_idx
    ON aging_snapshot_runs (started_at DESC);

-- ---------------------------------------------------------------------------
-- RLS — same pattern as other tenant-scoped tables
-- ---------------------------------------------------------------------------
ALTER TABLE aging_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE aging_snapshots FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS aging_snapshots_tenant_isolation_select ON aging_snapshots;
CREATE POLICY aging_snapshots_tenant_isolation_select ON aging_snapshots
    FOR SELECT
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

DROP POLICY IF EXISTS aging_snapshots_tenant_isolation_modify ON aging_snapshots;
CREATE POLICY aging_snapshots_tenant_isolation_modify ON aging_snapshots
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

DROP POLICY IF EXISTS aging_snapshots_admin_bypass ON aging_snapshots;
CREATE POLICY aging_snapshots_admin_bypass ON aging_snapshots
    TO app_admin
    USING (true) WITH CHECK (true);

-- Snapshot runs are operator-only (worker writes, admin reads).
-- No RLS needed since worker runs with elevated privileges; ops dashboard
-- runs as app_admin which already has bypass via existing pattern.

COMMIT;
