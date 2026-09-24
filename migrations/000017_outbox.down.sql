BEGIN;

DROP POLICY IF EXISTS outbox_events_admin_bypass ON outbox_events;
DROP POLICY IF EXISTS outbox_events_tenant_isolation_modify ON outbox_events;
DROP POLICY IF EXISTS outbox_events_tenant_isolation_select ON outbox_events;

DROP TABLE IF EXISTS outbox_events;

COMMIT;
