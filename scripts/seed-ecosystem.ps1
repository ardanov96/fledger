# ==============================================================================
# FLEDGER OS - FMCG Ecosystem Master Seeder
# Seeds 1 tenant, chart-of-accounts (Core), 2 trucks + 2 drivers (Fleet),
# 5 stores (Force/Order), FMCG catalog (Order), and 2 dunning invoices
# (Dunning) so demos walk through the full lifecycle in one shot.
# ==============================================================================
[CmdletBinding()]
param(
    [string]$CoreUrl    = "http://localhost:8081",
    [string]$FleetUrl   = "http://localhost:8082",
    [string]$PayUrl     = "http://localhost:8083",
    [string]$ForceUrl   = "http://localhost:8084",
    [string]$OrderUrl   = "http://localhost:8085",
    [string]$DunningUrl = "http://localhost:8086",
    [string]$TenantId   = "a0000000-0000-0000-0000-000000000001"
)

$ErrorActionPreference = "Stop"

function Step($msg) { Write-Host ""; Write-Host "==> $msg" -ForegroundColor Cyan }
function Ok($msg)   { Write-Host "  [OK]   $msg" -ForegroundColor Green }
function Warn($msg) { Write-Host "  [WARN] $msg" -ForegroundColor DarkYellow }
function Fail($msg){ Write-Host "  [FAIL] $msg" -ForegroundColor Red }

$script:fails = 0

# ----------------------------------------------------------------------------
# 1) Login to obtain a service token
# ----------------------------------------------------------------------------
Step "1/6 Login to obtain a service JWT"
try {
    $login = Invoke-RestMethod -Method POST -Uri "$DunningUrl/v1/dev/login?tenant_id=$TenantId" -Headers @{"Content-Type"="application/json"}
    $token = $login.access_token
    if ($token) { Ok "got JWT" } else { Fail "no token"; return }
} catch {
    Warn "dev login unavailable; will skip auth (tests will run as anonymous)"
    $token = ""
}
$authHeaders = @{
    "Content-Type" = "application/json"
    "X-Tenant-ID"  = $TenantId
}
if ($token) { $authHeaders["Authorization"] = "Bearer $token" }

# ----------------------------------------------------------------------------
# 2) Fledger Core - Chart of Accounts
# ----------------------------------------------------------------------------
Step "2/6 Seeding Fledger Core chart of accounts"
$coreAccounts = @(
    @{ code = "BANK-BCA-OP";    name = "Bank BCA Operasional";     type = "cash";       currency = "IDR" },
    @{ code = "CASH-HQ-001";  name = "Kas Gudang HQ";            type = "cash";       currency = "IDR" },
    @{ code = "CASH-REP-001"; name = "Kas Salesman Budi";        type = "cash";       currency = "IDR" },
    @{ code = "CASH-REP-002"; name = "Kas Salesman Andi";        type = "cash";       currency = "IDR" },
    @{ code = "AR-CTRL";      name = "AR Control (Piutang Toko)"; type = "receivable"; currency = "IDR" },
    @{ code = "SALES-REV";    name = "Sales Revenue";            type = "revenue";    currency = "IDR" }
)
foreach ($a in $coreAccounts) {
    try {
        $body = @{
            code     = $a.code
            name     = $a.name
            type     = $a.type
            currency = $a.currency
        } | ConvertTo-Json -Depth 4 -Compress
        $resp = Invoke-RestMethod -Method POST -Uri "$CoreUrl/v1/accounts" -Headers $authHeaders -Body $body
        Ok "account $($a.code) created"
    } catch {
        if ($_.Exception.Message -match "409" -or ($_.Exception.Response -and [int]$_.Exception.Response.StatusCode -eq 409)) {
            Ok "account $($a.code) already active"
        } else {
            Warn "$($a.code) skipped: $($_.Exception.Message)"
        }
    }
}

# ----------------------------------------------------------------------------
# 3) Fledger Fleet - 2 trucks + 2 drivers
# ----------------------------------------------------------------------------
Step "3/6 Seeding Fleet (2 trucks, 2 drivers)"
$fleetSeed = @(
    @{ plate = "B 9102 FMC"; model = "Truk Engkel Box Mitsubishi Colt Diesel"; capacity = 1500; driverName = "Pak Joko"; driverPhone = "6281234500001"; driverCode = "DRV-JKT-001"; type = "CDE_BOX" },
    @{ plate = "B 9204 FMC"; model = "Blindvan GranMax";                  capacity = 800;  driverName = "Pak Dani"; driverPhone = "6281234500002"; driverCode = "DRV-JKT-002"; type = "BLIND_VAN" }
)
foreach ($v in $fleetSeed) {
    try {
        $driverBody = @{
            full_name      = $v.driverName
            phone_number   = $v.driverPhone
            license_number = "SIM-B2-$($v.driverCode)"
        } | ConvertTo-Json -Depth 4 -Compress
        try { Invoke-RestMethod -Method POST -Uri "$FleetUrl/v1/fleet/drivers" -Headers $authHeaders -Body $driverBody | Out-Null } catch {}

        $vehicleBody = @{
            plate_number = $v.plate
            vehicle_type = $v.type
            brand_model  = $v.model
            capacity_kg  = [double]$v.capacity
        } | ConvertTo-Json -Depth 4 -Compress
        try { Invoke-RestMethod -Method POST -Uri "$FleetUrl/v1/fleet/vehicles" -Headers $authHeaders -Body $vehicleBody | Out-Null } catch {}
        Ok "truck $($v.plate) + driver $($v.driverName) ready"
    } catch {
        Warn "$($v.plate) skipped: $($_.Exception.Message)"
    }
}

# ----------------------------------------------------------------------------
# 4) Fledger Force — 5 stores (Jakarta)
# ----------------------------------------------------------------------------
Step "4/6 Seeding Force (5 stores in Jakarta)"
$forceStores = @(
    @{ id = "CUST-FMCG-01"; name = "Toko Berkah Mandiri";  owner = "Pak Herman";  phone = "6281234560001"; lat = -6.200000; lng = 106.816666; address = "Jl. Raya Pasar Induk No. 12, Jakarta Barat" },
    @{ id = "CUST-FMCG-02"; name = "Toko Sumber Rezeki";   owner = "Bu Ratna";   phone = "6281234560002"; lat = -6.175000; lng = 106.827000; address = "Jl. Pasar Baru No. 8, Jakarta Pusat" },
    @{ id = "CUST-FMCG-03"; name = "Toko Sinar Jaya";       owner = "Pak Bowo";    phone = "6281234560003"; lat = -6.261500; lng = 106.780800; address = "Jl. Mampang Prapatan No. 33, Jakarta Selatan" },
    @{ id = "CUST-FMCG-04"; name = "Toko Rezeki Abadi";    owner = "Bu Yuli";    phone = "6281234560004"; lat = -6.175110; lng = 106.865000; address = "Jl. Medan Merdeka Timur No. 5, Jakarta Pusat" },
    @{ id = "CUST-FMCG-05"; name = "Toko Makmur Sentosa";  owner = "Pak Hendra";  phone = "6281234560005"; lat = -6.106200; lng = 106.884700; address = "Jl. Raya Kelapa Gading No. 17, Jakarta Utara" }
)
foreach ($s in $forceStores) {
    try {
        $body = @{
            store_code            = $s.id
            name                  = $s.name
            owner_name            = $s.owner
            phone                 = $s.phone
            latitude              = $s.lat
            longitude             = $s.lng
            address               = $s.address
            geofence_radius_meters = 100
        } | ConvertTo-Json -Depth 5 -Compress
        Invoke-RestMethod -Method POST -Uri "$ForceUrl/v1/force/stores" -Headers $authHeaders -Body $body | Out-Null
        Ok "store $($s.name) registered"
    } catch {
        if ($_.Exception.Message -match "409") {
            Ok "store $($s.name) already registered"
        } else {
            Warn "$($s.name) skipped: $($_.Exception.Message)"
        }
    }
}

# ----------------------------------------------------------------------------
# 5) Fledger Order — FMCG catalog
# ----------------------------------------------------------------------------
Step "5/6 Seeding Order catalog (4 popular FMCG SKUs)"
$orderProducts = @(
    @{ sku = "SKU-INDOMIE-GRG"; name = "Indomie Goreng Rasa Ayam Panggang"; category = "MIE_INSTAN"; unit = "DUS"; weight_grams = 8000; tier = "GROSIR"; min_qty = 1; price = 115000 },
    @{ sku = "SKU-MINYAK-2L";   name = "Minyak Goreng Sawit 2L";              category = "SEMBAKO";    unit = "PACK"; weight_grams = 2000; tier = "GROSIR"; min_qty = 1; price = 34000 },
    @{ sku = "SKU-AQUA-600ML";   name = "Air Mineral Aqua 600ml x 24";         category = "MINUMAN";    unit = "KARTON"; weight_grams = 14000; tier = "GROSIR"; min_qty = 1; price = 48000 },
    @{ sku = "SKU-KOPI-KAPALAPI"; name = "Kopi Bubuk Special 165g";            category = "MINUMAN";    unit = "PACK";  weight_grams = 800;  tier = "GROSIR"; min_qty = 1; price = 16500 }
)
foreach ($p in $orderProducts) {
    try {
        $prodBody = @{
            sku          = $p.sku
            name         = $p.name
            category     = $p.category
            unit         = $p.unit
            weight_grams = $p.weight_grams
        } | ConvertTo-Json -Depth 4 -Compress
        $prodId = $null
        try {
            $created = Invoke-RestMethod -Method POST -Uri "$OrderUrl/v1/order/products" -Headers $authHeaders -Body $prodBody
            $prodId = if ($created.data -and $created.data.id) { $created.data.id } else { $created.id }
        } catch {
            $existing = Invoke-RestMethod -Method GET -Uri "$OrderUrl/v1/order/products" -Headers $authHeaders
            $matched = $existing | Where-Object { $_.sku -eq $p.sku }
            if ($matched) { $prodId = $matched.id }
        }

        if ($prodId) {
            $priceBody = @{
                tier         = $p.tier
                min_quantity = $p.min_qty
                unit_price   = $p.price
            } | ConvertTo-Json -Depth 4 -Compress
            try { Invoke-RestMethod -Method POST -Uri "$OrderUrl/v1/order/products/$prodId/pricing" -Headers $authHeaders -Body $priceBody | Out-Null } catch {}

            $stockBody = @{
                product_id   = $prodId
                warehouse_id = "WH-CENTRAL-01"
                delta        = 1000
            } | ConvertTo-Json -Compress
            try { Invoke-RestMethod -Method POST -Uri "$OrderUrl/v1/order/inventory/adjust" -Headers $authHeaders -Body $stockBody | Out-Null } catch {}

            Ok "product $($p.sku) ready (stock 1000)"
        }
    } catch {
        Warn "$($p.sku) skipped: $($_.Exception.Message)"
    }
}

# ----------------------------------------------------------------------------
# 6) Fledger Dunning - 2 invoices in different lifecycle states
# ----------------------------------------------------------------------------
Step "6/6 Seeding Dunning (1 H-3 invoice + 1 H+7 overdue invoice)"

# 6a) Register store contacts in Dunning so the WA gateway can address them.
$dunningStores = @(
    @{ id = "CUST-FMCG-01"; name = "Toko Berkah Mandiri"; owner = "Pak Herman"; phone = "6281234560001" },
    @{ id = "CUST-FMCG-04"; name = "Toko Rezeki Abadi";    owner = "Bu Yuli";    phone = "6281234560004" }
)
foreach ($c in $dunningStores) {
    try {
        $body = @{
            store_id   = $c.id
            store_name = $c.name
            owner_name = $c.owner
            phone_number = $c.phone
        } | ConvertTo-Json -Depth 4 -Compress
        Invoke-RestMethod -Method POST -Uri "$DunningUrl/v1/dunning/contacts" -Headers $authHeaders -Body $body | Out-Null
        Ok "dunning contact $($c.id) registered"
    } catch {
        Warn "dunning contact $($c.id) skipped: $($_.Exception.Message)"
    }
}

# 6b) Invoice 1 - H-3 (due in 3 days; will trigger PRE_DUE_H3 reminder today).
$invoice1Due = (Get-Date).AddDays(3).ToString("yyyy-MM-dd")
$invoice1Body = @{
    invoice_id       = "INV-FMCG-H3-001"
    invoice_number   = "INV/FMCG/H3/001"
    store_id         = "CUST-FMCG-01"
    phone_number     = "6281234560001"
    due_date         = $invoice1Due
    amount_due_minor = 1250000
    payment_link_url = "$PayUrl/pay/INV-FMCG-H3-001"
    store_name       = "Toko Berkah Mandiri"
} | ConvertTo-Json -Depth 5 -Compress
try {
    $resp = Invoke-RestMethod -Method POST -Uri "$DunningUrl/v1/dunning/queues/ingest-invoice" -Headers $authHeaders -Body $invoice1Body
    Ok "invoice INV-FMCG-H3-001 ingested (due $invoice1Due) - 5 stages scheduled"
} catch {
    Warn "invoice H-3 skipped: $($_.Exception.Message)"
}

# 6c) Invoice 2 - H+7 (overdue 7 days; will trigger OVERDUE_H7 reminder).
$invoice2Due = (Get-Date).AddDays(-7).ToString("yyyy-MM-dd")
$invoice2Body = @{
    invoice_id       = "INV-FMCG-OD7-001"
    invoice_number   = "INV/FMCG/OD7/001"
    store_id         = "CUST-FMCG-04"
    phone_number     = "6281234560004"
    due_date         = $invoice2Due
    amount_due_minor = 750000
    payment_link_url = "$PayUrl/pay/INV-FMCG-OD7-001"
    store_name       = "Toko Rezeki Abadi"
} | ConvertTo-Json -Depth 5 -Compress
try {
    $resp = Invoke-RestMethod -Method POST -Uri "$DunningUrl/v1/dunning/queues/ingest-invoice" -Headers $authHeaders -Body $invoice2Body
    Ok "invoice INV-FMCG-OD7-001 ingested (due $invoice2Due was 7 days ago) - 5 stages scheduled"
} catch {
    Warn "invoice OD7 skipped: $($_.Exception.Message)"
}

Write-Host ""
Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "  FMCG Ecosystem Seeder Complete!                                    " -ForegroundColor Green
Write-Host "  Login at: $PayUrl/web (use dev login to get a token)" -ForegroundColor Cyan
Write-Host "  View dunning queues: GET $DunningUrl/v1/dunning/queues" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan