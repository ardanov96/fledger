-- =============================================================================
-- Migration: 000003_create_periods_and_transactions
-- Description: Create accounting_periods + transactions tables
-- Author: FMCG Wallet
-- =============================================================================
-- accounting_periods enables period close / accounting cycle (Fase 1A).
-- transactions group entries; idempotency_key + UNIQUE(tenant_id, key) for retries.
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- accounting_periods
-- ---------------------------------------------------------------------------
CREATE TABLE accounting_periods (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID            NOT NULL,
    period_start    DATE            NOT NULL,
    period_end      DATE            NOT NULL,
    status          TEXT            NOT NULL DEFAULT 'open',  -- 'open' | 'closing' | 'closed'
    closed_at       TIMESTAMPTZ,
    closed_by       UUID,
    snapshot_id     UUID,                                    -- balance sheet snapshot
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT accounting_periods_status_check CHECK (
        status IN ('open', 'closing', 'closed')
    ),
    CONSTRAINT accounting_periods_range_check CHECK (period_start <= period_end)
);

-- One period per tenant per (start, end) range
CREATE UNIQUE INDEX accounting_periods_tenant_range_unique
    ON accounting_periods(tenant_id, period_start, period_end);

-- Prevent overlapping periods per tenant using btree_gist
ALTER TABLE accounting_periods
    ADD CONSTRAINT accounting_periods_no_overlap
    EXCLUDE USING gist (
        tenant_id WITH =,
        daterange(period_start, period_end, '[]') WITH &&
    );

CREATE INDEX accounting_periods_tenant_status_idx
    ON accounting_periods(tenant_id, status);

CREATE TRIGGER accounting_periods_set_updated_at
    BEFORE UPDATE ON accounting_periods
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

-- ---------------------------------------------------------------------------
-- transactions
-- ---------------------------------------------------------------------------
CREATE TABLE transactions (
    id               UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key  TEXT           NOT NULL,
    status           TEXT           NOT NULL DEFAULT 'pending',
    description      TEXT,
    ref_type         TEXT,
    ref_id           UUID,
    initiator_id     UUID,
    tenant_id        UUID           NOT NULL,
    period_id        UUID           NOT NULL REFERENCES accounting_periods(id),
    metadata         JSONB          NOT NULL DEFAULT '{}',
    posted_at        TIMESTAMPTZ,
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ    NOT NULL DEFAULT now(),

    CONSTRAINT transactions_status_check CHECK (
        status IN ('pending', 'posted', 'failed', 'reversed')
    )
);

-- Idempotency: same key cannot produce two different transactions per tenant
CREATE UNIQUE INDEX transactions_tenant_idem_key_unique
    ON transactions(tenant_id, idempotency_key);

-- Common lookups
CREATE INDEX transactions_tenant_status_idx   ON transactions(tenant_id, status);
CREATE INDEX transactions_tenant_period_idx   ON transactions(tenant_id, period_id);
CREATE INDEX transactions_tenant_ref_idx      ON transactions(tenant_id, ref_type, ref_id)
    WHERE ref_id IS NOT NULL;
CREATE INDEX transactions_tenant_created_idx  ON transactions(tenant_id, created_at DESC);

CREATE TRIGGER transactions_set_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

COMMIT;
