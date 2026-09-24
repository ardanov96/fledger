-- =============================================================================
-- Migration: 000017_outbox
-- Description: Outbox events table for transactional outbox pattern (Sprint 24 / Fase 4A)
-- Author: FMCG Wallet
-- =============================================================================
-- The outbox pattern guarantees at-least-once event delivery:
--   1. Business write + outbox INSERT in SAME tx → atomic.
--   2. Background publisher polls unpublished rows, sends to NATS, marks published.
--   3. On publisher crash, no events lost — they stay unpublished.
--   4. On consumer crash, NATS redelivers — idempotent handler handles dups.
--
-- This migration adds only the storage layer; the publisher worker is wired in
-- cmd/worker/main.go and the TransferService hook is in transfer_service.go.
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- outbox_events
-- ---------------------------------------------------------------------------
CREATE TABLE outbox_events (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID            NOT NULL,
    aggregate_type  TEXT            NOT NULL,            -- 'transfer' | 'invoice' | 'period' | ...
    aggregate_id    UUID            NOT NULL,            -- business entity id
    event_type      TEXT            NOT NULL,            -- 'transfer.posted' | ...
    subject         TEXT            NOT NULL,            -- NATS subject ('fmcg.transfer.posted')
    payload         JSONB           NOT NULL,            -- event data, opaque to outbox
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ,                        -- NULL = unpublished
    attempts        INT             NOT NULL DEFAULT 0,  -- publish failure count
    last_error      TEXT,                                -- most recent publish error
    metadata        JSONB           NOT NULL DEFAULT '{}',

    CONSTRAINT outbox_events_aggregate_type_check CHECK (
        aggregate_type IN ('transfer','invoice','period','payment','reconciler','auth')
    )
);

-- Hot path: poll for unpublished events (most queries filter on published_at IS NULL)
CREATE INDEX outbox_events_unpublished_idx
    ON outbox_events (created_at)
    WHERE published_at IS NULL;

-- Lookup by aggregate (for audit / debugging)
CREATE INDEX outbox_events_aggregate_idx
    ON outbox_events (tenant_id, aggregate_type, aggregate_id);

-- Tenant-scoped RLS (consistent with other domain tables — see migration 000014)
ALTER TABLE outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS outbox_events_tenant_isolation_select ON outbox_events;
CREATE POLICY outbox_events_tenant_isolation_select ON outbox_events
    FOR SELECT
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

DROP POLICY IF EXISTS outbox_events_tenant_isolation_modify ON outbox_events;
CREATE POLICY outbox_events_tenant_isolation_modify ON outbox_events
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

-- app_admin bypass role (Sprint 22A.4 / ADR-0007) — outbox publisher runs as
-- fmcg but uses the same RLS policies; ops queries via app_admin when needed.
DROP POLICY IF EXISTS outbox_events_admin_bypass ON outbox_events;
CREATE POLICY outbox_events_admin_bypass ON outbox_events
    TO app_admin
    USING (true) WITH CHECK (true);

COMMIT;
