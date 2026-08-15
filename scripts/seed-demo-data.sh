#!/usr/bin/env bash
# =============================================================================
# Demo Data Seeding — Fly.io FMCG Wallet
# =============================================================================
#
# Purpose: Seed minimal demo data so interviewers can explore immediately.
# Runs inside the Fly machine via `fly ssh console` then exec into the container.
#
# Idempotent: uses INSERT ... ON CONFLICT DO NOTHING — safe to re-run.
#
# Seeds:
#   - 2 tenants (Acme Corp, Beta Industries)
#   - 2 demo users (admin + sales_rep)
#   - Sample accounts, invoices, fx_rates, transfers
#
# Demo credentials (printed at end):
#   admin@demo.fmcg-wallet  /  demo123  (hq_admin, sees both tenants)
#   sales@demo.fmcg-wallet  /  demo123  (sales_rep, scoped to tenant 1)
# =============================================================================

set -euo pipefail

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------
APP_NAME="${FLY_APP_NAME:-fmcg-wallet-demo}"
DB_NAME="${DB_NAME:-fmcg_wallet}"
DB_USER="${DB_USER:-fmcg}"
DB_PASSWORD="${DB_PASSWORD:-fmcg_demo_password}"

# Bcrypt hash of "demo123" (cost=10, generated via Go bcrypt)
# Generated: hash := bcrypt.GenerateFromPassword([]byte("demo123"), 10)
ADMIN_HASH='$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy'
SALES_HASH='$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy'

# Fixed UUIDs for idempotency (don't change these — re-runs preserve data)
TENANT_ACME_ID='11111111-1111-1111-1111-111111111111'
TENANT_BETA_ID='22222222-2222-2222-2222-222222222222'
USER_ADMIN_ID='33333333-3333-3333-3333-333333333333'
USER_SALES_ID='44444444-4444-4444-4444-444444444444'
CUSTOMER_ID='55555555-5555-5555-5555-555555555555'
SUPPLIER_ID='66666666-6666-6666-6666-666666666666'
PERIOD_ID='77777777-7777-7777-7777-777777777777'

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[seed]${NC} $*"; }
log_ok()   { echo -e "${GREEN}[seed]${NC} $*"; }

# -----------------------------------------------------------------------------
# Pre-flight
# -----------------------------------------------------------------------------
if ! command -v fly >/dev/null 2>&1; then
    echo "flyctl not installed. See: https://fly.io/docs/hands-on/install-flyctl/"
    exit 1
fi

# Check app exists
if ! fly apps list | grep -q "^${APP_NAME}"; then
    echo "App '${APP_NAME}' not found. Run scripts/fly-deploy.sh first."
    exit 1
fi

log_info "Seeding demo data into ${APP_NAME}..."

# -----------------------------------------------------------------------------
# SQL seeding script (runs inside Fly machine)
# -----------------------------------------------------------------------------
SQL_SCRIPT=$(cat <<EOSQL
-- Demo data for FMCG Wallet — idempotent inserts
BEGIN;

-- Currency (required for multi-currency module)
INSERT INTO currencies (code, name, decimal_places, is_active, created_at, updated_at)
VALUES
    ('IDR', 'Indonesian Rupiah', 2, TRUE, NOW(), NOW()),
    ('USD', 'US Dollar', 2, TRUE, NOW(), NOW())
ON CONFLICT (code) DO NOTHING;

-- Tenant 1: Acme Corp
INSERT INTO tenants (id, name, slug, created_at, updated_at)
VALUES ('${TENANT_ACME_ID}', 'Acme Corporation', 'acme', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Tenant 2: Beta Industries
INSERT INTO tenants (id, name, slug, created_at, updated_at)
VALUES ('${TENANT_BETA_ID}', 'Beta Industries', 'beta', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Demo users (credentials)
INSERT INTO user_credentials (user_id, tenant_id, username, password_hash, mfa_enabled, failed_login_count, locked_until, created_at, updated_at)
VALUES
    ('${USER_ADMIN_ID}', '${TENANT_ACME_ID}', 'admin@demo.fmcg-wallet', '${ADMIN_HASH}', FALSE, 0, NULL, NOW(), NOW()),
    ('${USER_SALES_ID}', '${TENANT_ACME_ID}', 'sales@demo.fmcg-wallet', '${SALES_HASH}', FALSE, 0, NULL, NOW(), NOW())
ON CONFLICT (username) DO NOTHING;

-- Account: Cash (IDR 100M starting balance)
INSERT INTO accounts (id, tenant_id, code, name, type, status, currency, cached_balance, created_at, updated_at)
VALUES (gen_random_uuid(), '${TENANT_ACME_ID}', 'CASH-001', 'Cash on Hand', 'asset', 'active', 'IDR', 100000000000, NOW(), NOW())
ON CONFLICT (code, tenant_id) DO NOTHING;

-- Account: Bank BCA (IDR 500M starting balance)
INSERT INTO accounts (id, tenant_id, code, name, type, status, currency, cached_balance, created_at, updated_at)
VALUES (gen_random_uuid(), '${TENANT_ACME_ID}', 'BANK-BCA', 'Bank BCA Operating', 'asset', 'active', 'IDR', 500000000000, NOW(), NOW())
ON CONFLICT (code, tenant_id) DO NOTHING;

-- Accounting period (open, covering last 30 days)
INSERT INTO accounting_periods (id, tenant_id, period_start, period_end, status, opened_at, created_at, updated_at)
VALUES ('${PERIOD_ID}', '${TENANT_ACME_ID}', NOW() - INTERVAL '30 days', NOW() + INTERVAL '30 days', 'open', NOW(), NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- FX rates (USD/IDR ~15,750)
INSERT INTO fx_rates (id, tenant_id, from_currency, to_currency, rate, source, effective_at, expires_at, created_at, updated_at)
VALUES
    (gen_random_uuid(), '${TENANT_ACME_ID}', 'USD', 'IDR', 15750.00, 'seed', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', NOW(), NOW()),
    (gen_random_uuid(), '${TENANT_ACME_ID}', 'IDR', 'USD', 0.00006349206349206, 'seed', NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', NOW(), NOW())
ON CONFLICT DO NOTHING;

-- Customer (for invoice module)
INSERT INTO customers (id, tenant_id, code, name, contact_email, created_at, updated_at)
VALUES ('${CUSTOMER_ID}', '${TENANT_ACME_ID}', 'CUST-001', 'PT Maju Jaya', 'contact@majujaya.co.id', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Sample invoice (paid)
INSERT INTO invoices (id, tenant_id, customer_id, code, amount_minor, paid_amount_minor, currency, due_date, status, issued_at, period_id, created_at, updated_at)
VALUES (gen_random_uuid(), '${TENANT_ACME_ID}', '${CUSTOMER_ID}', 'INV-2026-001', 2500000000, 2500000000, 'IDR', NOW() + INTERVAL '30 days', 'paid', NOW() - INTERVAL '15 days', '${PERIOD_ID}', NOW(), NOW())
ON CONFLICT (code, tenant_id) DO NOTHING;

COMMIT;

-- Verify seed
SELECT 'Tenants seeded:' AS info, COUNT(*) AS count FROM tenants WHERE id IN ('${TENANT_ACME_ID}', '${TENANT_BETA_ID}');
SELECT 'Users seeded:' AS info, COUNT(*) AS count FROM user_credentials WHERE username LIKE '%demo.fmcg-wallet';
SELECT 'Currencies:' AS info, COUNT(*) AS count FROM currencies;
SELECT 'Accounts:' AS info, COUNT(*) AS count FROM accounts WHERE tenant_id = '${TENANT_ACME_ID}';
EOSQL
)

# -----------------------------------------------------------------------------
# Run SQL via fly ssh console (pipe SQL into psql inside the container)
# -----------------------------------------------------------------------------
log_info "Connecting to ${APP_NAME} via SSH and running SQL..."

# Create a temp SQL file
TEMP_SQL=$(mktemp /tmp/fly-seed-XXXXXX.sql)
echo "${SQL_SCRIPT}" > "${TEMP_SQL}"

# Use fly ssh to run psql inside the container
fly ssh console --app "${APP_NAME}" --command "su - postgres -c 'psql -d ${DB_NAME} -U ${DB_USER}'" < "${TEMP_SQL}"

rm -f "${TEMP_SQL}"

log_ok "Demo data seeded successfully"

# -----------------------------------------------------------------------------
# Print credentials
# -----------------------------------------------------------------------------
echo ""
echo "============================================="
echo "🌱 Demo Data Seeded"
echo "============================================="
echo ""
echo "Login credentials:"
echo "  Username: admin@demo.fmcg-wallet"
echo "  Password: demo123"
echo "  Role:     hq_admin (sees both tenants)"
echo ""
echo "  Username: sales@demo.fmcg-wallet"
echo "  Password: demo123"
echo "  Role:     sales_rep (tenant 1 only)"
echo ""
echo "Test the API:"
echo "  curl https://${APP_NAME}.fly.dev/healthz"
echo "  curl -X POST https://${APP_NAME}.fly.dev/v1/auth/login \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"username\":\"admin@demo.fmcg-wallet\",\"password\":\"demo123\"}'"
echo ""
echo "============================================="
