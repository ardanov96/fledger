-- Migration 000017 rollback: drop guc_bind_audit table.
--
-- Drops policies explicitly (defensive — DROP TABLE IF EXISTS would
-- also cascade policies, but listing them makes operator's intent
-- auditable in rollback log).

BEGIN;

DROP POLICY IF EXISTS guc_bind_audit_tenant_select  ON guc_bind_audit;
DROP POLICY IF EXISTS guc_bind_audit_tenant_modify ON guc_bind_audit;
DROP POLICY IF EXISTS guc_bind_audit_admin_bypass   ON guc_bind_audit;

DROP TABLE IF EXISTS guc_bind_audit;

COMMIT;
