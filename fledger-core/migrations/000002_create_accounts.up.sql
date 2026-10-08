-- =============================================================================
-- Migration: 000002_create_accounts
-- Description: Create accounts table (chart of accounts per tenant)
-- Author: FMCG Wallet
-- =============================================================================
-- Implements the account side of the double-entry ledger. Each account
-- belongs to a tenant (multi-tenancy via shared DB + tenant_id column).
-- cached_balance is a denormalization maintained atomically with entries.
-- =============================================================================

BEGIN;

CREATE TABLE accounts (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    code            TEXT            NOT NULL,
    name            TEXT            NOT NULL,
    type            TEXT            NOT NULL,
    status          TEXT            NOT NULL DEFAULT 'active',
    currency        TEXT            NOT NULL DEFAULT 'IDR',
    cached_balance  BIGINT          NOT NULL DEFAULT 0,    -- minor units (sen)
    owner_id        UUID,                                   -- polymorphic ref
    tenant_id       UUID            NOT NULL,
    metadata        JSONB           NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT accounts_type_check CHECK (
        type IN ('hq', 'outlet', 'sales_rep', 'customer',
                 'revenue', 'receivable', 'payable', 'cash', 'suspense')
    ),
    CONSTRAINT accounts_status_check CHECK (
        status IN ('active', 'frozen', 'closed')
    ),
    CONSTRAINT accounts_currency_check CHECK (length(currency) = 3),
    -- Cached balance cannot exceed sane bounds (prevent accidental corruption)
    CONSTRAINT accounts_cached_balance_range CHECK (
        cached_balance BETWEEN -100000000000000 AND 100000000000000
    )
);

-- Unique code per tenant
CREATE UNIQUE INDEX accounts_tenant_code_unique ON accounts(tenant_id, code);

-- Common lookups
CREATE INDEX accounts_tenant_id_idx     ON accounts(tenant_id);
CREATE INDEX accounts_tenant_type_idx   ON accounts(tenant_id, type);
CREATE INDEX accounts_tenant_status_idx ON accounts(tenant_id, status);
CREATE INDEX accounts_owner_id_idx      ON accounts(owner_id) WHERE owner_id IS NOT NULL;

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER accounts_set_updated_at
    BEFORE UPDATE ON accounts
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

COMMIT;
