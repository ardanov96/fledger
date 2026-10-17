#!/bin/bash
set -e

echo "================================================================="
echo "  FLEDGER OS — ALL-IN-ONE ENTERPRISE CLOUD CONTAINER             "
echo "  Ready for 24/7 Render / Koyeb / Hugging Face Deployment        "
echo "================================================================="

# -----------------------------------------------------------------------------
# 1. Start Embedded PostgreSQL
# -----------------------------------------------------------------------------
mkdir -p /run/postgresql /var/lib/postgresql/data /var/log/postgresql
chown -R postgres:postgres /run/postgresql /var/lib/postgresql /var/log/postgresql

if [ ! -f /var/lib/postgresql/data/PG_VERSION ]; then
    echo ">>> [1/5] Initializing local PostgreSQL 16 Cluster..."
    su - postgres -c "initdb -D /var/lib/postgresql/data --auth-local=trust --auth-host=trust"
    su - postgres -c "pg_ctl -D /var/lib/postgresql/data -l /var/log/postgresql/pg.log start"
    
    echo ">>> [1/5] Creating dedicated microservice databases & user..."
    su - postgres -c "psql -c \"CREATE USER fmcg WITH SUPERUSER PASSWORD 'fmcg_dev_password';\""
    for db in fmcg_wallet fledger_fleet fledger_pay fledger_force fledger_order fledger_dunning; do
        su - postgres -c "psql -c \"CREATE DATABASE $db OWNER fmcg;\""
    done

    echo ">>> [1/5] Running schema migrations..."
    # 1. Core migrations
    if [ -d "/app/migrations/core" ]; then
        for f in $(ls /app/migrations/core/*up.sql 2>/dev/null | sort); do
            echo "  -> Core migration: $f"
            su - postgres -c "psql -U fmcg -d fmcg_wallet -f '$f'" >/dev/null 2>&1 || true
        done
    fi
    # 2. Fleet migrations
    if [ -d "/app/migrations/fleet" ]; then
        for f in $(ls /app/migrations/fleet/*.sql 2>/dev/null | sort); do
            echo "  -> Fleet migration: $f"
            su - postgres -c "psql -U fmcg -d fledger_fleet -f '$f'" >/dev/null 2>&1 || true
        done
    fi
    # 3. Pay migrations
    if [ -d "/app/migrations/pay" ]; then
        for f in $(ls /app/migrations/pay/*.sql 2>/dev/null | sort); do
            echo "  -> Pay migration: $f"
            su - postgres -c "psql -U fmcg -d fledger_pay -f '$f'" >/dev/null 2>&1 || true
        done
    fi
    # 4. Force migrations
    if [ -d "/app/migrations/force" ]; then
        for f in $(ls /app/migrations/force/*.sql 2>/dev/null | sort); do
            echo "  -> Force migration: $f"
            su - postgres -c "psql -U fmcg -d fledger_force -f '$f'" >/dev/null 2>&1 || true
        done
    fi
    # 5. Order migrations
    if [ -d "/app/migrations/order" ]; then
        for f in $(ls /app/migrations/order/*.sql 2>/dev/null | sort); do
            echo "  -> Order migration: $f"
            su - postgres -c "psql -U fmcg -d fledger_order -f '$f'" >/dev/null 2>&1 || true
        done
    fi
    # 6. Dunning migrations
    if [ -d "/app/migrations/dunning" ]; then
        for f in $(ls /app/migrations/dunning/*.sql 2>/dev/null | sort); do
            echo "  -> Dunning migration: $f"
            su - postgres -c "psql -U fmcg -d fledger_dunning -f '$f'" >/dev/null 2>&1 || true
        done
    fi
else
    echo ">>> [1/5] Starting existing PostgreSQL..."
    su - postgres -c "pg_ctl -D /var/lib/postgresql/data -l /var/log/postgresql/pg.log start"
fi

DB_BASE="postgres://fmcg:fmcg_dev_password@127.0.0.1:5432"

# -----------------------------------------------------------------------------
# 2. Start Redis & NATS JetStream
# -----------------------------------------------------------------------------
echo ">>> [2/5] Starting Redis 7..."
redis-server --daemonize yes --port 6379 --bind 127.0.0.1

echo ">>> [2/5] Starting NATS JetStream..."
mkdir -p /data/nats
nats-server -js -sd /data/nats -p 4222 >/dev/null 2>&1 &

sleep 2

# -----------------------------------------------------------------------------
# 3. Start 6 Go Microservices (Loopback 127.0.0.1)
# -----------------------------------------------------------------------------
echo ">>> [3/5] Starting 6 Microservices..."
export JWT_SECRET="fledger-super-secure-jwt-secret-key-2026-min-32"
export FLEDGER_TENANT_ID="a0000000-0000-0000-0000-000000000001"

# 1. Core (:8081)
(
    export PORT=8081
    export DATABASE_URL="${DB_BASE}/fmcg_wallet?sslmode=disable"
    export REDIS_URL="redis://127.0.0.1:6379/0"
    export NATS_URL="nats://127.0.0.1:4222"
    export RBAC_POLICY_DIR="/app/fledger-core/internal/auth/rbac/policies"
    export WEB_DIR="/app/fledger-core/web/public"
    exec /app/bin/core
) >/dev/null 2>&1 &

# 2. Fleet (:8082)
(
    export PORT=8082
    export DATABASE_URL="${DB_BASE}/fledger_fleet?sslmode=disable"
    export FLEDGER_CORE_URL="http://127.0.0.1:8081"
    export WEB_DIR="/app/fledger-fleet/web"
    exec /app/bin/fleet
) >/dev/null 2>&1 &

# 3. Pay (:8083)
(
    export PORT=8083
    export DATABASE_URL="${DB_BASE}/fledger_pay?sslmode=disable"
    export FLEDGER_CORE_URL="http://127.0.0.1:8081"
    export FLEDGER_DUNNING_URL="http://127.0.0.1:8086"
    export WEBHOOK_SECRET="SB-Mid-server-XJsYUlLDpouyLjW5E6NxELyI"
    export MIDTRANS_SERVER_KEY="SB-Mid-server-XJsYUlLDpouyLjW5E6NxELyI"
    export MIDTRANS_CLIENT_KEY="SB-Mid-client-5c-O12snGFaCx2fj"
    export MIDTRANS_MERCHANT_ID="G426768001"
    export WEB_DIR="/app/fledger-pay/web"
    exec /app/bin/pay
) >/dev/null 2>&1 &

# 4. Force (:8084)
(
    export PORT=8084
    export DATABASE_URL="${DB_BASE}/fledger_force?sslmode=disable"
    export FLEDGER_CORE_URL="http://127.0.0.1:8081"
    export WEB_DIR="/app/fledger-force/web"
    exec /app/bin/force
) >/dev/null 2>&1 &

# 5. Order (:8085)
(
    export PORT=8085
    export DATABASE_URL="${DB_BASE}/fledger_order?sslmode=disable"
    export FLEDGER_CORE_URL="http://127.0.0.1:8081"
    export FLEDGER_FLEET_URL="http://127.0.0.1:8082"
    export MANAGER_OVERRIDE_PIN="123456"
    export WEB_DIR="/app/fledger-order/web"
    exec /app/bin/order
) >/dev/null 2>&1 &

# 6. Dunning (:8086)
(
    export PORT=8086
    export DATABASE_URL="${DB_BASE}/fledger_dunning?sslmode=disable"
    export FLEDGER_CORE_URL="http://127.0.0.1:8081"
    export FLEDGER_PAY_URL="http://127.0.0.1:8083"
    export PAY_WEBHOOK_SECRET="dunning-super-secret-key-2026"
    export WA_PROVIDER="FONNTE"
    export FONNTE_TOKEN="EM8i5t8sWe11CpqBhLTx"
    export FONNTE_BASE_URL="https://api.fonnte.com"
    export WA_JITTER_MIN_SECONDS="0"
    export WA_JITTER_MAX_SECONDS="1"
    exec /app/bin/dunning
) >/dev/null 2>&1 &

# -----------------------------------------------------------------------------
# 4. Auto-Seed Initial Data (Background)
# -----------------------------------------------------------------------------
echo ">>> [4/5] Scheduling Ecosystem Auto-Seeder..."
/app/seed.sh >/dev/null 2>&1 &

# -----------------------------------------------------------------------------
# 5. Start Unified API Gateway on $PORT (Foreground)
# -----------------------------------------------------------------------------
LISTEN_PORT="${PORT:-10000}"
export PORT="$LISTEN_PORT"
echo "================================================================="
echo ">>> [5/5] FLEDGER OS Gateway is ONLINE at port ${LISTEN_PORT}"
echo "    Landing Page   : http://0.0.0.0:${LISTEN_PORT}/"
echo "    Live Ops Hub   : http://0.0.0.0:${LISTEN_PORT}/hub"
echo "    Simulator      : http://0.0.0.0:${LISTEN_PORT}/simulator"
echo "================================================================="

cd /app/deployments/gateway
exec /app/bin/gateway
