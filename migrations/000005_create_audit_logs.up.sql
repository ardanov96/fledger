-- =============================================================================
-- Migration: 000005_create_audit_logs
-- Description: Create audit_logs table for compliance & debugging
-- Author: FMCG Wallet
-- =============================================================================
-- Audit logs are append-only records of significant user actions:
--   - transfer created
--   - account created / frozen / closed
--   - login / logout
--   - admin actions
--
-- Retention: 7 years (configurable) for financial compliance.
-- Immutability enforced by REVOKE on UPDATE/DELETE (see below).
-- =============================================================================

BEGIN;

CREATE TABLE audit_logs (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID            NOT NULL,
    actor_id        UUID,                                   -- user/service that did the action
    actor_type      TEXT            NOT NULL DEFAULT 'user',  -- 'user' | 'service' | 'system'
    action          TEXT            NOT NULL,                -- e.g. 'transfer.create', 'account.freeze'
    resource_type   TEXT,                                    -- e.g. 'transfer', 'account'
    resource_id     TEXT,                                    -- id of the resource (string for flexibility)
    outcome         TEXT            NOT NULL DEFAULT 'success',-- 'success' | 'failure'
    request_id      TEXT,                                    -- correlation id from request
    ip_address      INET,
    user_agent      TEXT,
    metadata        JSONB           NOT NULL DEFAULT '{}',
    occurred_at     TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT audit_logs_action_check CHECK (
        action ~ '^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$'
    ),
    CONSTRAINT audit_logs_actor_type_check CHECK (
        actor_type IN ('user', 'service', 'system', 'admin')
    ),
    CONSTRAINT audit_logs_outcome_check CHECK (
        outcome IN ('success', 'failure')
    )
);

-- Common query patterns
CREATE INDEX audit_logs_tenant_occurred_idx
    ON audit_logs(tenant_id, occurred_at DESC);
CREATE INDEX audit_logs_actor_idx
    ON audit_logs(actor_id, occurred_at DESC)
    WHERE actor_id IS NOT NULL;
CREATE INDEX audit_logs_resource_idx
    ON audit_logs(resource_type, resource_id, occurred_at DESC)
    WHERE resource_type IS NOT NULL;
CREATE INDEX audit_logs_action_idx
    ON audit_logs(action, occurred_at DESC);

-- GIN index for metadata queries
CREATE INDEX audit_logs_metadata_gin
    ON audit_logs USING gin (metadata);

-- Immutability: revoke UPDATE/DELETE at the table level.
-- (Application still has INSERT; only DBAs can modify history.)
-- NOTE: requires superuser to REVOKE; handled at deployment time.
-- See /docs/runbooks/audit-immutability.md

-- Retention: partition by month for efficient pruning
-- (Fase 1B; commented for now to keep migration simple)
-- ALTER TABLE audit_logs ... PARTITION BY RANGE (occurred_at);

COMMIT;

-- =============================================================================
-- DOWN
-- =============================================================================
-- DROP TABLE IF EXISTS audit_logs;
