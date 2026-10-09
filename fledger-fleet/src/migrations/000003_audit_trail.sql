-- =============================================================================
-- 000003_audit_trail.sql
-- Sprint Guardrail §7.3 — Audit trail for DO status transitions.
--
-- Adds:
--   1. fleet_do_status_history — append-only log of every status change with
--      actor (user/driver UUID), optional note, and timestamp.
--   2. fleet_delivery_orders.actor_id — denormalised "last actor" so the
--      common query (DO + its latest actor) is index-friendly.
-- =============================================================================

CREATE TABLE IF NOT EXISTS fleet_do_status_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    do_id UUID NOT NULL REFERENCES fleet_delivery_orders(id) ON DELETE CASCADE,
    from_status VARCHAR(30),                       -- nullable for first INSERT
    to_status VARCHAR(30) NOT NULL,
    actor_id UUID,                                 -- user/driver who made the change
    actor_type VARCHAR(20) NOT NULL DEFAULT 'user',-- 'user' | 'driver' | 'system' | 'worker'
    note TEXT,                                     -- e.g. "POD partial: 2 bocor"
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fleet_do_history_do
    ON fleet_do_status_history(do_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_fleet_do_history_actor
    ON fleet_do_status_history(actor_id, created_at DESC);

ALTER TABLE fleet_delivery_orders
    ADD COLUMN IF NOT EXISTS actor_id UUID;

-- Also add actor columns to trip + vehicle for completeness.
ALTER TABLE fleet_trips
    ADD COLUMN IF NOT EXISTS actor_id UUID;

ALTER TABLE fleet_vehicles
    ADD COLUMN IF NOT EXISTS updated_by UUID;

INSERT INTO fleet_schema_migrations (version, description)
VALUES (3, 'audit trail (status history + actor_id columns)')
ON CONFLICT (version) DO NOTHING;