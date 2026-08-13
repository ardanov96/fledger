-- =============================================================================
-- Migration: 000010_reconciler
-- Description: Trial balance reconciler run history (Fase 1B).
--
-- Tables:
--   * reconciler_runs — history of each reconciler invocation
--
-- The reconciler (Fase 1B) periodically:
--   1. Iterates all open/closing accounting periods
--   2. For each: SUM(debit) - SUM(credit) per period (trial balance)
--   3. Optional: runs HashChainVerifier on entries (tamper detection)
--   4. Records the result + any imbalance details
--
-- Sprint 10 also adds the per-account reconciler result table for granular
-- "which account imbalance" reporting (matters when one period has multiple
-- accounts with zero-sum, but a single account is off).
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- reconciler_runs — one row per reconciler invocation
-- ---------------------------------------------------------------------------
CREATE TABLE reconciler_runs (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID            NOT NULL,
    period_id       UUID            NOT NULL REFERENCES accounting_periods(id),
    started_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ,
    status          TEXT            NOT NULL DEFAULT 'running',
    total_debit     BIGINT          NOT NULL DEFAULT 0,
    total_credit    BIGINT          NOT NULL DEFAULT 0,
    imbalance       BIGINT          NOT NULL DEFAULT 0,
    hash_chain_ok   BOOLEAN,
    hash_chain_errors INTEGER       NOT NULL DEFAULT 0,
    triggered_by    TEXT            NOT NULL DEFAULT 'scheduler',
    metadata        JSONB           NOT NULL DEFAULT '{}',

    CONSTRAINT reconciler_runs_status_check CHECK (
        status IN ('running', 'balanced', 'imbalanced', 'tampered', 'error')
    ),
    CONSTRAINT reconciler_runs_triggered_by_check CHECK (
        triggered_by IN ('scheduler', 'manual', 'api')
    )
);

-- Hot-path: list recent runs per tenant
CREATE INDEX reconciler_runs_tenant_started_idx
    ON reconciler_runs(tenant_id, started_at DESC);

-- Hot-path: latest run for a period
CREATE INDEX reconciler_runs_period_idx
    ON reconciler_runs(period_id, started_at DESC);

-- Hot-path: filter by status (for alert dashboards)
CREATE INDEX reconciler_runs_status_idx
    ON reconciler_runs(status, started_at DESC)
    WHERE status IN ('imbalanced', 'tampered', 'error');

-- ---------------------------------------------------------------------------
-- reconciler_account_results — per-account breakdown
-- ---------------------------------------------------------------------------
CREATE TABLE reconciler_account_results (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id          UUID            NOT NULL REFERENCES reconciler_runs(id) ON DELETE CASCADE,
    period_id       UUID            NOT NULL REFERENCES accounting_periods(id),
    account_id      UUID            NOT NULL REFERENCES accounts(id),
    debit_minor     BIGINT          NOT NULL DEFAULT 0,
    credit_minor    BIGINT          NOT NULL DEFAULT 0,
    signed_balance  BIGINT          NOT NULL DEFAULT 0,
    entry_count     INTEGER         NOT NULL DEFAULT 0,
    currency        TEXT            NOT NULL DEFAULT 'IDR',

    CONSTRAINT reconciler_account_results_run_account_unique UNIQUE (run_id, account_id)
);

CREATE INDEX reconciler_account_results_run_idx
    ON reconciler_account_results(run_id);

CREATE INDEX reconciler_account_results_account_idx
    ON reconciler_account_results(account_id, period_id);

COMMENT ON TABLE reconciler_runs IS
    'Trial balance reconciler run history per period (Fase 1B).';
COMMENT ON TABLE reconciler_account_results IS
    'Per-account breakdown of a reconciler run — which account is off if imbalance found.';

COMMIT;

-- =============================================================================
-- DOWN (rollback only, not auto-applied)
-- =============================================================================
-- DROP TABLE IF EXISTS reconciler_account_results;
-- DROP TABLE IF EXISTS reconciler_runs;
