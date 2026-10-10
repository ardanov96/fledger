#!/bin/bash
set -e

# ==============================================================================
# FLEDGER OS — Multi-Database + Schema Auto-Bootstrap
# Creates all dedicated databases and applies the per-service migrations
# on first run (Postgres /docker-entrypoint-initdb.d/).
# ==============================================================================

DATABASES=(
    "fmcg_wallet"
    "fledger_fleet"
    "fledger_pay"
    "fledger_force"
    "fledger_order"
    "fledger_dunning"
    "fledger_dunning_test"
)

echo ">>> [FLEDGER OS] Initializing dedicated microservice databases..."

for db in "${DATABASES[@]}"; do
    echo ">>> Checking database: $db"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
        SELECT 'CREATE DATABASE $db'
        WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$db')\gexec
        GRANT ALL PRIVILEGES ON DATABASE $db TO $POSTGRES_USER;
EOSQL
done

echo ">>> [FLEDGER OS] All microservice databases are ready!"
echo ""
echo ">>> [FLEDGER OS] Applying initial database migrations..."

# 1. Fledger Core — every up.sql file in numeric order.
if compgen -G "/docker-entrypoint-initdb.d/schemas/core/*up.sql" > /dev/null; then
    for f in $(ls /docker-entrypoint-initdb.d/schemas/core/*up.sql | sort); do
        echo "  -> Core migration: $f"
        psql -v ON_ERROR_STOP=0 --username "$POSTGRES_USER" --dbname "fmcg_wallet" -f "$f" || true
    done
fi

# 2. Fledger Fleet
for f in $(ls /docker-entrypoint-initdb.d/schemas/fleet/*.sql 2>/dev/null | sort); do
    echo "  -> Fleet migration: $f"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "fledger_fleet" -f "$f"
done

# 3. Fledger Pay
for f in $(ls /docker-entrypoint-initdb.d/schemas/pay/*.sql 2>/dev/null | sort); do
    echo "  -> Pay migration: $f"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "fledger_pay" -f "$f"
done

# 4. Fledger Force
for f in $(ls /docker-entrypoint-initdb.d/schemas/force/*.sql 2>/dev/null | sort); do
    echo "  -> Force migration: $f"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "fledger_force" -f "$f"
done

# 5. Fledger Order
for f in $(ls /docker-entrypoint-initdb.d/schemas/order/*.sql 2>/dev/null | sort); do
    echo "  -> Order migration: $f"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "fledger_order" -f "$f"
done

# 6. Fledger Dunning
if compgen -G "/docker-entrypoint-initdb.d/schemas/dunning/*.sql" > /dev/null; then
    for f in $(ls /docker-entrypoint-initdb.d/schemas/dunning/*.sql | sort); do
        echo "  -> Dunning migration: $f"
        psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "fledger_dunning" -f "$f"
    done
fi

echo ""
echo ">>> [FLEDGER OS] All microservice databases and schemas are initialized successfully!"