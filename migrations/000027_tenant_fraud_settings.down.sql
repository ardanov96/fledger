-- Migration 000027 (down): reverse tenant_fraud_settings.
BEGIN;

DROP POLICY IF EXISTS tenant_fraud_settings_admin_bypass ON tenant_fraud_settings;
DROP POLICY IF EXISTS tenant_fraud_settings_isolation_modify ON tenant_fraud_settings;
DROP POLICY IF EXISTS tenant_fraud_settings_isolation_select ON tenant_fraud_settings;

DROP TABLE IF EXISTS tenant_fraud_settings;

COMMIT;