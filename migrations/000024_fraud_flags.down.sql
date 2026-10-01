-- Migration 000024 (down): drop fraud_flags table and policies.
BEGIN;

DROP POLICY IF EXISTS fraud_flags_tenant_isolation_modify ON fraud_flags;
DROP POLICY IF EXISTS fraud_flags_tenant_isolation_select ON fraud_flags;

DROP TABLE IF EXISTS fraud_flags;

COMMIT;