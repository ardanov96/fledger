-- ==============================================================================
-- FLEDGER OS - Core Dummy Seeder for Pagination & High-Volume Data Testing
-- Seeds ~15-25 realistic dummy records for:
-- 1. Accounts
-- 2. Accounting Periods
-- 3. Transactions & Ledger Entries
-- 4. Invoices (with diverse aging buckets)
-- 5. Reconciler Runs
-- 6. Currencies & FX Rates
-- 7. Audit Logs
-- ==============================================================================

BEGIN;

-- Set session tenant to 00000000-0000-0000-0000-000000000001 (Admin HQ)
SET LOCAL app.current_tenant_id = '00000000-0000-0000-0000-000000000001';

-- =============================================================================
-- 1. CURRENCIES (Add Global & ASEAN Currencies)
-- =============================================================================
INSERT INTO currencies (code, decimal_places, name, is_active, created_at)
VALUES
    ('EUR', 2, 'Euro', true, NOW()),
    ('JPY', 0, 'Japanese Yen', true, NOW()),
    ('GBP', 2, 'British Pound', true, NOW()),
    ('AUD', 2, 'Australian Dollar', true, NOW()),
    ('CNY', 2, 'Chinese Yuan', true, NOW()),
    ('MYR', 2, 'Malaysian Ringgit', true, NOW()),
    ('THB', 2, 'Thai Baht', true, NOW()),
    ('PHP', 2, 'Philippine Peso', true, NOW()),
    ('HKD', 2, 'Hong Kong Dollar', true, NOW())
ON CONFLICT (code) DO UPDATE SET is_active = true, name = EXCLUDED.name;

-- FX Rates against IDR
INSERT INTO fx_rates (id, tenant_id, from_currency, to_currency, rate, effective_at, expires_at, source, created_by, created_at)
VALUES
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'EUR', 'IDR', 17250.0000000000, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 'manual', '33333333-3333-3333-3333-333333333333', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'JPY', 'IDR', 105.5000000000, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 'manual', '33333333-3333-3333-3333-333333333333', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'GBP', 'IDR', 20150.0000000000, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 'manual', '33333333-3333-3333-3333-333333333333', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'AUD', 'IDR', 10450.0000000000, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 'manual', '33333333-3333-3333-3333-333333333333', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'CNY', 'IDR', 2180.0000000000, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 'manual', '33333333-3333-3333-3333-333333333333', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'MYR', 'IDR', 3550.0000000000, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 'manual', '33333333-3333-3333-3333-333333333333', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'THB', 'IDR', 460.0000000000, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 'manual', '33333333-3333-3333-3333-333333333333', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'PHP', 'IDR', 275.0000000000, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 'manual', '33333333-3333-3333-3333-333333333333', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'HKD', 'IDR', 2020.0000000000, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', 'manual', '33333333-3333-3333-3333-333333333333', NOW())
ON CONFLICT DO NOTHING;

-- =============================================================================
-- 2. ACCOUNTING PERIODS (Historical Monthly Periods)
-- =============================================================================
INSERT INTO accounting_periods (id, tenant_id, period_start, period_end, status, closed_at, closed_by, created_at, updated_at)
VALUES
    ('77777777-2025-0001-0000-000000000001', '00000000-0000-0000-0000-000000000001', '2025-01-01', '2025-01-31', 'closed', '2025-02-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0002-0000-000000000002', '00000000-0000-0000-0000-000000000001', '2025-02-01', '2025-02-28', 'closed', '2025-03-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0003-0000-000000000003', '00000000-0000-0000-0000-000000000001', '2025-03-01', '2025-03-31', 'closed', '2025-04-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0004-0000-000000000004', '00000000-0000-0000-0000-000000000001', '2025-04-01', '2025-04-30', 'closed', '2025-05-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0005-0000-000000000005', '00000000-0000-0000-0000-000000000001', '2025-05-01', '2025-05-31', 'closed', '2025-06-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0006-0000-000000000006', '00000000-0000-0000-0000-000000000001', '2025-06-01', '2025-06-30', 'closed', '2025-07-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0007-0000-000000000007', '00000000-0000-0000-0000-000000000001', '2025-07-01', '2025-07-31', 'closed', '2025-08-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0008-0000-000000000008', '00000000-0000-0000-0000-000000000001', '2025-08-01', '2025-08-31', 'closed', '2025-09-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0009-0000-000000000009', '00000000-0000-0000-0000-000000000001', '2025-09-01', '2025-09-30', 'closed', '2025-10-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0010-0000-000000000010', '00000000-0000-0000-0000-000000000001', '2025-10-01', '2025-10-31', 'closed', '2025-11-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0011-0000-000000000011', '00000000-0000-0000-0000-000000000001', '2025-11-01', '2025-11-30', 'closed', '2025-12-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2025-0012-0000-000000000012', '00000000-0000-0000-0000-000000000001', '2025-12-01', '2025-12-31', 'closed', '2026-01-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2026-0001-0000-000000000001', '00000000-0000-0000-0000-000000000001', '2026-01-01', '2026-01-31', 'closed', '2026-02-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2026-0002-0000-000000000002', '00000000-0000-0000-0000-000000000001', '2026-02-01', '2026-02-28', 'closed', '2026-03-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2026-0003-0000-000000000003', '00000000-0000-0000-0000-000000000001', '2026-03-01', '2026-03-31', 'closed', '2026-04-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2026-0004-0000-000000000004', '00000000-0000-0000-0000-000000000001', '2026-04-01', '2026-04-30', 'closed', '2026-05-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2026-0005-0000-000000000005', '00000000-0000-0000-0000-000000000001', '2026-05-01', '2026-05-31', 'closed', '2026-06-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2026-0006-0000-000000000006', '00000000-0000-0000-0000-000000000001', '2026-06-01', '2026-06-30', 'closed', '2026-07-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2026-0007-0000-000000000007', '00000000-0000-0000-0000-000000000001', '2026-07-01', '2026-07-31', 'closed', '2026-08-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW()),
    ('77777777-2026-0008-0000-000000000008', '00000000-0000-0000-0000-000000000001', '2026-08-01', '2026-08-31', 'closed', '2026-09-01 10:00:00+08', '33333333-3333-3333-3333-333333333333', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- =============================================================================
-- 3. ACCOUNTS (Varied FMCG Accounts)
-- =============================================================================
INSERT INTO accounts (id, tenant_id, code, name, type, status, currency, cached_balance, metadata, created_at, updated_at)
VALUES
    -- Banks & Cash
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'BANK-BRI-01', 'Bank BRI Penerimaan Ritel', 'cash', 'active', 'IDR', 6500000000, '{}', NOW() - INTERVAL '100 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'BANK-BNI-01', 'Bank BNI Payroll & Pajak', 'cash', 'active', 'IDR', 3200000000, '{}', NOW() - INTERVAL '90 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'BANK-PERMATA-VA', 'Bank Permata Auto-VA FMCG', 'cash', 'active', 'IDR', 18450000000, '{}', NOW() - INTERVAL '80 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'PETTY-CAKUNG', 'Kas Kecil Gudang Cakung', 'cash', 'active', 'IDR', 500000000, '{}', NOW() - INTERVAL '70 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'PETTY-SURABAYA', 'Kas Kecil Hub Rungkut SBY', 'cash', 'active', 'IDR', 750000000, '{}', NOW() - INTERVAL '60 days', NOW()),
    
    -- Customers (Retailers & Supermarkets)
    ('cccccccc-0006-0000-0000-000000000006', '00000000-0000-0000-0000-000000000001', 'CUST-BERKAH-06', 'Toko Berkah Mandiri Depok', 'customer', 'active', 'IDR', -620000000, '{}', NOW() - INTERVAL '50 days', NOW()),
    ('cccccccc-0007-0000-0000-000000000007', '00000000-0000-0000-0000-000000000001', 'CUST-MAKMUR-07', 'Minimarket Makmur Abadi Bekasi', 'customer', 'active', 'IDR', -1450000000, '{}', NOW() - INTERVAL '45 days', NOW()),
    ('cccccccc-0008-0000-0000-000000000008', '00000000-0000-0000-0000-000000000001', 'CUST-ALFAJAYA-08', 'Grosir Alfa Jaya Tangerang', 'customer', 'active', 'IDR', -4800000000, '{}', NOW() - INTERVAL '40 days', NOW()),
    ('cccccccc-0009-0000-0000-000000000009', '00000000-0000-0000-0000-000000000001', 'CUST-PANGAN-09', 'Toko Sumber Pangan Bogor', 'customer', 'active', 'IDR', -890000000, '{}', NOW() - INTERVAL '35 days', NOW()),
    ('cccccccc-0010-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'CUST-SLAMET-10', 'Warung Kelontong Pak Slamet', 'customer', 'active', 'IDR', -320000000, '{}', NOW() - INTERVAL '30 days', NOW()),
    ('cccccccc-0011-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', 'CUST-BINTANG-11', 'Toko Bintang Kejora Bandung', 'customer', 'active', 'IDR', -2750000000, '{}', NOW() - INTERVAL '25 days', NOW()),
    ('cccccccc-0012-0000-0000-000000000012', '00000000-0000-0000-0000-000000000001', 'CUST-SUBUR-12', 'Distributor Subur Makmur Semarang', 'customer', 'active', 'IDR', -8400000000, '{}', NOW() - INTERVAL '20 days', NOW()),
    ('cccccccc-0013-0000-0000-000000000013', '00000000-0000-0000-0000-000000000001', 'CUST-REJEKI-13', 'Toko Rejeki Nomplok Surabaya', 'customer', 'active', 'IDR', -1850000000, '{}', NOW() - INTERVAL '15 days', NOW()),
    ('cccccccc-0014-0000-0000-000000000014', '00000000-0000-0000-0000-000000000001', 'CUST-HARAPAN-14', 'Warung Madura Harapan Kita', 'customer', 'active', 'IDR', -450000000, '{}', NOW() - INTERVAL '10 days', NOW()),

    -- Suppliers (Payable)
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'AP-INDOFOOD', 'Hutang Dagang PT Indofood CBP Sukses', 'payable', 'active', 'IDR', 42000000000, '{}', NOW() - INTERVAL '120 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'AP-UNILEVER', 'Hutang Dagang PT Unilever Indonesia', 'payable', 'active', 'IDR', 38500000000, '{}', NOW() - INTERVAL '110 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'AP-WINGS', 'Hutang Dagang Wings Surya Surabaya', 'payable', 'active', 'IDR', 29000000000, '{}', NOW() - INTERVAL '100 days', NOW()),

    -- Sales Rep Wallets
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'CASH-DEWI', 'Kas Sales Rep Dewi Lestari', 'sales_rep', 'active', 'IDR', 125000000, '{}', NOW() - INTERVAL '50 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'CASH-BUDI', 'Kas Sales Rep Budi Santoso', 'sales_rep', 'active', 'IDR', 280000000, '{}', NOW() - INTERVAL '40 days', NOW()),

    -- Suspense / Clearing
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'SUSPENSE-SETTLE', 'Rekening Penampung Kliring Settlement', 'suspense', 'active', 'IDR', 0, '{}', NOW() - INTERVAL '90 days', NOW())
ON CONFLICT (tenant_id, code) DO NOTHING;

-- Credit Limits for new customers
INSERT INTO credit_limits (tenant_id, customer_id, limit_amount, used_amount, effective_from, updated_at)
SELECT '00000000-0000-0000-0000-000000000001', id, 5000000000, 0, NOW() - INTERVAL '60 days', NOW()
FROM accounts
WHERE tenant_id = '00000000-0000-0000-0000-000000000001' AND type = 'customer'
ON CONFLICT (tenant_id, customer_id) DO NOTHING;

-- =============================================================================
-- 4. INVOICES (Diverse Statuses & Aging Buckets)
-- =============================================================================
-- We'll attach them to the open October 2026 period: 77777777-7777-7777-7777-777777777777
INSERT INTO invoices (id, tenant_id, customer_id, code, amount, paid_amount, due_date, status, issued_at, period_id, description, metadata, created_at, updated_at)
VALUES
    -- Current (Due in future: late Oct / Nov 2026)
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0006-0000-0000-000000000006', 'INV-2026-0110', 250000000, 0, '2026-10-25', 'open', NOW() - INTERVAL '5 days', '77777777-7777-7777-7777-777777777777', 'Pengiriman 50 Karton Indomie Goreng Spesial', '{}', NOW() - INTERVAL '5 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0007-0000-0000-000000000007', 'INV-2026-0111', 850000000, 0, '2026-10-28', 'open', NOW() - INTERVAL '3 days', '77777777-7777-7777-7777-777777777777', 'Pasokan Minyak Goreng Bimoli 2L 100 Dus', '{}', NOW() - INTERVAL '3 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0008-0000-0000-000000000008', 'INV-2026-0112', 3200000000, 0, '2026-11-05', 'open', NOW() - INTERVAL '2 days', '77777777-7777-7777-7777-777777777777', 'Pemesanan Partai Besar Susu UHT Ultra Milk', '{}', NOW() - INTERVAL '2 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0009-0000-0000-000000000009', 'INV-2026-0113', 450000000, 200000000, '2026-10-22', 'partial', NOW() - INTERVAL '8 days', '77777777-7777-7777-7777-777777777777', 'Kopi Kapal Api Spesial Mix 20 Karton', '{}', NOW() - INTERVAL '8 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0010-0000-0000-000000000010', 'INV-2026-0114', 180000000, 180000000, '2026-10-18', 'paid', NOW() - INTERVAL '6 days', '77777777-7777-7777-7777-777777777777', 'Pocari Sweat 500ml 15 Karton', '{}', NOW() - INTERVAL '6 days', NOW()),

    -- Overdue Bucket: 1–7 Days (Due Oct 09 - Oct 14)
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0001-0000-0000-000000000001', 'INV-2026-0115', 750000000, 0, '2026-10-12', 'overdue', NOW() - INTERVAL '15 days', '77777777-7777-7777-7777-777777777777', 'Sabun Lifebuoy Total 10 40 Karton', '{}', NOW() - INTERVAL '15 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0006-0000-0000-000000000006', 'INV-2026-0116', 370000000, 0, '2026-10-10', 'overdue', NOW() - INTERVAL '18 days', '77777777-7777-7777-7777-777777777777', 'Deterjen Rinso Molto 800g 30 Dus', '{}', NOW() - INTERVAL '18 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0011-0000-0000-000000000011', 'INV-2026-0117', 1200000000, 500000000, '2026-10-11', 'partial', NOW() - INTERVAL '16 days', '77777777-7777-7777-7777-777777777777', 'Teh Botol Sosro Kotak 250ml 80 Karton', '{}', NOW() - INTERVAL '16 days', NOW()),

    -- Overdue Bucket: 8–30 Days (Due Sep 16 - Oct 07)
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0002-0000-0000-000000000002', 'INV-2026-0118', 420000000, 0, '2026-09-28', 'overdue', NOW() - INTERVAL '35 days', '77777777-7777-7777-7777-777777777777', 'Biskuit Roma Kelapa 300g 50 Dus', '{}', NOW() - INTERVAL '35 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0007-0000-0000-000000000007', 'INV-2026-0119', 600000000, 0, '2026-10-02', 'overdue', NOW() - INTERVAL '30 days', '77777777-7777-7777-7777-777777777777', 'Kecap Bango Manis Refill 550ml 40 Dus', '{}', NOW() - INTERVAL '30 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0008-0000-0000-000000000008', 'INV-2026-0120', 1600000000, 0, '2026-09-20', 'overdue', NOW() - INTERVAL '40 days', '77777777-7777-7777-7777-777777777777', 'Susu Kental Manis Frisian Flag 60 Karton', '{}', NOW() - INTERVAL '40 days', NOW()),

    -- Overdue Bucket: 31–60 Days (Due Aug 16 - Sep 15)
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0004-0000-0000-000000000004', 'INV-2026-0121', 1100000000, 0, '2026-08-25', 'overdue', NOW() - INTERVAL '65 days', '77777777-7777-7777-7777-777777777777', 'Pasta Gigi Pepsodent 190g 45 Karton', '{}', NOW() - INTERVAL '65 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0012-0000-0000-000000000012', 'INV-2026-0122', 2400000000, 0, '2026-09-05', 'overdue', NOW() - INTERVAL '55 days', '77777777-7777-7777-7777-777777777777', 'Tepung Terigu Segitiga Biru 1kg 100 Sak', '{}', NOW() - INTERVAL '55 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0013-0000-0000-000000000013', 'INV-2026-0123', 950000000, 0, '2026-08-30', 'overdue', NOW() - INTERVAL '60 days', '77777777-7777-7777-7777-777777777777', 'Garam Beryodium Cap Kapal 250g 60 Karton', '{}', NOW() - INTERVAL '60 days', NOW()),

    -- Overdue Bucket: 61–90 Days (Due Jul 17 - Aug 15)
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0005-0000-0000-000000000005', 'INV-2026-0124', 650000000, 0, '2026-08-05', 'overdue', NOW() - INTERVAL '85 days', '77777777-7777-7777-7777-777777777777', 'Shampoo Pantene Sachet 10ml 80 Renceng', '{}', NOW() - INTERVAL '85 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0011-0000-0000-000000000011', 'INV-2026-0125', 1050000000, 0, '2026-07-28', 'overdue', NOW() - INTERVAL '90 days', '77777777-7777-7777-7777-777777777777', 'Wafer Tango Cokelat 130g 75 Dus', '{}', NOW() - INTERVAL '90 days', NOW()),

    -- Overdue Bucket: 90+ Days (Due before Jul 17)
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0001-0000-0000-000000000001', 'INV-2026-0126', 1250000000, 0, '2026-06-15', 'overdue', NOW() - INTERVAL '130 days', '77777777-7777-7777-7777-777777777777', 'Faktur Tunggakan Lama Indofood Q2', '{}', NOW() - INTERVAL '130 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0012-0000-0000-000000000012', 'INV-2026-0127', 3500000000, 0, '2026-05-20', 'overdue', NOW() - INTERVAL '160 days', '77777777-7777-7777-7777-777777777777', 'Faktur Tunggakan Grosir Subur Makmur', '{}', NOW() - INTERVAL '160 days', NOW()),

    -- Fully Paid Invoices
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0003-0000-0000-000000000003', 'INV-2026-0128', 1500000000, 1500000000, '2026-09-15', 'paid', NOW() - INTERVAL '40 days', '77777777-7777-7777-7777-777777777777', 'Pelunasan Prima Mart Order September', '{}', NOW() - INTERVAL '40 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0006-0000-0000-000000000006', 'INV-2026-0129', 800000000, 800000000, '2026-09-10', 'paid', NOW() - INTERVAL '45 days', '77777777-7777-7777-7777-777777777777', 'Pelunasan Toko Berkah Mandiri', '{}', NOW() - INTERVAL '45 days', NOW()),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'cccccccc-0014-0000-0000-000000000014', 'INV-2026-0130', 450000000, 450000000, '2026-10-05', 'paid', NOW() - INTERVAL '12 days', '77777777-7777-7777-7777-777777777777', 'Pelunasan Warung Harapan Kita QRIS', '{}', NOW() - INTERVAL '12 days', NOW())
ON CONFLICT (tenant_id, code) DO NOTHING;

-- =============================================================================
-- 5. TRANSACTIONS & LEDGER ENTRIES (Double-entry transfers)
-- =============================================================================
-- Helper loop to create 15 balanced transfers
DO $$
DECLARE
    t_id uuid;
    acc_bca uuid;
    acc_mandiri uuid;
    acc_bri uuid;
    acc_cash uuid;
    prev_h text := '0000000000000000000000000000000000000000000000000000000000000000';
    cur_h text;
    amt bigint;
    desc_text text;
    i int;
BEGIN
    SELECT id INTO acc_bca FROM accounts WHERE tenant_id='00000000-0000-0000-0000-000000000001' AND code='BANK-BCA';
    SELECT id INTO acc_mandiri FROM accounts WHERE tenant_id='00000000-0000-0000-0000-000000000001' AND code='BANK-MANDIRI';
    SELECT id INTO acc_bri FROM accounts WHERE tenant_id='00000000-0000-0000-0000-000000000001' AND code='BANK-BRI-01';
    SELECT id INTO acc_cash FROM accounts WHERE tenant_id='00000000-0000-0000-0000-000000000001' AND code='CASH-001';

    IF acc_bca IS NOT NULL AND acc_mandiri IS NOT NULL THEN
        FOR i IN 1..18 LOOP
            t_id := gen_random_uuid();
            amt := (10000000::bigint + (i::bigint * 2500000::bigint)) * 100::bigint; -- in minor cents (Rp 10jt - Rp 55jt)
            desc_text := 'Mutasi Antar Kas & Bank FMCG #' || i || ' - Operasional Rutin';

            INSERT INTO transactions (id, tenant_id, period_id, idempotency_key, status, description, ref_type, posted_at, created_at, updated_at, metadata)
            VALUES (t_id, '00000000-0000-0000-0000-000000000001', '77777777-7777-7777-7777-777777777777', 'IDEM-TRANSFER-TEST-' || i || '-' || EXTRACT(EPOCH FROM NOW()), 'posted', desc_text, 'manual', NOW() - (i || ' days')::interval, NOW() - (i || ' days')::interval, NOW() - (i || ' days')::interval, '{}');

            -- Debit Entry
            cur_h := encode(sha256((prev_h || t_id::text || 'debit')::bytea), 'hex');
            INSERT INTO ledger_entries (id, transaction_id, account_id, period_id, amount, type, currency, description, prev_hash, entry_hash, created_at, metadata)
            VALUES (gen_random_uuid(), t_id, CASE WHEN i % 2 = 0 THEN acc_bca ELSE acc_mandiri END, '77777777-7777-7777-7777-777777777777', amt, 'debit', 'IDR', desc_text, prev_h, cur_h, NOW() - (i || ' days')::interval, '{}');
            prev_h := cur_h;

            -- Credit Entry (Balanced)
            cur_h := encode(sha256((prev_h || t_id::text || 'credit')::bytea), 'hex');
            INSERT INTO ledger_entries (id, transaction_id, account_id, period_id, amount, type, currency, description, prev_hash, entry_hash, created_at, metadata)
            VALUES (gen_random_uuid(), t_id, CASE WHEN i % 2 = 0 THEN acc_mandiri ELSE COALESCE(acc_bri, acc_cash) END, '77777777-7777-7777-7777-777777777777', amt, 'credit', 'IDR', desc_text, prev_h, cur_h, NOW() - (i || ' days')::interval, '{}');
            prev_h := cur_h;
        END LOOP;
    END IF;
END $$;

-- =============================================================================
-- 6. RECONCILER RUNS (Historical Reconciliations)
-- =============================================================================
INSERT INTO reconciler_runs (id, tenant_id, period_id, started_at, finished_at, status, total_debit, total_credit, imbalance, hash_chain_ok, hash_chain_errors, triggered_by, metadata)
SELECT
    gen_random_uuid(),
    '00000000-0000-0000-0000-000000000001',
    p.id,
    p.period_end + INTERVAL '1 day' + (i * INTERVAL '1 hour'),
    p.period_end + INTERVAL '1 day' + (i * INTERVAL '1 hour') + (ROUND(RANDOM() * 3 + 1) * INTERVAL '1 second'),
    'balanced',
    (150000000000 + (i * 12500000000)),
    (150000000000 + (i * 12500000000)),
    0,
    true,
    0,
    'manual',
    '{"audit_mode":"deep_hash"}'
FROM accounting_periods p
CROSS JOIN generate_series(1, 2) AS i
WHERE p.tenant_id = '00000000-0000-0000-0000-000000000001' AND p.status = 'closed'
ON CONFLICT DO NOTHING;

-- Also add 2 runs for the current open period (one balanced, one with minor imbalance test)
INSERT INTO reconciler_runs (id, tenant_id, period_id, started_at, finished_at, status, total_debit, total_credit, imbalance, hash_chain_ok, hash_chain_errors, triggered_by, metadata)
VALUES
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '77777777-7777-7777-7777-777777777777', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days' + INTERVAL '2 seconds', 'balanced', 245000000000, 245000000000, 0, true, 0, 'manual', '{"verified_by":"admin"}'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '77777777-7777-7777-7777-777777777777', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day' + INTERVAL '3 seconds', 'balanced', 278000000000, 278000000000, 0, true, 0, 'api', '{"verified_by":"system_cron"}')
ON CONFLICT DO NOTHING;

-- =============================================================================
-- 7. AUDIT LOGS (Diverse Activities Across Resources)
-- =============================================================================
INSERT INTO audit_logs (id, tenant_id, actor_id, actor_type, action, resource_type, resource_id, outcome, ip_address, metadata, occurred_at)
VALUES
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'account.create', 'account', 'BANK-BRI-01', 'success', '192.168.1.15', '{"details":"New bank account added"}', NOW() - INTERVAL '2 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'transfer.post', 'transaction', 'TX-78192', 'success', '192.168.1.15', '{"amount":3500000000}', NOW() - INTERVAL '3 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'invoice.create', 'invoice', 'INV-2026-0112', 'success', '192.168.1.15', '{"customer":"CUST-ALFAJAYA-08"}', NOW() - INTERVAL '5 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '44444444-4444-4444-4444-444444444444', 'user', 'auth.login', 'auth', '44444444-4444-4444-4444-444444444444', 'success', '10.0.4.88', '{"role":"sales_rep"}', NOW() - INTERVAL '6 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'reconciler.run', 'reconciler', 'RUN-REC-2026', 'success', '192.168.1.15', '{"status":"balanced"}', NOW() - INTERVAL '8 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'currency.rate_update', 'currency', 'USD', 'success', '192.168.1.15', '{"rate":15750}', NOW() - INTERVAL '10 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'period.request_close', 'period', '77777777-2026-0008-0000-000000000008', 'success', '192.168.1.15', '{"reason":"Month-end financial closing"}', NOW() - INTERVAL '12 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'period.approve_close', 'period', '77777777-2026-0008-0000-000000000008', 'success', '192.168.1.15', '{"approver":"Finance Director"}', NOW() - INTERVAL '14 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'credit_limit.update', 'customer', 'cccccccc-0008-0000-0000-000000000008', 'success', '192.168.1.15', '{"new_limit":5000000000}', NOW() - INTERVAL '18 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '44444444-4444-4444-4444-444444444444', 'user', 'cash_collection.record', 'collection', 'COL-2026-991', 'success', '10.0.4.88', '{"store":"Toko Sumber Rezeki","amount":4500000}', NOW() - INTERVAL '22 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'account.update', 'account', 'BANK-BCA', 'success', '192.168.1.15', '{"field":"name"}', NOW() - INTERVAL '1 day'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'invoice.payment', 'invoice', 'INV-2026-0114', 'success', '192.168.1.15', '{"paid_amount":180000000}', NOW() - INTERVAL '1 day 4 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'transfer.post', 'transaction', 'TX-78190', 'success', '192.168.1.15', '{"amount":1500000000}', NOW() - INTERVAL '1 day 8 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'auth.login', 'auth', '33333333-3333-3333-3333-333333333333', 'success', '192.168.1.15', '{"role":"hq_admin"}', NOW() - INTERVAL '2 days'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'reconciler.run', 'reconciler', 'RUN-REC-2025', 'success', '192.168.1.15', '{"period":"2025-12"}', NOW() - INTERVAL '2 days 6 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'invoice.create', 'invoice', 'INV-2026-0110', 'success', '192.168.1.15', '{"amount":250000000}', NOW() - INTERVAL '3 days'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '44444444-4444-4444-4444-444444444444', 'user', 'auth.login', 'auth', '44444444-4444-4444-4444-444444444444', 'success', '10.0.4.88', '{"role":"sales_rep"}', NOW() - INTERVAL '3 days 4 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'currency.create', 'currency', 'EUR', 'success', '192.168.1.15', '{"name":"Euro"}', NOW() - INTERVAL '4 days'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'currency.create', 'currency', 'JPY', 'success', '192.168.1.15', '{"name":"Japanese Yen"}', NOW() - INTERVAL '4 days 2 hours'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'transfer.post', 'transaction', 'TX-78188', 'success', '192.168.1.15', '{"amount":2000000000}', NOW() - INTERVAL '5 days')
ON CONFLICT DO NOTHING;

COMMIT;
