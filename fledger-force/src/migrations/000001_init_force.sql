-- =============================================================================
-- 000001_init_force.sql
-- Fledger Force initial schema (mirror of DATABASE-SCHEMA.sql from the brief,
-- plus the force_schema_migrations bookkeeping table).
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. SALES REPS
CREATE TABLE IF NOT EXISTS force_sales_reps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    employee_code VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    phone VARCHAR(32) NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'CANVASSER'
        CHECK (role IN ('CANVASSER', 'MOTORIS', 'SUPERVISOR', 'COLLECTOR')),
    fledger_wallet_account_id VARCHAR(128) NOT NULL,
    current_cash_held BIGINT NOT NULL DEFAULT 0 CHECK (current_cash_held >= 0),
    max_cash_limit BIGINT NOT NULL DEFAULT 50000000,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'SETTLEMENT_LOCKED', 'SUSPENDED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_force_sales_rep_emp UNIQUE (tenant_id, employee_code)
);

CREATE INDEX IF NOT EXISTS idx_force_sales_rep_tenant ON force_sales_reps (tenant_id);
CREATE INDEX IF NOT EXISTS idx_force_sales_rep_status ON force_sales_reps (status);

-- 2. STORES
CREATE TABLE IF NOT EXISTS force_stores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    store_code VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    owner_name VARCHAR(255),
    phone VARCHAR(32),
    address TEXT NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    geofence_radius_meters INT NOT NULL DEFAULT 100,
    tier VARCHAR(16) NOT NULL DEFAULT 'RETAIL'
        CHECK (tier IN ('GROSIR', 'SEMI_GROSIR', 'RETAIL', 'STAR_OUTLET')),
    credit_limit BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_force_store_code UNIQUE (tenant_id, store_code)
);

CREATE INDEX IF NOT EXISTS idx_force_stores_tenant ON force_stores (tenant_id);

-- 3. BEAT PLANS
CREATE TABLE IF NOT EXISTS force_beat_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    plan_number VARCHAR(64) NOT NULL UNIQUE,
    sales_rep_id UUID NOT NULL REFERENCES force_sales_reps(id),
    plan_date DATE NOT NULL,
    territory VARCHAR(128) NOT NULL,
    target_stores_count INT NOT NULL DEFAULT 0,
    visited_stores_count INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'ASSIGNED'
        CHECK (status IN ('ASSIGNED', 'IN_PROGRESS', 'COMPLETED', 'CANCELLED')),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_force_beat_rep_date ON force_beat_plans (sales_rep_id, plan_date);

-- 4. STORE VISITS
CREATE TABLE IF NOT EXISTS force_visits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    beat_plan_id UUID NOT NULL REFERENCES force_beat_plans(id),
    store_id UUID NOT NULL REFERENCES force_stores(id),
    sales_rep_id UUID NOT NULL REFERENCES force_sales_reps(id),
    check_in_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    check_out_at TIMESTAMPTZ,
    check_in_lat DOUBLE PRECISION NOT NULL,
    check_in_long DOUBLE PRECISION NOT NULL,
    distance_meters INT NOT NULL,
    geofence_verified BOOLEAN NOT NULL DEFAULT FALSE,
    visit_type VARCHAR(32) NOT NULL DEFAULT 'TAKING_ORDER_AND_COLLECTION'
        CHECK (visit_type IN ('TAKING_ORDER', 'CASH_COLLECTION', 'TAKING_ORDER_AND_COLLECTION', 'CANVASSING', 'NO_ORDER')),
    status VARCHAR(32) NOT NULL DEFAULT 'CHECKED_IN'
        CHECK (status IN ('CHECKED_IN', 'COMPLETED', 'SKIPPED')),
    skip_reason VARCHAR(255),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_force_visits_beat ON force_visits (beat_plan_id);
CREATE INDEX IF NOT EXISTS idx_force_visits_store ON force_visits (store_id);

-- 5. CASH COLLECTIONS
CREATE TABLE IF NOT EXISTS force_cash_collections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    visit_id UUID REFERENCES force_visits(id),
    sales_rep_id UUID NOT NULL REFERENCES force_sales_reps(id),
    store_id UUID NOT NULL REFERENCES force_stores(id),
    fledger_invoice_id UUID NOT NULL,
    receipt_number VARCHAR(64) NOT NULL UNIQUE,
    amount BIGINT NOT NULL CHECK (amount > 0),
    collected_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    payer_name VARCHAR(255) NOT NULL,
    payer_phone VARCHAR(32),
    wa_receipt_sent BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(32) NOT NULL DEFAULT 'HELD_BY_SALES'
        CHECK (status IN ('HELD_BY_SALES', 'SETTLED_TO_HQ', 'CANCELLED')),
    settlement_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_force_col_rep_status ON force_cash_collections (sales_rep_id, status);
CREATE INDEX IF NOT EXISTS idx_force_col_invoice ON force_cash_collections (fledger_invoice_id);

-- 6. EOD SETTLEMENTS
CREATE TABLE IF NOT EXISTS force_eod_settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    settlement_number VARCHAR(64) NOT NULL UNIQUE,
    sales_rep_id UUID NOT NULL REFERENCES force_sales_reps(id),
    cashier_user_id VARCHAR(128) NOT NULL,
    total_system_cash BIGINT NOT NULL,
    total_physical_cash BIGINT NOT NULL,
    discrepancy_amount BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'SETTLED'
        CHECK (status IN ('SETTLED', 'DISCREPANCY_FLAGGED', 'REJECTED')),
    cashier_notes TEXT,
    settled_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_force_eod_rep ON force_eod_settlements (sales_rep_id);

-- 7. SETTLEMENT OUTBOX
CREATE TABLE IF NOT EXISTS force_settlement_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
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

CREATE INDEX IF NOT EXISTS idx_force_outbox_queue ON force_settlement_outbox (status, next_retry_at)
    WHERE status IN ('PENDING', 'FAILED');

-- 8. AUDIT LOGS
CREATE TABLE IF NOT EXISTS force_audit_logs (
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

CREATE INDEX IF NOT EXISTS idx_force_audit_resource ON force_audit_logs (resource_type, resource_id);

-- 9. SCHEMA MIGRATIONS
CREATE TABLE IF NOT EXISTS force_schema_migrations (
    version INT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    description TEXT
);

INSERT INTO force_schema_migrations (version, description)
VALUES (1, 'init force schema (reps, stores, beat_plans, visits, collections, eod_settlements, outbox, audit)')
ON CONFLICT (version) DO NOTHING;