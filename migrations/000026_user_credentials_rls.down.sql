-- Migration 000026 (down): disable RLS on user_credentials.
BEGIN;

DROP POLICY IF EXISTS user_credentials_admin_bypass ON user_credentials;
DROP POLICY IF EXISTS user_credentials_tenant_isolation_modify ON user_credentials;
DROP POLICY IF EXISTS user_credentials_tenant_isolation_select ON user_credentials;

ALTER TABLE user_credentials DISABLE ROW LEVEL SECURITY;

-- Note: we intentionally do NOT revoke GRANTs on rollback — the role needs
-- the access for the app to work even after RLS is disabled. If a stricter
-- rollback is needed, revoke manually:
--   REVOKE SELECT, INSERT, UPDATE, DELETE ON user_credentials FROM fmcg;
--   REVOKE SELECT, INSERT, UPDATE, DELETE ON user_credentials FROM app_admin;

COMMIT;