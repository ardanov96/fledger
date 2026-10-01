-- =============================================================================
-- Migration: 000024_fraud_flags
-- Description: Fraud flag table for rule-based fraud detection (Sprint 31 / Fase 8)
-- Author: FMCG Wallet
-- =============================================================================
-- Fraud flags are created by the FraudScannerWorker when it consumes
-- transfer.posted events from NATS (Sprint 24) and one or more rules
-- trigger (large amount, off-hours, velocity, first-time recipient).
--
-- Schema:
--   id            UUID PK
--   tenant_id     UUID ( RLS — tenant isolation)
--   account_id    UUID (account that triggered the rule)
--   transfer_id   UUID (transfer that triggered the flag)
--   rule_name     TEXT ('large_amount' | 'off_hours' | 'velocity' | 'first_time_recipient')
--   severity      TEXT ('info' | 'warn' | 'critical')
--   status        TEXT ('open' | 'dismissed' | 'confirmed')
--   evidence      JSONB (rule-specific: amount, hour, count, etc.)
--   detected_at   TIMESTAMPTZ
--   resolved_at   TIMESTAMPTZ (set when status transitions away from 'open')
--
-- Indexes:
--   - (tenant_id, account_id, detected_at DESC) - hot path for "flags for this account"
--   - (status) where status = 'open' - "open flag count" badge per tenant
--   - (tenant_id, rule_name, detected_at DESC) - admin views "flags by rule"
--
-- Sprint 30 follow-up: this table is created AFTER migration 000023 which
-- issued ALTER DEFAULT PRIVILEGES for app_admin. New tables created AFTER
-- 000023 automatically receive GRANTs to app_admin, so this migration does
-- NOT need explicit GRANT statements for app_admin (unlike 000016/000017/000018
-- which were created before 000023).
-- =============================================================================

BEGIN;

CREATE TABLE fraud_flags (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID         NOT NULL,
    account_id    UUID         NOT NULL,
    transfer_id   UUID         NOT NULL,
    rule_name     TEXT         NOT NULL,
    severity      TEXT         NOT NULL,
    status        TEXT         NOT NULL DEFAULT 'open',
    evidence      JSONB        NOT NULL DEFAULT '{}'::jsonb,
    detected_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    resolved_at   TIMESTAMPTZ,

    CONSTRAINT fraud_flags_rule_name_valid CHECK (
        rule_name IN ('large_amount', 'off_hours', 'velocity', 'first_time_recipient')
    ),
    CONSTRAINT fraud_flags_severity_valid CHECK (
        severity IN ('info', 'warn', 'critical')
    ),
    CONSTRAINT fraud_flags_status_valid CHECK (
        status IN ('open', 'dismissed', 'confirmed')
    )
);

-- Grant basic table/sequence permissions to fmcg BEFORE enabling RLS.
-- (app_admin gets GRANTs automatically via ALTER DEFAULT PRIVILEGES from
-- migration 000023.)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fmcg') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON fraud_flags TO fmcg;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO fmcg;
    END IF;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

-- Hot-path: list flags for one account
CREATE INDEX fraud_flags_account_detected_idx
    ON fraud_flags (tenant_id, account_id, detected_at DESC);

-- Hot-path: count of open flags per tenant (admin overview)
CREATE INDEX fraud_flags_open_idx
    ON fraud_flags (tenant_id, status)
    WHERE status = 'open';

-- Admin view: flags by rule
CREATE INDEX fraud_flags_rule_idx
    ON fraud_flags (tenant_id, rule_name, detected_at DESC);

-- RLS + app_admin bypass (Sprint 22A.4 pattern)
ALTER TABLE fraud_flags ENABLE ROW LEVEL SECURITY;
ALTER TABLE fraud_flags FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS fraud_flags_tenant_isolation_select ON fraud_flags;
CREATE POLICY fraud_flags_tenant_isolation_select ON fraud_flags
    FOR SELECT USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    );

DROP POLICY IF EXISTS fraud_flags_tenant_isolation_modify ON fraud_flags;
CREATE POLICY fraud_flags_tenant_isolation_modify ON fraud_flags
    FOR ALL USING (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    )
    WITH CHECK (
        tenant_id = current_setting('app.current_tenant_id', true)::uuid
        OR current_user = 'app_admin'
    );

COMMENT ON TABLE fraud_flags IS
    'Sprint 31 / Fase 8: rule-based fraud detection. Created by FraudScannerWorker on NATS '
    'transfer.posted events. End users see critical flags via the notification feed (Sprint 28).';

COMMIT;

-- =============================================================================
-- DOWN (rollback)
-- =============================================================================
-- BEGIN;
-- DROP POLICY IF EXISTS fraud_flags_tenant_isolation_modify ON fraud_flags;
-- DROP POLICY IF EXISTS fraud_flags_tenant_isolation_select ON fraud_flags;
-- DROP TABLE IF EXISTS fraud_flags;
-- COMMIT;