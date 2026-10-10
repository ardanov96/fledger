#!/bin/bash
set -e

# ==============================================================================
# FLEDGER OS — Multi-Database Auto-Initializer
# Creates all dedicated databases for the 6 microservices on first run
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
