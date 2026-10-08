#!/usr/bin/env bash
# =============================================================================
# Fly.io Demo Data Seeding — FMCG Wallet (production-like)
# =============================================================================
#
# Purpose: Seed minimal demo data into the FLY machine's Postgres so
# interviewers can explore the live demo immediately.
#
# Runs inside the Fly machine via `fly ssh console` then exec into psql.
#
# Differs from scripts/seed-local-data.sh (local Docker):
#   - Uses `fly ssh console` to pipe SQL (not docker compose exec)
#   - Single fixed tenant + 2 demo users (was: 2 tenants + 2 users; the
#     second tenant table doesn't exist yet in the schema, so we keep one)
#   - Same bcrypt hash for both demo users (single shared secret)
#
# Idempotent: uses INSERT ... ON CONFLICT DO NOTHING — safe to re-run.
#
# Seeds:
#   - 1 default tenant_id (UUID)
#   - 2 demo users in user_credentials (admin + sales_rep)
#   - 2 sample accounts (Cash + Bank BCA) with starting balances
#   - 1 accounting period (open)
#   - 2 fx_rates (USD->IDR + IDR->USD)
#   - 2 currency references (IDR already seeded by migration 000012)
#
# Demo credentials (printed at end):
#   admin@demo.fmcg-wallet  /  demo123  (role: admin)
#   sales@demo.fmcg-wallet  /  demo123  (role: sales_rep)
# =============================================================================

set -euo pipefail

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------
APP_NAME="${FLY_APP_NAME:-fmcg-wallet-demo}"
DB_NAME="${DB_NAME:-fmcg_wallet}"
DB_USER="${DB_USER:-fmcg}"
DB_PASSWORD="${DB_PASSWORD:-fmcg_demo_password}"

# Bcrypt hash of "demo123" (cost=10). Generated locally via:
#   go run scripts/gen-bcrypt-hash.go demo123 10
# Regenerate if you change the demo password.
ADMIN_HASH='$2a$10$DLh1KoEhiSXc7urJp3IYQeucbUcurag7PtANaOxeR7IzwBEd0KSZW'
SALES_HASH='$2a$10$DLh1KoEhiSXc7urJp3IYQeucbUcurag7PtANaOxeR7IzwBEd0KSZW'

# Fixed UUIDs for idempotency (don't change these — re-runs preserve data)
TENANT_ID='00000000-0000-0000-0000-000000000001'
USER_ADMIN_ID='33333333-3333-3333-3333-333333333333'
USER_SALES_ID='44444444-4444-4444-4444-444444444444'
PERIOD_ID='77777777-7777-7777-7777-777777777777'

# Colors (disabled if non-TTY to avoid noise in fly ssh logs)
if [ -t 1 ]; then
    GREEN='\033[0;32m'
    BLUE='\033[0;34m'
    NC='\033[0m'
else
    GREEN=''; BLUE=''; NC=''
fi
log_info() { echo -e "${BLUE}[seed-fly]${NC} $*"; }
log_ok()   { echo -e "${GREEN}[seed-fly]${NC} $*"; }

# -----------------------------------------------------------------------------
# Pre-flight
# -----------------------------------------------------------------------------
if ! command -v fly >/dev/null 2>&1; then
    echo "flyctl not installed. See: https://fly.io/docs/hands-on/install-flyctl/"
    exit 1
fi

if ! fly apps list 2>/dev/null | grep -q "^${APP_NAME}"; then
    echo "App '${APP_NAME}' not found. Run scripts/fly-deploy.sh first."
    exit 1
fi

log_info "Seeding demo data into ${APP_NAME}..."

# -----------------------------------------------------------------------------
# SQL — runs inside the Fly machine via fly ssh + psql.
# -----------------------------------------------------------------------------
# Schema notes (match migration 000013 + 000012):
#   - user_credentials has NO `username` column (PK is user_id); demo users
#     are identified by user_id directly. The LoginRequest DTO accepts user_id
#     as `username`.
#   - accounting_periods.period_start / period_end are DATE (not TIMESTAMPTZ).
#   - currencies already seeded by migration 000012 (IDR). We add USD here.
# -----------------------------------------------------------------------------
SQL_SCRIPT=$(cat <<EOSQL
BEGIN;

-- USD currency reference (IDR is seeded by migration 000012).
INSERT INTO currencies (code, name, decimal_places, is_active)
VALUES ('USD', 'US Dollar', 2, TRUE)
ON CONFLICT (code) DO NOTHING;

-- Admin user (hq_admin role). Single-row table keyed on user_id.
INSERT INTO user_credentials (
    user_id, tenant_id, password_hash, mfa_enabled,
    failed_login_count, locked_until
) VALUES (
    '${USER_ADMIN_ID}', '${TENANT_ID}', '${ADMIN_HASH}', FALSE,
    0, NULL
) ON CONFLICT (user_id) DO NOTHING;

-- Sales-rep user.
INSERT INTO user_credentials (
    user_id, tenant_id, password_hash, mfa_enabled,
    failed_login_count, locked_until
) VALUES (
    '${USER_SALES_ID}', '${TENANT_ID}', '${SALES_HASH}', FALSE,
    0, NULL
) ON CONFLICT (user_id) DO NOTHING;

-- Sample accounts (chart of accounts for demo tenant).
INSERT INTO accounts (
    id, tenant_id, code, name, type, status, currency, cached_balance, owner_id
) VALUES
    (gen_random_uuid(), '${TENANT_ID}', 'CASH-001', 'Cash on Hand',
     'cash', 'active', 'IDR', 100000000000, NULL),
    (gen_random_uuid(), '${TENANT_ID}', 'BANK-BCA', 'Bank BCA Operating',
     'cash', 'active', 'IDR', 500000000000, NULL)
ON CONFLICT (tenant_id, code) DO NOTHING;

-- Open accounting period (current month + 30 days buffer).
INSERT INTO accounting_periods (
    id, tenant_id, period_start, period_end, status
) VALUES (
    '${PERIOD_ID}', '${TENANT_ID}',
    CURRENT_DATE - 30, CURRENT_DATE + 30,
    'open'
) ON CONFLICT (id) DO NOTHING;

-- FX rates (USD <-> IDR). Required for cross-currency transfers.
INSERT INTO fx_rates (
    id, tenant_id, from_currency, to_currency, rate, source,
    effective_at, expires_at, created_by
) VALUES
    (gen_random_uuid(), '${TENANT_ID}', 'USD', 'IDR', 15750.00, 'seed',
     NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', '${USER_ADMIN_ID}'),
    (gen_random_uuid(), '${TENANT_ID}', 'IDR', 'USD', 0.00006349206349206, 'seed',
     NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', '${USER_ADMIN_ID}')
ON CONFLICT DO NOTHING;

COMMIT;

-- Verification queries (informational; non-fatal).
SELECT 'tenant_id' AS info, '${TENANT_ID}'::text AS value;
SELECT 'users' AS info, COUNT(*) AS count FROM user_credentials WHERE tenant_id = '${TENANT_ID}';
SELECT 'accounts' AS info, COUNT(*) AS count FROM accounts WHERE tenant_id = '${TENANT_ID}';
SELECT 'currencies' AS info, COUNT(*) AS count FROM currencies;
EOSQL
)

# -----------------------------------------------------------------------------
# Pipe SQL into psql running inside the Fly machine via fly ssh.
# -----------------------------------------------------------------------------
TEMP_SQL=$(mktemp /tmp/fly-seed-XXXXXX.sql)
echo "${SQL_SCRIPT}" > "${TEMP_SQL}"

log_info "Connecting to ${APP_NAME} via SSH and running SQL..."
# `fly ssh console -C` lets us run a non-interactive command. We pipe the
# SQL file via stdin so the multi-statement script is preserved.
fly ssh console --app "${APP_NAME}" --command "su - postgres -c 'psql -d ${DB_NAME} -U ${DB_USER}'" < "${TEMP_SQL}"

rm -f "${TEMP_SQL}"

log_ok "Demo data seeded successfully"

# -----------------------------------------------------------------------------
# Print credentials
# -----------------------------------------------------------------------------
cat <<EOCRED

==============================================
🌱 Demo data seeded (FLY.IO)
==============================================

Login credentials (both use password: demo123):

  Username: ${USER_ADMIN_ID}
  TenantID: ${TENANT_ID}
  Role:     admin
  Password: demo123

  Username: ${USER_SALES_ID}
  TenantID: ${TENANT_ID}
  Role:     sales_rep
  Password: demo123

Test the API:
  curl https://${APP_NAME}.fly.dev/healthz

  curl -X POST https://${APP_NAME}.fly.dev/v1/auth/login \\
    -H 'Content-Type: application/json' \\
    -d '{
      "tenant_id": "${TENANT_ID}",
      "username": "${USER_ADMIN_ID}",
      "password": "demo123"
    }'

==============================================
EOCRED
