#!/bin/bash
set -e

# ==============================================================================
# FLEDGER OS — Cloud Auto-Seeder
# Automatically seeds Chart of Accounts, Vehicles, Drivers, Stores, and Catalog
# ==============================================================================

CORE_URL="http://127.0.0.1:8081"
FLEET_URL="http://127.0.0.1:8082"
PAY_URL="http://127.0.0.1:8083"
FORCE_URL="http://127.0.0.1:8084"
ORDER_URL="http://127.0.0.1:8085"
DUNNING_URL="http://127.0.0.1:8086"
TENANT_ID="a0000000-0000-0000-0000-000000000001"

echo ">>> [Seeder] Waiting for all microservices to be ready..."
for i in $(seq 1 30); do
    if curl -s "${CORE_URL}/healthz" >/dev/null 2>&1 && \
       curl -s "${FLEET_URL}/healthz" >/dev/null 2>&1 && \
       curl -s "${PAY_URL}/healthz" >/dev/null 2>&1 && \
       curl -s "${FORCE_URL}/healthz" >/dev/null 2>&1 && \
       curl -s "${ORDER_URL}/healthz" >/dev/null 2>&1 && \
       curl -s "${DUNNING_URL}/healthz" >/dev/null 2>&1; then
        echo ">>> [Seeder] All microservices are online!"
        break
    fi
    sleep 1
done

# 1. Login dev token from Dunning
TOKEN=$(curl -s -X POST "${DUNNING_URL}/v1/dev/login?tenant_id=${TENANT_ID}" -H "Content-Type: application/json" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4 || true)
AUTH_HEADER=""
if [ -n "$TOKEN" ]; then
    AUTH_HEADER="Authorization: Bearer ${TOKEN}"
fi

echo ">>> [Seeder] Seeding Chart of Accounts..."
accounts=(
    '{"code":"BANK-BCA-OP","name":"Bank BCA Operasional","type":"cash","currency":"IDR"}'
    '{"code":"CASH-HQ-001","name":"Kas Gudang HQ","type":"cash","currency":"IDR"}'
    '{"code":"CASH-REP-001","name":"Kas Salesman Budi","type":"cash","currency":"IDR"}'
    '{"code":"CASH-REP-002","name":"Kas Salesman Andi","type":"cash","currency":"IDR"}'
    '{"code":"AR-CTRL","name":"AR Control (Piutang Toko)","type":"receivable","currency":"IDR"}'
    '{"code":"SALES-REV","name":"Sales Revenue","type":"revenue","currency":"IDR"}'
)
for a in "${accounts[@]}"; do
    curl -s -X POST "${CORE_URL}/v1/accounts" -H "Content-Type: application/json" -H "X-Tenant-ID: ${TENANT_ID}" -H "$AUTH_HEADER" -d "$a" >/dev/null 2>&1 || true
done

echo ">>> [Seeder] Seeding Fleet Drivers & Vehicles..."
curl -s -X POST "${FLEET_URL}/v1/fleet/drivers" -H "Content-Type: application/json" -H "X-Tenant-ID: ${TENANT_ID}" -H "$AUTH_HEADER" \
    -d '{"full_name":"Pak Joko","phone_number":"6281234500001","license_number":"SIM-B2-DRV-JKT-001"}' >/dev/null 2>&1 || true
curl -s -X POST "${FLEET_URL}/v1/fleet/drivers" -H "Content-Type: application/json" -H "X-Tenant-ID: ${TENANT_ID}" -H "$AUTH_HEADER" \
    -d '{"full_name":"Pak Dani","phone_number":"6281234500002","license_number":"SIM-B2-DRV-JKT-002"}' >/dev/null 2>&1 || true

curl -s -X POST "${FLEET_URL}/v1/fleet/vehicles" -H "Content-Type: application/json" -H "X-Tenant-ID: ${TENANT_ID}" -H "$AUTH_HEADER" \
    -d '{"plate_number":"B 9102 FMC","vehicle_type":"CDE_BOX","brand_model":"Truk Engkel Box Mitsubishi Colt Diesel","capacity_kg":1500}' >/dev/null 2>&1 || true
curl -s -X POST "${FLEET_URL}/v1/fleet/vehicles" -H "Content-Type: application/json" -H "X-Tenant-ID: ${TENANT_ID}" -H "$AUTH_HEADER" \
    -d '{"plate_number":"B 9204 FMC","vehicle_type":"BLIND_VAN","brand_model":"Blindvan GranMax","capacity_kg":800}' >/dev/null 2>&1 || true

echo ">>> [Seeder] Seeding Stores (Force SFA)..."
stores=(
    '{"store_code":"CUST-FMCG-01","name":"Toko Berkah Mandiri","owner_name":"Pak Herman","phone":"6281234560001","latitude":-6.200000,"longitude":106.816666,"address":"Jl. Raya Pasar Induk No. 12, Jakarta Barat","geofence_radius_meters":100}'
    '{"store_code":"CUST-FMCG-02","name":"Toko Sumber Rezeki","owner_name":"Bu Ratna","phone":"6281234560002","latitude":-6.175000,"longitude":106.827000,"address":"Jl. Pasar Baru No. 8, Jakarta Pusat","geofence_radius_meters":100}'
    '{"store_code":"CUST-FMCG-03","name":"Toko Sinar Jaya","owner_name":"Pak Bowo","phone":"6281234560003","latitude":-6.261500,"longitude":106.780800,"address":"Jl. Mampang Prapatan No. 33, Jakarta Selatan","geofence_radius_meters":100}'
    '{"store_code":"CUST-FMCG-04","name":"Toko Rezeki Abadi","owner_name":"Bu Yuli","phone":"6281234560004","latitude":-6.175110,"longitude":106.865000,"address":"Jl. Medan Merdeka Timur No. 5, Jakarta Pusat","geofence_radius_meters":100}'
    '{"store_code":"CUST-FMCG-05","name":"Toko Makmur Sentosa","owner_name":"Pak Hendra","phone":"6281234560005","latitude":-6.106200,"longitude":106.884700,"address":"Jl. Raya Kelapa Gading No. 17, Jakarta Utara","geofence_radius_meters":100}'
)
for s in "${stores[@]}"; do
    curl -s -X POST "${FORCE_URL}/v1/force/stores" -H "Content-Type: application/json" -H "X-Tenant-ID: ${TENANT_ID}" -H "$AUTH_HEADER" -d "$s" >/dev/null 2>&1 || true
done

echo ">>> [Seeder] Seeding Order FMCG Catalog..."
products=(
    '{"sku":"FMCG-OIL-001","name":"Minyak Goreng Sawit 2L","category":"Sembako","uom":"Karton (6 Pch)","base_price":96000,"current_stock":500,"reserved_stock":0}'
    '{"sku":"FMCG-MIE-002","name":"Mie Instan Goreng Spesial (Dus)","category":"Makanan","uom":"Dus (40 Bks)","base_price":118000,"current_stock":1200,"reserved_stock":0}'
    '{"sku":"FMCG-BEV-003","name":"Teh Melati Kemasan Botol 350ml","category":"Minuman","uom":"Karton (24 Btl)","base_price":62000,"current_stock":800,"reserved_stock":0}'
    '{"sku":"FMCG-SNK-004","name":"Biskuit Gandum Cokelat 150g","category":"Snack","uom":"Karton (24 Bks)","base_price":144000,"current_stock":350,"reserved_stock":0}'
    '{"sku":"FMCG-SOP-005","name":"Sabun Cuci Piring Jeruk Nipis 800ml","category":"Kebersihan","uom":"Karton (12 Pch)","base_price":132000,"current_stock":450,"reserved_stock":0}'
)
for p in "${products[@]}"; do
    curl -s -X POST "${ORDER_URL}/v1/order/products" -H "Content-Type: application/json" -H "X-Tenant-ID: ${TENANT_ID}" -H "$AUTH_HEADER" -d "$p" >/dev/null 2>&1 || true
done

echo ">>> [Seeder] Demo Ecosystem is fully populated and ready for pitching!"
