#!/usr/bin/env bash
# =============================================================================
# Local Demo Data Seeding — FMCG Wallet (development only)
# =============================================================================
#
# Purpose: Seed minimal demo data into the LOCAL docker-compose Postgres so
# you can immediately run an E2E smoke test against `make run-api`.
#
# Differs from scripts/seed-demo-data.sh (Fly.io only):
#   - Runs locally via `docker compose exec` (no `fly ssh`)
#   - Matches the actual schema produced by migrations/000001..000016
#   - Uses the same bcrypt hash for both demo users (single shared secret)
#   - Avoids INSERT into tables that don't exist yet (`tenants`, `customers`)
#
# Idempotent: uses INSERT ... ON CONFLICT DO NOTHING — safe to re-run.
#
# Seeds:
#   - 1 default tenant_id (UUID)
#   - 2 demo users in user_credentials (admin + sales_rep)
#   - 2 sample accounts (Cash + Bank BCA) with starting balances
#   - 1 accounting period (open)
#   - 2 fx_rates (USD->IDR + IDR->USD)
#   - 1 currency reference (USD — IDR already seeded by migration 000012)
#
# Demo credentials (printed at end):
#   admin@demo.fmcg-wallet  /  demo123  (role: admin)
#   sales@demo.fmcg-wallet  /  demo123  (role: sales_rep)
# =============================================================================

set -euo pipefail

# -----------------------------------------------------------------------------
# Configuration (matches docker-compose.yml + .env.example)
# -----------------------------------------------------------------------------
DB_NAME="${DB_NAME:-fmcg_wallet}"
DB_USER="${DB_USER:-fmcg}"

# Bcrypt hash of "demo123" (cost=10). Generated locally via:
#   go run scripts/gen-bcrypt-hash.go demo123 10
# Regenerate if you change the demo password.
ADMIN_HASH='$2a$10$DLh1KoEhiSXc7urJp3IYQeucbUcurag7PtANaOxeR7IzwBEd0KSZW'
SALES_HASH='$2a$10$DLh1KoEhiSXc7urJp3IYQeucbUcurag7PtANaOxeR7IzwBEd0KSZW'

# Fixed UUIDs for idempotency
TENANT_ID='00000000-0000-0000-0000-000000000001'
USER_ADMIN_ID='33333333-3333-3333-3333-333333333333'
USER_SALES_ID='44444444-4444-4444-4444-444444444444'
PERIOD_ID='77777777-7777-7777-7777-777777777777'

# -----------------------------------------------------------------------------
# Pre-flight
# -----------------------------------------------------------------------------
if ! command -v docker >/dev/null 2>&1; then
    echo "docker not installed. See: https://docs.docker.com/engine/install/"
    exit 1
fi

if ! docker compose ps --status running postgres 2>/dev/null | grep -q postgres; then
    echo "Postgres container not running. Start with: make up"
    exit 1
fi

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'
log_info() { echo -e "${BLUE}[seed-local]${NC} $*"; }
log_ok()   { echo -e "${GREEN}[seed-local]${NC} $*"; }

log_info "Seeding demo data into local Postgres (${DB_USER}@fmcg_postgres/${DB_NAME})..."

# -----------------------------------------------------------------------------
# SQL — runs inside the running fmcg_postgres container via psql.
# -----------------------------------------------------------------------------
# Notes on schema choices:
#   - user_credentials has NO `username` column (PK is user_id); we identify
#     users via user_id directly. The LoginRequest DTO accepts user_id or
#     any opaque identifier; for demo we use the user_id directly.
#   - accounts.code is unique per tenant (accounts_tenant_code_unique).
#   - currencies already seeded by migration 000012 (IDR). We only add USD.
#   - We do NOT insert into a `tenants` table — none exists yet. tenant_id
#     is a plain UUID column everywhere.
# -----------------------------------------------------------------------------
SQL=$(cat <<EOSQL
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
-- 1) HQ Cash account (type=cash, IDR 100M starting balance).
INSERT INTO accounts (
    id, tenant_id, code, name, type, status, currency, cached_balance, owner_id
) VALUES (
    gen_random_uuid(), '${TENANT_ID}', 'CASH-001', 'Cash on Hand',
    'cash', 'active', 'IDR', 100000000000, NULL
) ON CONFLICT (tenant_id, code) DO NOTHING;

-- 2) HQ Bank BCA account (type=cash used as bank-cash surrogate, IDR 500M).
--    (The CHECK constraint allows: hq|outlet|sales_rep|customer|revenue|receivable|payable|cash|suspense.
--     For demo we keep using `cash` to represent bank-side liquid funds.)
INSERT INTO accounts (
    id, tenant_id, code, name, type, status, currency, cached_balance, owner_id
) VALUES (
    gen_random_uuid(), '${TENANT_ID}', 'BANK-BCA', 'Bank BCA Operating',
    'cash', 'active', 'IDR', 500000000000, NULL
) ON CONFLICT (tenant_id, code) DO NOTHING;

-- Open accounting period covering last 30 days to next 30 days.
-- Schema notes: period_start/period_end are DATE (not TIMESTAMPTZ). No
-- `opened_at` column exists; closed_at captures when status flips to closed.
INSERT INTO accounting_periods (
    id, tenant_id, period_start, period_end, status
) VALUES (
    '${PERIOD_ID}', '${TENANT_ID}',
    CURRENT_DATE - 30, CURRENT_DATE + 30,
    'open'
) ON CONFLICT (id) DO NOTHING;

-- FX rates (USD <-> IDR). required for cross-currency transfers.
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
# Execute via docker compose exec
# -----------------------------------------------------------------------------
docker compose exec -T postgres \
    psql -v ON_ERROR_STOP=1 --username "${DB_USER}" --dbname "${DB_NAME}" <<< "${SQL}"

log_ok "Demo data seeded successfully"

# -----------------------------------------------------------------------------
# Print credentials
# -----------------------------------------------------------------------------
cat <<EOCRED

==============================================
🌱 Demo data seeded (LOCAL)
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
  curl http://localhost:8080/healthz

  curl -X POST http://localhost:8080/v1/auth/login \\
    -H 'Content-Type: application/json' \\
    -d '{
      "tenant_id": "${TENANT_ID}",
      "username": "${USER_ADMIN_ID}",
      "password": "demo123"
    }'

==============================================
EOCRED
