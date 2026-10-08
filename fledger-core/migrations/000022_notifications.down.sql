BEGIN;

DROP POLICY IF EXISTS notifications_admin_bypass ON notifications;
DROP POLICY IF EXISTS notifications_tenant_isolation_modify ON notifications;
DROP POLICY IF EXISTS notifications_tenant_isolation_select ON notifications;

DROP TABLE IF EXISTS notifications;

COMMIT;
