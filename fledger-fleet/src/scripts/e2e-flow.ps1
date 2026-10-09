# =============================================================================
# FLEDGER FLEET — End-to-end smoke test (PowerShell)
# Runs the full Sprint-5 happy path against a live Fledger Fleet API.
# Exits 0 on success, non-zero on any failure.
# =============================================================================

$ErrorActionPreference = "Stop"
$BaseUrl = if ($env:FLEDGER_FLEET_URL) { $env:FLEDGER_FLEET_URL } else { "http://localhost:8082" }
$runTag = [DateTime]::UtcNow.ToString("HHmmssfff")

# Use a here-string so PowerShell never tries to interpret "+" in base64.
$signatureSample = @'
data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=
'@

function Step($msg) { Write-Host ""; Write-Host "==> $msg" }
function OkMsg($msg)  { Write-Host "  [OK]   $msg" }
function FailMsg($msg){ Write-Host "  [FAIL] $msg"; $script:exitcode = 1 }

$script:exitcode = 0
$jsonHdr = @{ "Content-Type" = "application/json" }

# 1. Login
Step "Login (dev endpoint)"
$login = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/dev/login" -Headers $jsonHdr
$token = $login.access_token
if (-not $token) { FailMsg "no token"; exit 1 }
OkMsg "got JWT"
$authHdr = @{ Authorization = "Bearer $token" }

# 2. List vehicles + drivers
Step "Master data"
$vehicles = Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/fleet/vehicles" -Headers $authHdr
$drivers = Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/fleet/drivers" -Headers $authHdr
if ($vehicles.Count -lt 2) { FailMsg "need >=2 vehicles"; exit 1 }
if ($drivers.Count -lt 2)  { FailMsg "need >=2 drivers"; exit 1 }
OkMsg "vehicles=$($vehicles.Count) drivers=$($drivers.Count)"
$vehicleId = $vehicles[0].id
$driverId = $drivers[0].id

# 3. Create DO #1 (Minyak Goreng)
Step "Create DO #1 - Minyak Goreng"
$do1Body = @{
    do_number           = "DO-DEMO-$runTag-001"
    customer_id         = "cccccccc-0001-0000-0000-000000000001"
    customer_name       = "Toko Sumber Rezeki"
    destination_address = "Jl. Raya Daan Mogot 45"
    destination_lat     = -6.152345
    destination_lng     = 106.758912
    items = @(
        @{ product_sku = "SKU-OIL-01"; product_name = "Minyak Goreng 2L Pouch"; qty_ordered = 10; unit_price_cents = 500000 }
    )
}
$do1 = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/fleet/delivery-orders" -Headers ($authHdr + $jsonHdr) -Body ($do1Body | ConvertTo-Json -Depth 6 -Compress)
Write-Host "  DO1 raw id='$($do1.id)' nominal=$($do1.nominal_ordered_cents)"
if ($do1.nominal_ordered_cents -ne 5000000) { FailMsg "DO1 nominal wrong: $($do1.nominal_ordered_cents)" } else { OkMsg "DO1 nominal=5,000,000" }
$do1Id = $do1.id

# 4. Create DO #2 (Mie Instan)
Step "Create DO #2 - Mie Instan"
$do2Body = @{
    do_number           = "DO-DEMO-$runTag-002"
    customer_id         = "cccccccc-0002-0000-0000-000000000002"
    customer_name       = "Warung Berkah Jaya"
    destination_address = "Jl. Merdeka 12"
    items = @(
        @{ product_sku = "SKU-MIE-01"; product_name = "Mie Instan Goreng"; qty_ordered = 5; unit_price_cents = 120000 }
    )
}
$do2 = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/fleet/delivery-orders" -Headers ($authHdr + $jsonHdr) -Body ($do2Body | ConvertTo-Json -Depth 6 -Compress)
if ($do2.nominal_ordered_cents -ne 600000) { FailMsg "DO2 nominal wrong: $($do2.nominal_ordered_cents)" } else { OkMsg "DO2 nominal=600,000" }
$do2Id = $do2.id

# 5. Create trip
Step "Create trip linking DO1 + DO2"
$tripBody = @{
    trip_number = "TRIP-DEMO-$runTag"
    vehicle_id  = $vehicleId
    driver_id   = $driverId
    do_ids      = @($do1Id, $do2Id)
    notes       = "E2E smoke test"
}
$tripJson = $tripBody | ConvertTo-Json -Depth 6 -Compress
Write-Host "  trip request: $tripJson"
$trip = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/fleet/trips" -Headers ($authHdr + $jsonHdr) -Body $tripJson
if ($trip.status -ne "DRAFT") { FailMsg "trip not DRAFT: $($trip.status)" } else { OkMsg "trip in DRAFT, stops=$($trip.total_stops)" }
$tripId = $trip.id

# 6. Dispatch
Step "Dispatch trip"
$dispatched = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/fleet/trips/$tripId/dispatch" -Headers $authHdr
if ($dispatched.status -ne "IN_TRANSIT") { FailMsg "dispatch failed: $($dispatched.status)" } else { OkMsg "trip IN_TRANSIT" }

# 7. POD DO1 (partial: 8/10 minyak, 2 bocor)
Step "Submit POD partial - DO1"
$pod1Body = @{
    recipient_name      = "Ibu Siti Fatimah"
    signature_data_url  = $signatureSample
    photo_evidence_urls = @("https://storage.example/pod-do1-damage.jpg")
    delivered_lat       = -6.152350
    delivered_lng       = 106.758920
    driver_notes        = "2 karton bocor di jalan"
    items = @(
        @{ product_sku = "SKU-OIL-01"; qty_delivered = 8; qty_rejected = 2; rejection_reason = "DAMAGED_LEAK" }
    )
}
$pod1 = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/fleet/delivery-orders/$do1Id/pod" -Headers ($authHdr + $jsonHdr) -Body ($pod1Body | ConvertTo-Json -Depth 6 -Compress)
if ($pod1.status -ne "DELIVERED_PARTIAL") { FailMsg "DO1 POD status wrong: $($pod1.status)" } else { OkMsg "DO1 status=DELIVERED_PARTIAL" }
if ($pod1.nominal_delivered_cents -ne 4000000) { FailMsg "DO1 nominal wrong: $($pod1.nominal_delivered_cents)" } else { OkMsg "DO1 nominal=4,000,000" }

# 8. POD DO2 (full: 5/5 mie)
Step "Submit POD full - DO2"
$pod2Body = @{
    recipient_name     = "Pak Darmawan"
    signature_data_url = $signatureSample
    delivered_lat      = -6.15
    delivered_lng      = 106.75
    items = @(
        @{ product_sku = "SKU-MIE-01"; qty_delivered = 5; qty_rejected = 0 }
    )
}
$pod2 = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/fleet/delivery-orders/$do2Id/pod" -Headers ($authHdr + $jsonHdr) -Body ($pod2Body | ConvertTo-Json -Depth 6 -Compress)
if ($pod2.status -ne "DELIVERED_FULL") { FailMsg "DO2 POD status wrong: $($pod2.status)" } else { OkMsg "DO2 status=DELIVERED_FULL" }
if ($pod2.nominal_delivered_cents -ne 600000) { FailMsg "DO2 nominal wrong: $($pod2.nominal_delivered_cents)" } else { OkMsg "DO2 nominal=600,000" }

# 9. Negative test: POD without photo for rejected items
Step "Negative: POD without photo evidence for rejected items"
try {
    $badBody = @{
        recipient_name     = "X"
        signature_data_url = $signatureSample
        delivered_lat      = -6.1
        delivered_lng      = 106.7
        items = @(
            @{ product_sku = "SKU-OIL-01"; qty_delivered = 8; qty_rejected = 2; rejection_reason = "DAMAGED_LEAK" }
        )
    }
    Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/fleet/delivery-orders/$do1Id/pod" -Headers ($authHdr + $jsonHdr) -Body ($badBody | ConvertTo-Json -Depth 6 -Compress) | Out-Null
    FailMsg "expected 400 for missing photo, got 200"
} catch {
    if ($_.Exception.Response.StatusCode -eq 400) { OkMsg "correctly rejected with 400" } else { FailMsg "wrong status: $($_.Exception.Response.StatusCode)" }
}

# 10. Outbox counts
Step "Outbox counts"
$counts = Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/fleet/outbox/counts" -Headers $authHdr
OkMsg "pending=$($counts.pending) sent=$($counts.sent) failed=$($counts.failed)"

Write-Host ""
$line = "================================================"
if ($script:exitcode -eq 0) {
    Write-Host $line
    Write-Host "E2E FLOW PASSED"
    Write-Host $line
} else {
    Write-Host $line
    Write-Host "E2E FLOW FAILED"
    Write-Host $line
}
exit $script:exitcode