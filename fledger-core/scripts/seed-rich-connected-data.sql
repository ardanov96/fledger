BEGIN;

-- 1. Currencies
INSERT INTO currencies (code, name, decimal_places, is_active)
VALUES 
  ('IDR', 'Indonesian Rupiah', 2, TRUE),
  ('USD', 'US Dollar', 2, TRUE),
  ('SGD', 'Singapore Dollar', 2, TRUE)
ON CONFLICT (code) DO UPDATE SET is_active = TRUE;

-- 2. Demo User Credentials (Password: DemoTest1234!)
INSERT INTO user_credentials (user_id, tenant_id, password_hash, mfa_enabled, failed_login_count, locked_until)
VALUES
  ('33333333-3333-3333-3333-333333333333', '00000000-0000-0000-0000-000000000001', '$2b$10$PI57J4P4ic01wmEE2RIxmObgnEChG3IHfXGiQ3ebY9g4T3966TCBO', FALSE, 0, NULL),
  ('44444444-4444-4444-4444-444444444444', '00000000-0000-0000-0000-000000000001', '$2b$10$PI57J4P4ic01wmEE2RIxmObgnEChG3IHfXGiQ3ebY9g4T3966TCBO', FALSE, 0, NULL)
ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash;

-- 3. Accounting Periods
UPDATE accounting_periods 
SET period_start = '2026-10-01', period_end = '2026-10-31', status = 'open'
WHERE id = '77777777-7777-7777-7777-777777777777';

INSERT INTO accounting_periods (id, tenant_id, period_start, period_end, status, closed_at, closed_by)
VALUES
  ('77777777-7777-7777-7777-000000000009', '00000000-0000-0000-0000-000000000001', '2026-09-01', '2026-09-30', 'closed', '2026-10-01 00:00:00Z', '33333333-3333-3333-3333-333333333333')
ON CONFLICT (id) DO UPDATE SET 
  status = EXCLUDED.status, 
  period_start = EXCLUDED.period_start, 
  period_end = EXCLUDED.period_end, 
  closed_at = EXCLUDED.closed_at;

-- 4. Accounts (Cash, Banks, AR, Revenue, Expense, Customers)
INSERT INTO accounts (id, tenant_id, code, name, type, status, currency, cached_balance, metadata)
VALUES
  ('aaaaaaaa-0001-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'CASH-001', 'Cash on Hand HQ', 'cash', 'active', 'IDR', 15000000000, '{"location":"HQ Jakarta"}'),
  ('aaaaaaaa-0002-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'BANK-BCA', 'Bank BCA Operasional', 'cash', 'active', 'IDR', 85000000000, '{"account_no":"8820192341"}'),
  ('aaaaaaaa-0006-0000-0000-000000000006', '00000000-0000-0000-0000-000000000001', 'BANK-MANDIRI', 'Bank Mandiri Penerimaan', 'cash', 'active', 'IDR', 42000000000, '{"account_no":"1240098765"}'),
  ('aaaaaaaa-0007-0000-0000-000000000007', '00000000-0000-0000-0000-000000000001', 'CASH-JONI', 'Kas Sales Rep Joni', 'cash', 'active', 'IDR', 2500000000, '{"sales_rep":"Joni Hermanto","area":"Jakarta Barat"}'),
  ('aaaaaaaa-0008-0000-0000-000000000008', '00000000-0000-0000-0000-000000000001', 'CASH-RIAN', 'Kas Sales Rep Rian', 'cash', 'active', 'IDR', 1200000000, '{"sales_rep":"Rian Hidayat","area":"Jakarta Selatan"}'),
  ('aaaaaaaa-0003-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', 'AR-CTRL', 'AR Control (Receivables)', 'receivable', 'active', 'IDR', 10350000000, '{}'),
  ('aaaaaaaa-0004-0000-0000-000000000004', '00000000-0000-0000-0000-000000000001', 'SALES-REV', 'Sales Revenue', 'revenue', 'active', 'IDR', 155000000000, '{}'),
  ('aaaaaaaa-0005-0000-0000-000000000005', '00000000-0000-0000-0000-000000000001', 'OP-EXP', 'Operating Expense HQ', 'hq', 'active', 'IDR', 4500000000, '{}'),
  ('cccccccc-0001-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'CUST-TOKO-A', 'Toko Sumber Rezeki', 'customer', 'active', 'IDR', -4350000000, '{"owner":"Pak Ahmad","address":"Jl. Daan Mogot No. 12"}'),
  ('cccccccc-0002-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'CUST-WARUNG-B', 'Warung Berkah Jaya', 'customer', 'active', 'IDR', -500000000, '{"owner":"Bu Siti","address":"Jl. Tebet Raya No. 45"}'),
  ('cccccccc-0003-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', 'CUST-PRIMA-C', 'Minimarket Prima Mart', 'customer', 'active', 'IDR', 0, '{"owner":"Pak Budi","address":"Jl. Raya Serpong No. 8"}'),
  ('cccccccc-0004-0000-0000-000000000004', '00000000-0000-0000-0000-000000000001', 'CUST-SENTOSA-D', 'Agen Sentosa Abadi', 'customer', 'active', 'IDR', -3500000000, '{"owner":"Ko Hendra","address":"Jl. Ahmad Yani No. 99"}'),
  ('cccccccc-0005-0000-0000-000000000005', '00000000-0000-0000-0000-000000000001', 'CUST-MADURA-E', 'Warung Madura 24 Jam', 'customer', 'active', 'IDR', -2300000000, '{"owner":"Cak Rohim","address":"Jl. Margonda Raya No. 17"}')
ON CONFLICT (id) DO UPDATE SET
  code = EXCLUDED.code,
  name = EXCLUDED.name,
  type = EXCLUDED.type,
  cached_balance = EXCLUDED.cached_balance,
  metadata = EXCLUDED.metadata;

-- 5. FX Rates
DELETE FROM fx_rates WHERE tenant_id = '00000000-0000-0000-0000-000000000001';
INSERT INTO fx_rates (id, tenant_id, from_currency, to_currency, rate, source, effective_at, expires_at, created_by)
VALUES
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'USD', 'IDR', 15750.00, 'bank', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', '33333333-3333-3333-3333-333333333333'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'IDR', 'USD', 0.000063492063492, 'bank', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', '33333333-3333-3333-3333-333333333333'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'SGD', 'IDR', 11850.00, 'bank', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', '33333333-3333-3333-3333-333333333333'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'IDR', 'SGD', 0.000084388185654, 'bank', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', '33333333-3333-3333-3333-333333333333');

-- 6. Invoices
DELETE FROM invoice_payments WHERE tenant_id = '00000000-0000-0000-0000-000000000001';
DELETE FROM invoices WHERE tenant_id = '00000000-0000-0000-0000-000000000001';

INSERT INTO invoices (id, tenant_id, customer_id, code, amount, paid_amount, due_date, status, period_id, description)
VALUES
  ('10000001-0101-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'cccccccc-0001-0000-0000-000000000001', 'INV-2026-0101', 2500000000, 0, CURRENT_DATE + 14, 'open', '77777777-7777-7777-7777-777777777777', 'PO #8812 - 50 Dus Minyak Goreng Bimoli 2L & 20 Karung Beras Rojo'),
  ('10000001-0089-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'cccccccc-0001-0000-0000-000000000001', 'INV-2026-0089', 1850000000, 0, CURRENT_DATE - 35, 'open', '77777777-7777-7777-7777-000000000009', 'PO #8140 - 100 Dus Gula Pasir Gulaku & Tepung Segitiga Biru'),
  ('10000002-0102-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', 'cccccccc-0002-0000-0000-000000000002', 'INV-2026-0102', 1200000000, 0, CURRENT_DATE - 5, 'open', '77777777-7777-7777-7777-777777777777', 'PO #8855 - Paket Sabun Mandi Lifebuoy & Deterjen Rinso Matic'),
  ('10000003-0095-0000-0000-000000000004', '00000000-0000-0000-0000-000000000001', 'cccccccc-0003-0000-0000-000000000003', 'INV-2026-0095', 5000000000, 0, CURRENT_DATE - 15, 'open', '77777777-7777-7777-7777-000000000009', 'PO #8720 - 150 Karton Minuman Teh Botol & Biskuit Khong Guan'),
  ('10000004-0098-0000-0000-000000000005', '00000000-0000-0000-0000-000000000001', 'cccccccc-0004-0000-0000-000000000004', 'INV-2026-0098', 3500000000, 0, CURRENT_DATE - 20, 'open', '77777777-7777-7777-7777-000000000009', 'PO #8790 - 200 Dus Indomie Goreng & Susu Kental Manis Frisian Flag'),
  ('10000005-0105-0000-0000-000000000006', '00000000-0000-0000-0000-000000000001', 'cccccccc-0005-0000-0000-000000000005', 'INV-2026-0105', 1500000000, 0, CURRENT_DATE + 25, 'open', '77777777-7777-7777-7777-777777777777', 'PO #8901 - 30 Slop Rokok Sampoerna Mild & Kopi Kapal Api Mix'),
  ('10000005-0072-0000-0000-000000000007', '00000000-0000-0000-0000-000000000001', 'cccccccc-0005-0000-0000-000000000005', 'INV-2026-0072', 800000000, 0, CURRENT_DATE - 95, 'open', '77777777-7777-7777-7777-000000000009', 'PO #7990 - Pengadaan Etalase & Minuman Isotonik Mizone');

-- 7. Invoice Payments & Allocations (Triggers update invoice status)
INSERT INTO invoice_payments (id, tenant_id, payment_id, invoice_id, customer_id, amount, method, allocated_at, metadata)
VALUES
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', gen_random_uuid(), '10000002-0102-0000-0000-000000000003', 'cccccccc-0002-0000-0000-000000000002', 700000000, 'transfer', NOW() - INTERVAL '3 days', '{"note":"Cicilan termin 1 via transfer BCA"}'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', gen_random_uuid(), '10000003-0095-0000-0000-000000000004', 'cccccccc-0003-0000-0000-000000000003', 5000000000, 'transfer', NOW() - INTERVAL '15 days', '{"note":"Pelunasan penuh PO #8720 via transfer BCA"}');

-- Mark overdue status for invoices that are past due date and still open
UPDATE invoices 
SET status = 'overdue' 
WHERE tenant_id = '00000000-0000-0000-0000-000000000001' AND due_date < CURRENT_DATE AND status = 'open';


-- 9. Reconciler Runs
DELETE FROM reconciler_runs WHERE tenant_id = '00000000-0000-0000-0000-000000000001';
INSERT INTO reconciler_runs (id, tenant_id, period_id, started_at, finished_at, status, total_debit, total_credit, imbalance, hash_chain_ok, hash_chain_errors, triggered_by)
VALUES
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '77777777-7777-7777-7777-000000000009', NOW() - INTERVAL '7 days', NOW() - INTERVAL '7 days' + INTERVAL '2 seconds', 'balanced', 45000000000, 45000000000, 0, TRUE, 0, 'api'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '77777777-7777-7777-7777-777777777777', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour' + INTERVAL '1 second', 'balanced', 14700000000, 14700000000, 0, TRUE, 0, 'scheduler');

-- 10. Audit Logs
DELETE FROM audit_logs WHERE tenant_id = '00000000-0000-0000-0000-000000000001';
INSERT INTO audit_logs (id, tenant_id, actor_id, actor_type, action, resource_type, resource_id, outcome, request_id, ip_address, user_agent, metadata, occurred_at)
VALUES
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'auth.login', 'user', '33333333-3333-3333-3333-333333333333', 'success', 'req-001', '127.0.0.1'::inet, 'Mozilla/5.0', '{"username":"admin@fmcg.com","method":"password"}', NOW() - INTERVAL '48 hours'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'period.close', 'period', '77777777-7777-7777-7777-000000000009', 'success', 'req-002', '127.0.0.1'::inet, 'Mozilla/5.0', '{"period":"2026-09","closed_by":"hq_admin"}', NOW() - INTERVAL '168 hours'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'transfer.create', 'transfer', 'TXN-001', 'success', 'req-003', '127.0.0.1'::inet, 'Mozilla/5.0', '{"amount":20000000,"from":"CASH-JONI","to":"BANK-BCA"}', NOW() - INTERVAL '48 hours'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'transfer.create', 'transfer', 'TXN-002', 'success', 'req-004', '127.0.0.1'::inet, 'Mozilla/5.0', '{"amount":15000000,"from":"BANK-BCA","to":"CASH-001"}', NOW() - INTERVAL '72 hours'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'invoice.create', 'invoice', '10000001-0101-0000-0000-000000000001', 'success', 'req-005', '127.0.0.1'::inet, 'Mozilla/5.0', '{"code":"INV-2026-0101","customer":"Toko Sumber Rezeki","amount":25000000}', NOW() - INTERVAL '24 hours'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'invoice.create', 'invoice', '10000005-0105-0000-0000-000000000006', 'success', 'req-006', '127.0.0.1'::inet, 'Mozilla/5.0', '{"code":"INV-2026-0105","customer":"Warung Madura 24 Jam","amount":15000000}', NOW() - INTERVAL '12 hours'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'user', 'payment.record', 'payment', '10000002-0102-0000-0000-000000000003', 'success', 'req-007', '127.0.0.1'::inet, 'Mozilla/5.0', '{"code":"INV-2026-0102","customer":"Warung Berkah Jaya","amount":7000000}', NOW() - INTERVAL '72 hours'),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'system', 'reconciler.run', 'reconciler', '77777777-7777-7777-7777-777777777777', 'success', 'req-008', '127.0.0.1'::inet, 'System-Scheduler', '{"status":"balanced","imbalance":0,"hash_chain_ok":true}', NOW() - INTERVAL '1 hour');

COMMIT;
