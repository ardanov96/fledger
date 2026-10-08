-- =============================================================================
-- Migration: 000009_period_approval
-- Description: Period close approval workflow + period-end snapshot
--              (Fase 1A — Period Close, two-step approval).
--
-- Tables:
--   * period_close_requests — workflow: pending → approved | rejected
--   * period_snapshots      — frozen state per account saat close di-approve
--
-- State machine:
--   accounting_periods.status (open/closing/closed)
--      open    --RequestClose-->   closing
--      closing --ApproveClose-->   closed
--      closing --RejectClose-->    open
--      closed  --Reopen (admin)--> open
--
-- Snapshot integrity: snapshots di-buat dalam tx yang sama dengan update
-- period ke status=closed. Kalau tx gagal, snapshot tidak ter-commit.
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- period_close_requests — workflow record
-- ---------------------------------------------------------------------------
CREATE TABLE period_close_requests (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID            NOT NULL,
    period_id       UUID            NOT NULL REFERENCES accounting_periods(id),
    requester_id    UUID            NOT NULL,                -- user who initiated
    approver_id     UUID,                                  -- nullable until decided
    status          TEXT            NOT NULL DEFAULT 'pending',
    trial_balance_ok BOOLEAN        NOT NULL DEFAULT FALSE,
    total_debit     BIGINT          NOT NULL DEFAULT 0,
    total_credit    BIGINT          NOT NULL DEFAULT 0,
    imbalance       BIGINT          NOT NULL DEFAULT 0,
    rejection_reason TEXT,
    requested_at    TIMESTAMPTZ     NOT NULL DEFAULT now(),
    decided_at      TIMESTAMPTZ,
    metadata        JSONB           NOT NULL DEFAULT '{}',

    CONSTRAINT period_close_requests_status_check CHECK (
        status IN ('pending', 'approved', 'rejected', 'cancelled')
    )
);

-- One pending request per (period, requester) — biar idempotent kalau user klik 2x
CREATE UNIQUE INDEX period_close_requests_pending_unique
    ON period_close_requests(period_id, requester_id)
    WHERE status = 'pending';

-- Hot-path: list pending requests per tenant
CREATE INDEX period_close_requests_tenant_status_idx
    ON period_close_requests(tenant_id, status, requested_at DESC);

-- Hot-path: lookup by period
CREATE INDEX period_close_requests_period_idx
    ON period_close_requests(period_id);

-- ---------------------------------------------------------------------------
-- period_snapshots — frozen account balances saat close approved
-- ---------------------------------------------------------------------------
CREATE TABLE period_snapshots (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID            NOT NULL,
    period_id       UUID            NOT NULL REFERENCES accounting_periods(id),
    request_id      UUID            NOT NULL REFERENCES period_close_requests(id),
    account_id      UUID            NOT NULL REFERENCES accounts(id),
    balance_minor   BIGINT          NOT NULL,                -- signed (debit-positive)
    currency        TEXT            NOT NULL DEFAULT 'IDR',
    entry_count     INTEGER         NOT NULL DEFAULT 0,      -- audit context
    snapshot_at     TIMESTAMPTZ     NOT NULL DEFAULT now(),
    metadata        JSONB           NOT NULL DEFAULT '{}',

    CONSTRAINT period_snapshots_request_account_unique UNIQUE (request_id, account_id)
);

-- Hot-path: query snapshots by period (e.g. balance sheet per period)
CREATE INDEX period_snapshots_period_account_idx
    ON period_snapshots(period_id, account_id);

-- Hot-path: query snapshots by tenant (e.g. all closed periods for tenant)
CREATE INDEX period_snapshots_tenant_period_idx
    ON period_snapshots(tenant_id, period_id);

COMMENT ON TABLE period_close_requests IS
    'Period close workflow with two-step approval (request → approve/reject).';
COMMENT ON TABLE period_snapshots IS
    'Frozen account balances per period saat close approved; immutable.';

COMMIT;

-- =============================================================================
-- DOWN (untuk rollback manual; tidak di-apply otomatis)
-- =============================================================================
-- DROP TABLE IF EXISTS period_snapshots;
-- DROP TABLE IF EXISTS period_close_requests;
