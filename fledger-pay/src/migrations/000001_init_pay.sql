-- =============================================================================
-- 000001_init_pay.sql — Fledger Pay initial schema (mirror of DATABASE-SCHEMA.sql
-- from the brief, plus pay_schema_migrations bookkeeping table).
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. PAYMENT REQUESTS
CREATE TABLE IF NOT EXISTS pay_payment_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    fledger_invoice_id UUID NOT NULL,
    request_number VARCHAR(64) NOT NULL UNIQUE,
    customer_id UUID NOT NULL,
    customer_name VARCHAR(255) NOT NULL,
    customer_phone VARCHAR(32),
    currency VARCHAR(3) NOT NULL DEFAULT 'IDR',
    amount BIGINT NOT NULL CHECK (amount > 0),
    fee_amount BIGINT NOT NULL DEFAULT 0,
    total_amount BIGINT NOT NULL CHECK (total_amount >= amount),
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PARTIALLY_PAID', 'SETTLED', 'EXPIRED', 'CANCELLED')),
    expires_at TIMESTAMPTZ NOT NULL,
    settled_at TIMESTAMPTZ,
    settled_amount BIGINT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pay_req_tenant_invoice ON pay_payment_requests (tenant_id, fledger_invoice_id);
CREATE INDEX IF NOT EXISTS idx_pay_req_status ON pay_payment_requests (status);
CREATE INDEX IF NOT EXISTS idx_pay_req_expires_at ON pay_payment_requests (expires_at) WHERE status = 'PENDING';

-- 2. VIRTUAL ACCOUNTS
CREATE TABLE IF NOT EXISTS pay_virtual_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    payment_request_id UUID NOT NULL REFERENCES pay_payment_requests(id) ON DELETE CASCADE,
    bank_code VARCHAR(16) NOT NULL
        CHECK (bank_code IN ('BCA', 'MANDIRI', 'BRI', 'BNI', 'PERMATA', 'CIMB')),
    va_number VARCHAR(64) NOT NULL,
    va_name VARCHAR(255) NOT NULL,
    expected_amount BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'PAID', 'EXPIRED', 'INACTIVE')),
    expires_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_pay_va_bank_number UNIQUE (bank_code, va_number)
);

CREATE INDEX IF NOT EXISTS idx_pay_va_request ON pay_virtual_accounts (payment_request_id);
CREATE INDEX IF NOT EXISTS idx_pay_va_number ON pay_virtual_accounts (bank_code, va_number);

-- 3. QRIS CODES
CREATE TABLE IF NOT EXISTS pay_qris_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    payment_request_id UUID NOT NULL REFERENCES pay_payment_requests(id) ON DELETE CASCADE,
    qr_string TEXT NOT NULL,
    qr_image_url TEXT,
    expected_amount BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'PAID', 'EXPIRED', 'INACTIVE')),
    expires_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pay_qris_request ON pay_qris_codes (payment_request_id);

-- 4. PAYMENT TRANSACTIONS
CREATE TABLE IF NOT EXISTS pay_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    payment_request_id UUID NOT NULL REFERENCES pay_payment_requests(id),
    channel VARCHAR(32) NOT NULL
        CHECK (channel IN ('VA_BCA', 'VA_MANDIRI', 'VA_BRI', 'VA_BNI', 'VA_PERMATA', 'QRIS', 'MANUAL_TRANSFER')),
    external_reference VARCHAR(128) NOT NULL UNIQUE,
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    gross_amount BIGINT NOT NULL,
    net_amount BIGINT NOT NULL,
    fee_amount BIGINT NOT NULL DEFAULT 0,
    paid_at TIMESTAMPTZ NOT NULL,
    payer_name VARCHAR(255),
    payer_bank VARCHAR(32),
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    signature_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pay_tx_request ON pay_transactions (payment_request_id);
CREATE INDEX IF NOT EXISTS idx_pay_tx_ext_ref ON pay_transactions (external_reference);

-- 5. OUTBOX SETTLEMENTS
CREATE TABLE IF NOT EXISTS pay_settlement_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    transaction_id UUID NOT NULL REFERENCES pay_transactions(id),
    fledger_invoice_id UUID NOT NULL,
    event_type VARCHAR(64) NOT NULL DEFAULT 'fledger.pay.settled',
    payload JSONB NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'SENT', 'FAILED')),
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 10,
    last_error TEXT,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pay_outbox_queue ON pay_settlement_outbox (status, next_retry_at)
    WHERE status IN ('PENDING', 'FAILED');

-- 6. AUDIT LOGS
CREATE TABLE IF NOT EXISTS pay_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    actor_id VARCHAR(128) NOT NULL,
    actor_role VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id UUID NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pay_audit_resource ON pay_audit_logs (resource_type, resource_id);

-- 7. Schema migrations bookkeeping
CREATE TABLE IF NOT EXISTS pay_schema_migrations (
    version INT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    description TEXT
);

INSERT INTO pay_schema_migrations (version, description)
VALUES (1, 'init pay schema (payment_requests, virtual_accounts, qris, transactions, outbox, audit)')
ON CONFLICT (version) DO NOTHING;