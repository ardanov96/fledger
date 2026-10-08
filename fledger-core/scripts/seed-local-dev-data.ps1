# =============================================================================
# FMCG Wallet — Local dev seed (PowerShell, no Docker required)
# =============================================================================
# Seeds demo data into fmcg_wallet db so end-user testing has something
# to click on. Idempotent: uses ON CONFLICT DO NOTHING / fixed UUIDs.
#
# Seeds:
#   - 1 USD currency reference (IDR is seeded by migration 000012)
#   - 2 demo users (admin + sales_rep)
#   - 5 accounts (Cash, Bank, AR Control, Sales Revenue, Operating Expense)
#   - 1 open accounting period
#   - 2 fx_rates (USD<->IDR)
#   - 3 demo customers (linked to AR Control account)
#   - 3 demo invoices (open, partial, paid)
#   - 2 invoice_payments (partial + full payment)
#
# Demo credentials (both use password: DemoTest1234! — meets 12-char min policy):
#   user_id: 33333333-3333-3333-3333-333333333333  role: hq_admin
#   user_id: 44444444-4444-4444-4444-444444444444  role: sales_rep
#
# Usage:
#   $env:PGPASSWORD="fmcg_dev_password"
#   .\scripts\seed-local-dev-data.ps1
# =============================================================================

[CmdletBinding()]
param(
    [string]$PgHost = "127.0.0.1",
    [int]$PgPort = 5432,
    [string]$DbName = "fmcg_wallet",
    [string]$DbUser = "fmcg",
    [string]$DbPassword = $env:PGPASSWORD
)

$ErrorActionPreference = "Stop"
$pg = "C:\Program Files\PostgreSQL\17"
$psql = "$pg\bin\psql.exe"

if (-not $DbPassword) {
    Write-Error "DBPassword not set. Set `$env:PGPASSWORD before running."
    exit 1
}

$env:PGPASSWORD = $DbPassword

# Fixed UUIDs (idempotent re-runs)
$tenantId   = "00000000-0000-0000-0000-000000000001"
$adminId    = "33333333-3333-3333-3333-333333333333"
$salesId    = "44444444-4444-4444-4444-444444444444"
$periodId   = "77777777-7777-7777-7777-777777777777"
$accCash    = "aaaaaaaa-0001-0000-0000-000000000001"
$accBank    = "aaaaaaaa-0002-0000-0000-000000000002"
$accAR      = "aaaaaaaa-0003-0000-0000-000000000003"
$accRev     = "aaaaaaaa-0004-0000-0000-000000000004"
$accExp     = "aaaaaaaa-0005-0000-0000-000000000005"
$customer1  = "cccccccc-0001-0000-0000-000000000001"
$customer2  = "cccccccc-0002-0000-0000-000000000002"
$customer3  = "cccccccc-0003-0000-0000-000000000003"
$invOpen    = "10000001-0001-0000-0000-000000000001"
$invPartial = "10000002-0002-0000-0000-000000000002"
$invPaid    = "10000003-0003-0000-0000-000000000003"
$payment1   = "20000001-0001-0000-0000-000000000001"
$payment2   = "20000002-0002-0000-0000-000000000002"

# Bcrypt hash of "demo123" (cost=10). Generated via:
#   go run scripts/gen-bcrypt-hash.go demo123 10
$bcryptHash = '$2b$10$PI57J4P4ic01wmEE2RIxmObgnEChG3IHfXGiQ3ebY9g4T3966TCBO'

Write-Host "=== FMCG Wallet Local Dev Seed ===" -ForegroundColor Cyan
Write-Host "Target: $DbUser@$PgHost`:$PgPort/$DbName"
Write-Host ""

# --- Build SQL as a single batch (BEGIN; ... COMMIT;) ---
$sql = @"
BEGIN;

-- 1) USD currency reference (IDR seeded by migration 000012)
INSERT INTO currencies (code, name, decimal_places, is_active)
VALUES ('USD', 'US Dollar', 2, TRUE)
ON CONFLICT (code) DO NOTHING;

-- 2) Demo users
INSERT INTO user_credentials (user_id, tenant_id, password_hash, mfa_enabled, failed_login_count, locked_until)
VALUES ('$adminId', '$tenantId', '$bcryptHash', FALSE, 0, NULL)
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_credentials (user_id, tenant_id, password_hash, mfa_enabled, failed_login_count, locked_until)
VALUES ('$salesId', '$tenantId', '$bcryptHash', FALSE, 0, NULL)
ON CONFLICT (user_id) DO NOTHING;

-- 3) Chart of accounts (5 demo accounts)
-- NOTE: valid types per migration 000002: hq, outlet, sales_rep, customer,
-- revenue, receivable, payable, cash, suspense (no 'expense' type).
INSERT INTO accounts (id, tenant_id, code, name, type, status, currency, cached_balance, owner_id, metadata)
VALUES
  ('$accCash', '$tenantId', 'CASH-001',  'Cash on Hand',         'cash',     'active', 'IDR', 100000000000, NULL, '{}'),
  ('$accBank', '$tenantId', 'BANK-BCA',  'Bank BCA Operating',   'cash',     'active', 'IDR', 500000000000, NULL, '{}'),
  ('$accAR',   '$tenantId', 'AR-CTRL',   'AR Control (Receivables)','receivable','active', 'IDR',         0, NULL, '{}'),
  ('$accRev',  '$tenantId', 'SALES-REV', 'Sales Revenue',        'revenue',  'active', 'IDR',         0, NULL, '{}'),
  ('$accExp',  '$tenantId', 'OP-EXP',    'Operating Expense',    'hq',       'active', 'IDR',         0, NULL, '{}')
ON CONFLICT (tenant_id, code) DO NOTHING;

-- 4) Open accounting period (covers today +/- 30 days)
INSERT INTO accounting_periods (id, tenant_id, period_start, period_end, status)
VALUES ('$periodId', '$tenantId', CURRENT_DATE - 30, CURRENT_DATE + 30, 'open')
ON CONFLICT (id) DO NOTHING;

-- 5) FX rates (USD <-> IDR)
INSERT INTO fx_rates (id, tenant_id, from_currency, to_currency, rate, source, effective_at, expires_at, created_by)
VALUES
  (gen_random_uuid(), '$tenantId', 'USD', 'IDR', 15750.00, 'seed', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', '$adminId'),
  (gen_random_uuid(), '$tenantId', 'IDR', 'USD', 0.00006349206349206, 'seed', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', '$adminId')
ON CONFLICT DO NOTHING;

-- 6) Customers (type='customer' per migration 000002)
INSERT INTO accounts (id, tenant_id, code, name, type, status, currency, cached_balance, owner_id, metadata)
VALUES
  ('$customer1', '$tenantId', 'CUST-TOKO-A',  'Toko Sumber Rezeki',   'customer',  'active', 'IDR', -2500000000, NULL, '{"customer_name":"Pak Ahmad"}'),
  ('$customer2', '$tenantId', 'CUST-WARUNG-B','Warung Berkah Jaya',   'customer',  'active', 'IDR',  -750000000, NULL, '{"customer_name":"Bu Siti"}'),
  ('$customer3', '$tenantId', 'CUST-MINIMARKET','Minimarket Sumber',  'customer',  'active', 'IDR',         0, NULL, '{"customer_name":"Manager Cici"}')
ON CONFLICT (tenant_id, code) DO NOTHING;

-- 7) Demo invoices
-- IMPORTANT: paid_amount must be set by the invoice_payments trigger, not by
-- the invoice INSERT, otherwise constraint invoices_paid_lte_amount fails.
-- INV-2026-0001: open (unpaid), Rp 2.5M, due in 14 days
INSERT INTO invoices (id, tenant_id, customer_id, code, amount, paid_amount, due_date, status, period_id, description)
VALUES ('$invOpen', '$tenantId', '$customer1', 'INV-2026-0001', 2500000000, 0, CURRENT_DATE + 14, 'open', '$periodId', 'Order #1234 - groceries')
ON CONFLICT (tenant_id, code) DO NOTHING;

-- INV-2026-0002: paid_amount=0; trigger will set to 500M after allocation insert below
INSERT INTO invoices (id, tenant_id, customer_id, code, amount, paid_amount, due_date, status, period_id, description)
VALUES ('$invPartial', '$tenantId', '$customer2', 'INV-2026-0002', 1000000000, 0, CURRENT_DATE + 7, 'open', '$periodId', 'Order #5678 - mixed')
ON CONFLICT (tenant_id, code) DO NOTHING;

-- INV-2026-0003: paid_amount=0; trigger will set to 5B after allocation insert below
INSERT INTO invoices (id, tenant_id, customer_id, code, amount, paid_amount, due_date, status, period_id, description)
VALUES ('$invPaid', '$tenantId', '$customer3', 'INV-2026-0003', 5000000000, 0, CURRENT_DATE - 30, 'open', '$periodId', 'Order #9012 - bulk')
ON CONFLICT (tenant_id, code) DO NOTHING;

-- 8) Invoice payments (allocation table: 1 payment_id may have N allocations)
-- Schema (migration 000006): invoice_payments(id, tenant_id, payment_id, invoice_id,
-- customer_id, amount, method, allocated_at, metadata). No separate payments table.
-- Payment 1: partial for INV-2026-0002 (Rp 500K of Rp 1M)
INSERT INTO invoice_payments (id, tenant_id, payment_id, invoice_id, customer_id, amount, method, allocated_at, metadata)
VALUES ('aaaaaaaa-bbbb-cccc-dddd-000000000001', '$tenantId', '$payment1', '$invPartial', '$customer2', 500000000, 'transfer', NOW() - INTERVAL '3 days', '{"note":"partial payment via transfer"}')
ON CONFLICT (id) DO NOTHING;

-- Payment 2: full for INV-2026-0003 (Rp 5M)
INSERT INTO invoice_payments (id, tenant_id, payment_id, invoice_id, customer_id, amount, method, allocated_at, metadata)
VALUES ('aaaaaaaa-bbbb-cccc-dddd-000000000002', '$tenantId', '$payment2', '$invPaid', '$customer3', 5000000000, 'cash', NOW() - INTERVAL '15 days', '{"note":"full cash payment"}')
ON CONFLICT (id) DO NOTHING;

COMMIT;
"@

Write-Host "Seeding demo data..." -ForegroundColor Yellow
# Write SQL to a temp file to avoid PowerShell argument parsing issues with
# curly braces in JSON values.
$tmpSql = Join-Path $env:TEMP "fmcg-seed.sql"
[System.IO.File]::WriteAllText($tmpSql, $sql, [System.Text.Encoding]::UTF8)
& $psql -U $DbUser -h $PgHost -p $PgPort -d $DbName -w -v ON_ERROR_STOP=1 -f $tmpSql 2>&1 | ForEach-Object { Write-Host "  $_" }
Remove-Item $tmpSql -Force -ErrorAction SilentlyContinue

# --- Verification queries ---
Write-Host ""
Write-Host "Verification:" -ForegroundColor Yellow
$verifySql = @"
SELECT 'currencies'   AS info, COUNT(*)::text AS count FROM currencies;
SELECT 'users'        AS info, COUNT(*)::text AS count FROM user_credentials WHERE tenant_id = '$tenantId';
SELECT 'accounts'     AS info, COUNT(*)::text AS count FROM accounts WHERE tenant_id = '$tenantId';
SELECT 'periods'      AS info, COUNT(*)::text AS count FROM accounting_periods WHERE tenant_id = '$tenantId';
SELECT 'fx_rates'     AS info, COUNT(*)::text AS count FROM fx_rates WHERE tenant_id = '$tenantId';
SELECT 'invoices'     AS info, COUNT(*)::text AS count FROM invoices WHERE tenant_id = '$tenantId';
SELECT 'payments'     AS info, COUNT(*)::text AS count FROM invoice_payments WHERE tenant_id = '$tenantId';
SELECT 'allocations'  AS info, COUNT(*)::text AS count FROM invoice_payments;
"@
& $psql -U $DbUser -h $PgHost -p $PgPort -d $DbName -w -c $verifySql 2>&1 | ForEach-Object { Write-Host "  $_" }

Write-Host ""
Write-Host "=============================================" -ForegroundColor Green
Write-Host "Demo data seeded successfully" -ForegroundColor Green
Write-Host "=============================================" -ForegroundColor Green
Write-Host ""
Write-Host "Login credentials (both use password: DemoTest1234!):" -ForegroundColor Cyan
Write-Host "  Admin (hq_admin):     $adminId"
Write-Host "  Sales  (sales_rep):   $salesId"
Write-Host "  Tenant:              $tenantId"
Write-Host ""
Write-Host "Test login:"
Write-Host "  curl -X POST http://localhost:8080/v1/auth/login -H `"Content-Type: application/json`" -d '{\"tenant_id\":\"$tenantId\",\"username\":\"$adminId\",\"password\":\"DemoTest1234!\"}'"

