-- =============================================================================
-- FLEDGER FLEET — PostgreSQL 16 Database Schema
-- Part of FLEDGER OS Ecosystem
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Tabel Kendaraan / Truk Armada
CREATE TABLE IF NOT EXISTS fleet_vehicles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    plate_number VARCHAR(20) NOT NULL UNIQUE,
    vehicle_type VARCHAR(50) NOT NULL, -- 'CDE_BOX', 'CDD_BOX', 'BLIND_VAN', 'MOTOR_CARGO'
    brand_model VARCHAR(100),
    capacity_kg NUMERIC(10, 2) NOT NULL DEFAULT 1000.00,
    status VARCHAR(20) NOT NULL DEFAULT 'AVAILABLE', -- 'AVAILABLE', 'ON_TRIP', 'MAINTENANCE'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Tabel Supir Ekspedisi
CREATE TABLE IF NOT EXISTS fleet_drivers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    phone_number VARCHAR(30) NOT NULL UNIQUE,
    license_number VARCHAR(50) NOT NULL, -- Nomor SIM B1 / SIM A
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE', -- 'ACTIVE', 'ON_DUTY', 'INACTIVE'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Tabel Trip Pengiriman (Surat Tugas Jalan Armada)
CREATE TABLE IF NOT EXISTS fleet_trips (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    trip_number VARCHAR(50) NOT NULL UNIQUE, -- e.g., 'TRIP-202610-001'
    vehicle_id UUID NOT NULL REFERENCES fleet_vehicles(id),
    driver_id UUID NOT NULL REFERENCES fleet_drivers(id),
    departure_time TIMESTAMPTZ,
    completed_time TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'DRAFT', -- 'DRAFT', 'DISPATCHED', 'IN_TRANSIT', 'COMPLETED', 'CANCELLED'
    total_stops INT NOT NULL DEFAULT 0,
    total_delivered INT NOT NULL DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Tabel Surat Jalan / Delivery Order (DO)
CREATE TABLE IF NOT EXISTS fleet_delivery_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    trip_id UUID REFERENCES fleet_trips(id) ON DELETE SET NULL,
    do_number VARCHAR(50) NOT NULL UNIQUE, -- e.g., 'DO-202610-0089'
    customer_id UUID NOT NULL, -- Foreign ID Toko (terdaftar di Fledger Core)
    customer_name VARCHAR(150) NOT NULL,
    destination_address TEXT NOT NULL,
    destination_lat NUMERIC(10, 7),
    destination_lng NUMERIC(10, 7),
    total_items_ordered INT NOT NULL,
    total_items_delivered INT NOT NULL DEFAULT 0,
    total_items_rejected INT NOT NULL DEFAULT 0,
    nominal_ordered_cents BIGINT NOT NULL, -- Nilai total pesanan kotor (dalam Rupiah)
    nominal_delivered_cents BIGINT NOT NULL DEFAULT 0, -- Nilai bersih barang yang diterima
    status VARCHAR(30) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'LOADED', 'OUT_FOR_DELIVERY', 'DELIVERED_FULL', 'DELIVERED_PARTIAL', 'DELIVERY_FAILED'
    fledger_invoice_id UUID, -- UUID invoice resmi di Fledger Core
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 5. Tabel Item Detail Pengiriman
CREATE TABLE IF NOT EXISTS fleet_do_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    do_id UUID NOT NULL REFERENCES fleet_delivery_orders(id) ON DELETE CASCADE,
    product_sku VARCHAR(50) NOT NULL,
    product_name VARCHAR(150) NOT NULL,
    qty_ordered INT NOT NULL,
    qty_delivered INT NOT NULL DEFAULT 0,
    qty_rejected INT NOT NULL DEFAULT 0,
    unit_price_cents BIGINT NOT NULL, -- Harga satuan barang
    rejection_reason VARCHAR(100), -- 'DAMAGED_LEAK', 'EXPIRED', 'WRONG_ITEM', 'STORE_REJECTED'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 6. Tabel Proof of Delivery (POD)
CREATE TABLE IF NOT EXISTS fleet_proof_of_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    do_id UUID NOT NULL UNIQUE REFERENCES fleet_delivery_orders(id) ON DELETE CASCADE,
    recipient_name VARCHAR(100) NOT NULL,
    recipient_phone VARCHAR(30),
    signature_data_url TEXT NOT NULL, -- Base64 data URL tanda tangan canvas
    photo_evidence_urls TEXT[], -- Array URL foto bukti barang & serah terima
    delivered_lat NUMERIC(10, 7) NOT NULL,
    delivered_lng NUMERIC(10, 7) NOT NULL,
    delivered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    driver_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes untuk optimasi query
CREATE INDEX IF NOT EXISTS idx_fleet_trips_status ON fleet_trips(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_fleet_do_trip ON fleet_delivery_orders(trip_id);
CREATE INDEX IF NOT EXISTS idx_fleet_do_customer ON fleet_delivery_orders(tenant_id, customer_id);
CREATE INDEX IF NOT EXISTS idx_fleet_do_status ON fleet_delivery_orders(status);
