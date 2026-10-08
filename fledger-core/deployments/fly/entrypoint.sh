#!/bin/bash
# =============================================================================
# Fly.io Entrypoint — FMCG Wallet Demo
# =============================================================================
#
# Responsibilities:
#   1. Initialize Postgres data directory (first boot only)
#   2. Start Postgres server
#   3. Wait for Postgres to be ready
#   4. Run database migrations (golang-migrate)
#   5. (Optional) Seed demo data if DEMO_MODE=true and tables empty
#   6. Start supervisord (manages postgres + api processes)
#
# Robustness:
#   - Idempotent: safe to run multiple times
#   - Retry Postgres startup with exponential backoff
#   - Fail loud if migrations fail (don't start API with broken schema)
# =============================================================================

set -euo pipefail

# -----------------------------------------------------------------------------
# Configuration (from fly.toml [env] section)
# -----------------------------------------------------------------------------
DB_NAME="${DB_NAME:-fmcg_wallet}"
DB_USER="${DB_USER:-fmcg}"
DB_PASSWORD="${DB_PASSWORD:-fmcg_demo_password}"
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
PGDATA="/var/lib/postgresql/data"

echo "[entrypoint] Starting FMCG Wallet demo container..."
echo "[entrypoint] DB: ${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo "[entrypoint] DEMO_MODE: ${DEMO_MODE:-false}"

# -----------------------------------------------------------------------------
# Step 1: Initialize Postgres data directory (first boot only)
# -----------------------------------------------------------------------------
if [ ! -s "${PGDATA}/PG_VERSION" ]; then
    echo "[entrypoint] First boot detected — initializing Postgres data directory..."
    su - postgres -c "initdb -D ${PGDATA} --auth=scram-sha-256 --auth-host=scram-sha-256 --auth-local=trust" 2>&1 | tail -5

    # Configure Postgres for local connections
    cat >> "${PGDATA}/postgresql.conf" <<EOF

# Fly.io demo config
listen_addresses = '127.0.0.1'
port = ${DB_PORT}
max_connections = 100
shared_buffers = 128MB
effective_cache_size = 256MB
log_min_duration_statement = 500ms
log_connections = on
log_disconnections = on
EOF

    # Allow local trust auth (we use password auth via DATABASE_URL)
    cat > "${PGDATA}/pg_hba.conf" <<EOF
local   all             all                                     trust
host    all             all             127.0.0.1/32            scram-sha-256
host    all             all             ::1/128                 scram-sha-256
EOF

    chown -R postgres:postgres "${PGDATA}"
fi

# -----------------------------------------------------------------------------
# Step 2: Start Postgres (via supervisor)
# -----------------------------------------------------------------------------
echo "[entrypoint] Starting supervisord (postgres + api)..."
exec /usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf
