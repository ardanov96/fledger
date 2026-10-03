-- Migration 000028 (down): reverse tenant_rate_limit_tiers.
BEGIN;

DROP POLICY IF EXISTS tenant_rate_limit_tiers_admin_bypass ON tenant_rate_limit_tiers;
DROP POLICY IF EXISTS tenant_rate_limit_tiers_isolation_modify ON tenant_rate_limit_tiers;
DROP POLICY IF EXISTS tenant_rate_limit_tiers_isolation_select ON tenant_rate_limit_tiers;

DROP TABLE IF EXISTS tenant_rate_limit_tiers;

COMMIT;