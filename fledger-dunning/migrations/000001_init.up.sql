-- ==============================================================================
-- FLEDGER DUNNING — PostgreSQL 16 DDL Schema
-- Service: fledger-dunning (:8086)
-- Ecosystem: FLEDGER OS (Automated AR Dunning & WhatsApp Gateway)
-- Target DB: PostgreSQL 16
-- Currency Rule: All monetary amounts MUST use BIGINT minor units (Rupiah murni, 1 IDR = 1 unit).
-- Anti-Spam Rule: Rate limiting, jitter delay, and automated cancellation on payment.
-- ==============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ------------------------------------------------------------------------------
-- 1. Tabel dunning_configurations (Konfigurasi Dunning Per Tenant)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dunning_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    cadence_days JSONB NOT NULL DEFAULT '[-3, 0, 3, 7, 14]'::jsonb,
    wa_provider VARCHAR(50) NOT NULL DEFAULT 'MOCK', -- 'MOCK', 'BAILEYS', 'META_CLOUD'
    statement_day_of_month INT NOT NULL DEFAULT 1 CHECK (statement_day_of_month BETWEEN 1 AND 28),
    jitter_min_seconds INT NOT NULL DEFAULT 3,
    jitter_max_seconds INT NOT NULL DEFAULT 8,
    auto_cancel_on_payment BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_dunning_config_tenant UNIQUE (tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_dunning_config_tenant ON dunning_configurations(tenant_id);

-- ------------------------------------------------------------------------------
-- 2. Tabel dunning_store_contacts (Master Kontak WhatsApp & PIC Toko)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dunning_store_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    store_id VARCHAR(100) NOT NULL,
    store_name VARCHAR(255) NOT NULL,
    owner_name VARCHAR(255) NOT NULL,
    phone_number VARCHAR(30) NOT NULL, -- Format E.164 internasional (contoh: '6281234567890')
    email VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    opt_out BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_dunning_contact_tenant_store UNIQUE (tenant_id, store_id)
);

CREATE INDEX IF NOT EXISTS idx_dunning_contact_phone ON dunning_store_contacts(phone_number);
CREATE INDEX IF NOT EXISTS idx_dunning_contact_tenant_active ON dunning_store_contacts(tenant_id, is_active);

-- ------------------------------------------------------------------------------
-- 3. Tabel dunning_queues (Antrian Pesan Penagihan Faktur Bertingkat)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dunning_queues (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    invoice_id VARCHAR(100) NOT NULL,
    invoice_number VARCHAR(100) NOT NULL,
    store_id VARCHAR(100) NOT NULL,
    phone_number VARCHAR(30) NOT NULL,
    stage VARCHAR(50) NOT NULL, -- 'PRE_DUE_H3', 'DUE_DATE', 'OVERDUE_H3', 'OVERDUE_H7', 'OVERDUE_H14', 'MANUAL_REMINDER'
    due_date DATE NOT NULL,
    amount_due_minor BIGINT NOT NULL CHECK (amount_due_minor >= 0),
    payment_link_url TEXT NOT NULL,
    message_body TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'QUEUED', -- 'QUEUED', 'PROCESSING', 'SENT', 'FAILED', 'CANCELLED_BY_PAYMENT'
    scheduled_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    failure_reason TEXT,
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_dunning_queue_invoice_stage UNIQUE (tenant_id, invoice_id, stage)
);

CREATE INDEX IF NOT EXISTS idx_dunning_queue_scheduled ON dunning_queues(scheduled_at, status) WHERE status = 'QUEUED';
CREATE INDEX IF NOT EXISTS idx_dunning_queue_invoice ON dunning_queues(tenant_id, invoice_id);
CREATE INDEX IF NOT EXISTS idx_dunning_queue_store ON dunning_queues(tenant_id, store_id);

-- ------------------------------------------------------------------------------
-- 4. Tabel dunning_statements (Rekening Koran Toko Bulanan Format PDF)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dunning_statements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    store_id VARCHAR(100) NOT NULL,
    statement_month VARCHAR(7) NOT NULL, -- Format 'YYYY-MM', e.g. '2026-10'
    total_invoiced_minor BIGINT NOT NULL DEFAULT 0 CHECK (total_invoiced_minor >= 0),
    total_paid_minor BIGINT NOT NULL DEFAULT 0 CHECK (total_paid_minor >= 0),
    total_returned_minor BIGINT NOT NULL DEFAULT 0 CHECK (total_returned_minor >= 0),
    closing_balance_minor BIGINT NOT NULL DEFAULT 0 CHECK (closing_balance_minor >= 0),
    pdf_file_path TEXT NOT NULL,
    phone_number VARCHAR(30) NOT NULL,
    dispatch_status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'GENERATED', 'SENT', 'FAILED'
    sent_at TIMESTAMPTZ,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_dunning_statement_store_month UNIQUE (tenant_id, store_id, statement_month)
);

CREATE INDEX IF NOT EXISTS idx_dunning_statement_tenant_month ON dunning_statements(tenant_id, statement_month);
CREATE INDEX IF NOT EXISTS idx_dunning_statement_status ON dunning_statements(dispatch_status);

-- ------------------------------------------------------------------------------
-- 5. Tabel dunning_whatsapp_sessions (Koneksi & Session Pairing WhatsApp)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dunning_whatsapp_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    session_name VARCHAR(100) NOT NULL DEFAULT 'default',
    connection_status VARCHAR(50) NOT NULL DEFAULT 'DISCONNECTED', -- 'DISCONNECTED', 'SCAN_QR', 'CONNECTED', 'PAIRING_TIMEOUT'
    qr_code_data TEXT,
    phone_connected VARCHAR(30),
    last_heartbeat TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_dunning_session_tenant UNIQUE (tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_dunning_session_status ON dunning_whatsapp_sessions(connection_status);

-- ------------------------------------------------------------------------------
-- 6. Tabel dunning_message_logs (Log Riwayat Transmisi Pesan WhatsApp)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dunning_message_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    queue_id UUID REFERENCES dunning_queues(id) ON DELETE SET NULL,
    direction VARCHAR(10) NOT NULL DEFAULT 'OUTBOUND', -- 'OUTBOUND', 'INBOUND'
    phone_number VARCHAR(30) NOT NULL,
    message_type VARCHAR(30) NOT NULL DEFAULT 'TEXT', -- 'TEXT', 'DOCUMENT_PDF', 'PAYMENT_LINK'
    provider_message_id VARCHAR(100),
    status VARCHAR(50) NOT NULL DEFAULT 'SENT', -- 'SENT', 'DELIVERED', 'READ', 'FAILED'
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dunning_msg_logs_tenant ON dunning_message_logs(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_dunning_msg_logs_phone ON dunning_message_logs(phone_number);

-- ------------------------------------------------------------------------------
-- 7. Tabel dunning_audit_logs (Audit Trail Aksi Managerial)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dunning_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    actor_id VARCHAR(100) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_id VARCHAR(100) NOT NULL,
    details JSONB,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dunning_audit_tenant_action ON dunning_audit_logs(tenant_id, action, created_at DESC);

-- ------------------------------------------------------------------------------
-- Seeding Initial Demo Data (Tenant FMCG Mandiri Abadi)
-- ------------------------------------------------------------------------------
DO $$
DECLARE
    v_tenant_id UUID := 'a0000000-0000-0000-0000-000000000001'::uuid;
BEGIN
    -- 1. Insert Konfigurasi Default
    INSERT INTO dunning_configurations (
        tenant_id, cadence_days, wa_provider, statement_day_of_month, jitter_min_seconds, jitter_max_seconds, auto_cancel_on_payment
    ) VALUES (
        v_tenant_id, '[-3, 0, 3, 7, 14]'::jsonb, 'MOCK', 1, 3, 8, TRUE
    ) ON CONFLICT (tenant_id) DO NOTHING;

    -- 2. Insert Kontak Toko Retail
    INSERT INTO dunning_store_contacts (
        tenant_id, store_id, store_name, owner_name, phone_number, email
    ) VALUES 
        (v_tenant_id, 'CUST-001', 'Toko Berkah Kelontong', 'Haji Mahmud', '6281234567801', 'berkah@gmail.com'),
        (v_tenant_id, 'CUST-002', 'Minimarket Rezeki Lancar', 'Ibu Ratna', '6281234567802', 'rezeki.lancar@gmail.com'),
        (v_tenant_id, 'CUST-003', 'Warung Sumber Jaya', 'Pak Bowo', '6281234567803', 'sumberjaya@gmail.com')
    ON CONFLICT (tenant_id, store_id) DO NOTHING;

    -- 3. Insert Antrian Dunning Demo
    INSERT INTO dunning_queues (
        tenant_id, invoice_id, invoice_number, store_id, phone_number, stage, due_date,
        amount_due_minor, payment_link_url, message_body, status, scheduled_at
    ) VALUES 
        (v_tenant_id, 'INV-2026-001', 'INV/2026/10/001', 'CUST-001', '6281234567801', 'PRE_DUE_H3', CURRENT_DATE + INTERVAL '3 days',
         4500000, 'http://localhost:8083/pay/INV-2026-001', 'Halo Toko Berkah Kelontong, tagihan Faktur INV/2026/10/001 senilai Rp 4.500.000 akan jatuh tempo 3 hari lagi. Klik link untuk bayar via QRIS/VA: http://localhost:8083/pay/INV-2026-001',
         'QUEUED', NOW()),
        (v_tenant_id, 'INV-2026-002', 'INV/2026/10/002', 'CUST-002', '6281234567802', 'OVERDUE_H7', CURRENT_DATE - INTERVAL '7 days',
         12800000, 'http://localhost:8083/pay/INV-2026-002', 'PERINGATAN: Faktur INV/2026/10/002 telah terlambat 7 hari. Mohon segera lunasi Rp 12.800.000 untuk mencegah penguncian pesanan baru di http://localhost:8083/pay/INV-2026-002',
         'QUEUED', NOW() - INTERVAL '1 hour')
    ON CONFLICT (tenant_id, invoice_id, stage) DO NOTHING;

    -- 4. Inisialisasi Sesi WhatsApp Sandbox
    INSERT INTO dunning_whatsapp_sessions (
        tenant_id, session_name, connection_status, phone_connected, last_heartbeat
    ) VALUES (
        v_tenant_id, 'official-distributor-wa', 'CONNECTED', '6281198765432', NOW()
    ) ON CONFLICT (tenant_id) DO NOTHING;
END $$;
