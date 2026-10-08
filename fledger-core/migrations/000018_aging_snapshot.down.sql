BEGIN;

DROP TABLE IF EXISTS aging_snapshot_runs;
DROP POLICY IF EXISTS aging_snapshots_admin_bypass ON aging_snapshots;
DROP POLICY IF EXISTS aging_snapshots_tenant_isolation_modify ON aging_snapshots;
DROP POLICY IF EXISTS aging_snapshots_tenant_isolation_select ON aging_snapshots;

DROP TABLE IF EXISTS aging_snapshots;

COMMIT;
