-- Migration 000023 (down): reverse app_admin grants added in 000023.
BEGIN;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM app_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    REVOKE USAGE, SELECT ON SEQUENCES FROM app_admin;

DROP POLICY IF EXISTS collection_routes_admin_bypass ON collection_routes;

REVOKE SELECT, INSERT, UPDATE, DELETE ON guc_bind_audit FROM app_admin;
REVOKE SELECT, INSERT, UPDATE, DELETE ON outbox_events FROM app_admin;
REVOKE SELECT, INSERT, UPDATE, DELETE ON aging_snapshots FROM app_admin;
REVOKE SELECT, INSERT, UPDATE, DELETE ON aging_snapshot_runs FROM app_admin;
REVOKE SELECT, INSERT, UPDATE, DELETE ON notifications FROM app_admin;

COMMIT;