-- =============================================================================
-- 000001_init_fleet.sql
-- Sprint 1 + 4 — initial Fleet schema (mirror of DATABASE-SCHEMA.sql) plus
-- the outbox table introduced in Sprint 4 for durable retry of invoice calls.
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Kendaraan / Truk Armada
CREATE TABLE IF NOT EXISTS fleet_vehicles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    plate_number VARCHAR(20) NOT NULL UNIQUE,
    vehicle_type VARCHAR(50) NOT NULL,
    brand_model VARCHAR(100),
    capacity_kg NUMERIC(10, 2) NOT NULL DEFAULT 1000.00,
    status VARCHAR(20) NOT NULL DEFAULT 'AVAILABLE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Supir Ekspedisi
CREATE TABLE IF NOT EXISTS fleet_drivers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    phone_number VARCHAR(30) NOT NULL UNIQUE,
    license_number VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Trip Pengiriman (Surat Tugas Jalan)
CREATE TABLE IF NOT EXISTS fleet_trips (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    trip_number VARCHAR(50) NOT NULL UNIQUE,
    vehicle_id UUID NOT NULL REFERENCES fleet_vehicles(id),
    driver_id UUID NOT NULL REFERENCES fleet_drivers(id),
    departure_time TIMESTAMPTZ,
    completed_time TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'DRAFT',
    total_stops INT NOT NULL DEFAULT 0,
    total_delivered INT NOT NULL DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Surat Jalan / Delivery Order
CREATE TABLE IF NOT EXISTS fleet_delivery_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    trip_id UUID REFERENCES fleet_trips(id) ON DELETE SET NULL,
    do_number VARCHAR(50) NOT NULL UNIQUE,
    customer_id UUID NOT NULL,
    customer_name VARCHAR(150) NOT NULL,
    destination_address TEXT NOT NULL,
    destination_lat NUMERIC(10, 7),
    destination_lng NUMERIC(10, 7),
    total_items_ordered INT NOT NULL,
    total_items_delivered INT NOT NULL DEFAULT 0,
    total_items_rejected INT NOT NULL DEFAULT 0,
    nominal_ordered_cents BIGINT NOT NULL,
    nominal_delivered_cents BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(30) NOT NULL DEFAULT 'PENDING',
    fledger_invoice_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 5. Item Detail Pengiriman
CREATE TABLE IF NOT EXISTS fleet_do_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    do_id UUID NOT NULL REFERENCES fleet_delivery_orders(id) ON DELETE CASCADE,
    product_sku VARCHAR(50) NOT NULL,
    product_name VARCHAR(150) NOT NULL,
    qty_ordered INT NOT NULL,
    qty_delivered INT NOT NULL DEFAULT 0,
    qty_rejected INT NOT NULL DEFAULT 0,
    unit_price_cents BIGINT NOT NULL,
    rejection_reason VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 6. Proof of Delivery
CREATE TABLE IF NOT EXISTS fleet_proof_of_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    do_id UUID NOT NULL UNIQUE REFERENCES fleet_delivery_orders(id) ON DELETE CASCADE,
    recipient_name VARCHAR(100) NOT NULL,
    recipient_phone VARCHAR(30),
    signature_data_url TEXT NOT NULL,
    photo_evidence_urls TEXT[],
    delivered_lat NUMERIC(10, 7) NOT NULL,
    delivered_lng NUMERIC(10, 7) NOT NULL,
    delivered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    driver_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 7. Outbox (Sprint 4) — durable retry of POD → Fledger Core invoice sync.
CREATE TABLE IF NOT EXISTS fleet_outbox (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    subject VARCHAR(150) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_fleet_trips_status ON fleet_trips(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_fleet_do_trip ON fleet_delivery_orders(trip_id);
CREATE INDEX IF NOT EXISTS idx_fleet_do_customer ON fleet_delivery_orders(tenant_id, customer_id);
CREATE INDEX IF NOT EXISTS idx_fleet_do_status ON fleet_delivery_orders(status);
CREATE INDEX IF NOT EXISTS idx_fleet_outbox_due ON fleet_outbox(status, next_attempt_at)
    WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_fleet_outbox_aggregate ON fleet_outbox(aggregate_type, aggregate_id);

-- 8. Schema migrations bookkeeping (lightweight — used by `migrate status`).
CREATE TABLE IF NOT EXISTS fleet_schema_migrations (
    version INT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    description TEXT
);

INSERT INTO fleet_schema_migrations (version, description)
VALUES (1, 'initial fleet schema + outbox')
ON CONFLICT (version) DO NOTHING;