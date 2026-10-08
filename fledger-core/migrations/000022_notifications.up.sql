-- =============================================================================
-- Migration: 000022_notifications
-- Description: Notifications table for event-driven notifications (Sprint 28 / Fase 8)
-- Author: FMCG Wallet
-- =============================================================================
-- Notifications are created by the NotificationWorker when it consumes outbox
-- events from NATS (Sprint 24). End users see them via GET /v1/notifications.
--
-- Schema:
--   id          UUID PK
--   tenant_id   UUID NOT NULL (for RLS - tenant isolation)
--   user_id     UUID NOT NULL (recipient; FK to user_credentials logically)
--   type        TEXT NOT NULL  ('transfer.posted', 'invoice.created', etc.)
--   title       TEXT NOT NULL  (short human-readable headline)
--   body        JSONB          (full event payload + extra context)
--   severity    TEXT NOT NULL  ('info' | 'warn' | 'critical')
--   status      TEXT NOT NULL  ('unread' | 'read' | 'archived')
--   created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
--   read_at     TIMESTAMPTZ                  -- set when user marks as read
--
-- Indexes:
--   - (tenant_id, user_id, created_at DESC) - the hot path for "my notifications"
--   - (status) where status = 'unread' - for badge count queries
--   - (tenant_id, type, created_at DESC) - admin views "notifications by type"
--
-- RLS: enabled with strict tenant-isolation policy (unlike refresh_tokens,
-- this DOES need tenant context since users are always authenticated).
-- =============================================================================

BEGIN;

CREATE TABLE notifications (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID         NOT NULL,
    user_id     UUID         NOT NULL,
    type        TEXT         NOT NULL,
    title       TEXT         NOT NULL,
    body        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    severity    TEXT         NOT NULL DEFAULT 'info',
    status      TEXT         NOT NULL DEFAULT 'unread',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    read_at     TIMESTAMPTZ,

    CONSTRAINT notifications_severity_valid CHECK (severity IN ('info', 'warn', 'critical')),
    CONSTRAINT notifications_status_valid  CHECK (status    IN ('unread', 'read', 'archived'))
);

-- Grant basic table/sequence permissions to fmcg BEFORE enabling RLS.
-- Without this, fmcg gets "permission denied" on the table.
-- (app_admin bypasses RLS via separate grant; fmcg relies on table-level
--  GRANTs + RLS policies for tenant isolation.)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fmcg') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON notifications TO fmcg;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO fmcg;
    END IF;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

CREATE INDEX notifications_user_unread_idx
    ON notifications (tenant_id, user_id, created_at DESC)
    WHERE status = 'unread';

CREATE INDEX notifications_user_all_idx
    ON notifications (tenant_id, user_id, created_at DESC);

CREATE INDEX notifications_type_idx
    ON notifications (tenant_id, type, created_at DESC);

-- RLS + app_admin bypass (consistent with other tenant-scoped tables)
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS notifications_tenant_isolation_select ON notifications;
CREATE POLICY notifications_tenant_isolation_select ON notifications
    FOR SELECT USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    );

DROP POLICY IF EXISTS notifications_tenant_isolation_modify ON notifications;
CREATE POLICY notifications_tenant_isolation_modify ON notifications
    FOR ALL USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    )
    WITH CHECK (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    );

DROP POLICY IF EXISTS notifications_admin_bypass ON notifications;
CREATE POLICY notifications_admin_bypass ON notifications
    TO app_admin
    USING (true) WITH CHECK (true);

COMMENT ON TABLE notifications IS
    'Sprint 28: in-app notification feed. Created by NotificationWorker on NATS outbox events.';

COMMIT;

-- =============================================================================
-- DOWN (rollback)
-- =============================================================================
-- BEGIN;
-- DROP POLICY IF EXISTS notifications_admin_bypass ON notifications;
-- DROP POLICY IF EXISTS notifications_tenant_isolation_modify ON notifications;
-- DROP POLICY IF EXISTS notifications_tenant_isolation_select ON notifications;
-- DROP TABLE IF EXISTS notifications;
-- COMMIT;
