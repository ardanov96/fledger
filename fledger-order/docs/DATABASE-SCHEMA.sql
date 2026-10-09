-- =============================================================================
-- FLEDGER ORDER — PostgreSQL 16 DDL Schema Definition
-- Microservice: B2B Order Management System (OMS) & Hard Credit Gate
-- Port: :8085 | Multi-Tenant RLS Ready | BigInt Minor Units for Currency
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- -----------------------------------------------------------------------------
-- 1. PRODUCTS / FMCG MASTER SKU CATALOG
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS order_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    sku VARCHAR(64) NOT NULL,                        -- Contoh: SKU-OIL-BIMOLI-2L
    barcode VARCHAR(64),
    name VARCHAR(255) NOT NULL,                      -- Contoh: Minyak Goreng Bimoli 2L (Dus)
    category VARCHAR(64) NOT NULL DEFAULT 'SEMBAKO', -- 'SEMBAKO', 'MIE_INSTAN', 'MINUMAN', 'TOILETRIES', 'ROKOK'
    unit VARCHAR(32) NOT NULL DEFAULT 'DUS',         -- 'DUS', 'KARTON', 'PACK', 'RENCENG', 'PCS'
    weight_grams INT NOT NULL DEFAULT 1000,          -- Berat per satuan dalam gram (untuk beban truk Fleet)
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_order_product_sku UNIQUE (tenant_id, sku)
);

CREATE INDEX IF NOT EXISTS idx_order_products_tenant ON order_products (tenant_id);
CREATE INDEX IF NOT EXISTS idx_order_products_category ON order_products (category);

-- -----------------------------------------------------------------------------
-- 2. MULTI-TIER PRICING (Penetapan Harga Grosir Bertingkat)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS order_price_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    product_id UUID NOT NULL REFERENCES order_products(id) ON DELETE CASCADE,
    tier VARCHAR(32) NOT NULL                        -- 'GROSIR', 'SEMI_GROSIR', 'RETAIL', 'STAR_OUTLET'
        CHECK (tier IN ('GROSIR', 'SEMI_GROSIR', 'RETAIL', 'STAR_OUTLET')),
    min_quantity INT NOT NULL DEFAULT 1,             -- Minimum pembelian untuk mendapatkan harga ini
    unit_price BIGINT NOT NULL CHECK (unit_price > 0),-- Harga jual dalam rupiah bulat (Rp)
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_order_price_tier UNIQUE (tenant_id, product_id, tier, min_quantity)
);

CREATE INDEX IF NOT EXISTS idx_order_pricing_prod_tier ON order_price_tiers (product_id, tier);

-- -----------------------------------------------------------------------------
-- 3. INVENTORY STOCKS & RESERVATION (Alokasi Stok Gudang Distributor)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS order_inventory_stocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    product_id UUID NOT NULL REFERENCES order_products(id) ON DELETE CASCADE,
    warehouse_id VARCHAR(64) NOT NULL DEFAULT 'WH-CENTRAL-01',
    on_hand_qty INT NOT NULL DEFAULT 0 CHECK (on_hand_qty >= 0),       -- Stok fisik di gudang
    reserved_qty INT NOT NULL DEFAULT 0 CHECK (reserved_qty >= 0),     -- Stok terkunci oleh order berjalan
    available_qty INT GENERATED ALWAYS AS (on_hand_qty - reserved_qty) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_order_stock_prod_wh UNIQUE (tenant_id, product_id, warehouse_id)
);

CREATE INDEX IF NOT EXISTS idx_order_stocks_avail ON order_inventory_stocks (product_id, warehouse_id);

-- -----------------------------------------------------------------------------
-- 4. B2B PURCHASE ORDERS (Pesanan Pembelian Toko)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS order_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    order_number VARCHAR(64) NOT NULL UNIQUE,        -- Contoh: ORD-202610-00812
    customer_id UUID NOT NULL,                       -- ID Toko di Fledger Core
    customer_name VARCHAR(255) NOT NULL,
    customer_tier VARCHAR(32) NOT NULL DEFAULT 'RETAIL',
    destination_address TEXT NOT NULL,
    customer_phone VARCHAR(32),
    total_weight_kg INT NOT NULL DEFAULT 0,          -- Akumulasi berat untuk kapasitas muat Fleet
    subtotal BIGINT NOT NULL CHECK (subtotal >= 0),
    discount BIGINT NOT NULL DEFAULT 0,
    total_amount BIGINT NOT NULL CHECK (total_amount > 0),
    status VARCHAR(32) NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'PENDING_CREDIT_CHECK', 'CREDIT_BLOCKED', 'APPROVED', 'DISPATCHED_TO_FLEET', 'COMPLETED', 'CANCELLED')),
    
    -- Hasil Evaluasi Hard Credit Gate
    credit_gate_status VARCHAR(32) DEFAULT 'UNEVALUATED'
        CHECK (credit_gate_status IN ('UNEVALUATED', 'PASSED', 'LIMIT_EXCEEDED', 'OVERDUE_BLOCKED', 'OVERRIDDEN')),
    credit_check_details JSONB NOT NULL DEFAULT '{}'::jsonb,
    override_by VARCHAR(128),                        -- User ID Finance Manager jika ada emergency override
    override_reason TEXT,
    
    -- Bridge ke Fledger Fleet
    fledger_fleet_do_id UUID,                        -- Diisi setelah berhasil create DO di fledger-fleet
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_order_orders_tenant ON order_orders (tenant_id);
CREATE INDEX IF NOT EXISTS idx_order_orders_cust ON order_orders (customer_id);
CREATE INDEX IF NOT EXISTS idx_order_orders_status ON order_orders (status);

-- -----------------------------------------------------------------------------
-- 5. ORDER LINE ITEMS (Rincian Produk Pesanan)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    order_id UUID NOT NULL REFERENCES order_orders(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES order_products(id),
    sku VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    unit_price BIGINT NOT NULL CHECK (unit_price > 0),
    line_total BIGINT NOT NULL CHECK (line_total > 0),
    weight_grams INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_order_items_order ON order_items (order_id);

-- -----------------------------------------------------------------------------
-- 6. OUTBOX DISPATCH EVENTS (Integrasi Transaksional ke Fledger Fleet)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS order_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    event_type VARCHAR(64) NOT NULL DEFAULT 'order.dispatched_fleet',
    aggregate_id UUID NOT NULL,                      -- order_id
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

CREATE INDEX IF NOT EXISTS idx_order_outbox_queue ON order_outbox (status, next_retry_at)
    WHERE status IN ('PENDING', 'FAILED');

-- -----------------------------------------------------------------------------
-- 7. AUDIT TRAIL LOGS
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS order_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    actor_id VARCHAR(128) NOT NULL,
    actor_role VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,                      -- 'CREATE_ORDER', 'CREDIT_PASSED', 'CREDIT_BLOCKED', 'OVERRIDE_APPROVED'
    resource_type VARCHAR(64) NOT NULL,
    resource_id UUID NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_order_audit_resource ON order_audit_logs (resource_type, resource_id);
