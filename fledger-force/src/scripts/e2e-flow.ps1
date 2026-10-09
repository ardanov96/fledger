$ErrorActionPreference = "Stop"
$BaseUrl = if ($env:FLEDGER_FORCE_URL) { $env:FLEDGER_FORCE_URL } else { "http://localhost:8084" }
$env:PGPASSWORD = if ($env:PGPASSWORD) { $env:PGPASSWORD } else { "fmcg_dev_password" }

function Step($msg) { Write-Host ""; Write-Host "==> $msg" }
function OkMsg($msg)  { Write-Host "  [OK]   $msg" }
function FailMsg($msg){ Write-Host "  [FAIL] $msg"; $script:exitcode = 1 }

$script:exitcode = 0
$jsonHdr = @{ "Content-Type" = "application/json" }

# 1. Login
Step "Login (dev endpoint)"
$login = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/dev/login" -Headers $jsonHdr
$token = $login.access_token
$tenantId = $login.tenant_id
if (-not $token) { FailMsg "no token"; exit 1 }
OkMsg "got JWT"
$authHdr = @{ Authorization = "Bearer $token"; "X-Tenant-ID" = $tenantId }

# 2. Health
Step "Health probes"
$h = Invoke-RestMethod -Method GET -Uri "$BaseUrl/healthz" -Headers $authHdr
OkMsg "healthz: $($h.status)"
$h2 = Invoke-RestMethod -Method GET -Uri "$BaseUrl/readyz" -Headers $authHdr
OkMsg "readyz: $($h2.status)"

# 3. Setup: create rep + store + beat plan
Step "Setup master data"
$suffix = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$repBody = @{
    employee_code = "SLS-DEMO-$suffix"
    name = "Budi Santoso"
    phone = "081234567890"
    role = "CANVASSER"
    fledger_wallet_account_id = "ACC_SALES_WALLET_BUDI"
    max_cash_limit = 50000000
}
$rep = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/force/sales-reps" -Headers ($authHdr + $jsonHdr) -Body ($repBody | ConvertTo-Json -Depth 5 -Compress)
$repId = $rep.id
OkMsg "rep created: $repId ($($rep.employee_code))"

$storeBody = @{
    store_code = "TKO-DEMO-$suffix"
    name = "Toko Sumber Rezeki"
    owner_name = "Ibu Siti"
    phone = "081298765432"
    address = "Jl. Monas No.1, Jakarta Pusat"
    latitude = -6.175392
    longitude = 106.827153
    geofence_radius_meters = 100
    tier = "RETAIL"
    credit_limit = 5000000
}
$store = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/force/stores" -Headers ($authHdr + $jsonHdr) -Body ($storeBody | ConvertTo-Json -Depth 5 -Compress)
$storeId = $store.id
OkMsg "store created: $storeId ($($store.store_code))"

$today = Get-Date -Format "yyyy-MM-dd"
$planBody = @{
    sales_rep_id = $repId
    plan_date = $today
    territory = "Jakarta Pusat - Demo"
    target_stores_count = 1
    notes = "E2E smoke test"
}
$plan = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/force/beat-plans" -Headers ($authHdr + $jsonHdr) -Body ($planBody | ConvertTo-Json -Depth 5 -Compress)
$planId = $plan.id
OkMsg "beat plan: $planId"

# 4. Check-in within geofence
Step "Check-in (within geofence)"
$checkInBody = @{
    beat_plan_id = $planId
    store_id = $storeId
    latitude = -6.175392
    longitude = 106.827153
    visit_type = "TAKING_ORDER_AND_COLLECTION"
}
$checkIn = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/force/visits/check-in?sales_rep_id=$repId" -Headers ($authHdr + $jsonHdr) -Body ($checkInBody | ConvertTo-Json -Depth 5 -Compress)
Write-Host "  store: $($checkIn.store_name), distance: $($checkIn.distance_meters) m, verified: $($checkIn.geofence_verified)"
if ($checkIn.geofence_verified -ne $true) { FailMsg "geofence check failed" } else { OkMsg "geofence PASS (< 100m)" }
$visitId = $checkIn.id

# 5. Check-in OUT of geofence (audit log)
Step "Check-in (out of geofence, audit trail)"
$farBody = @{
    beat_plan_id = $planId
    store_id = $storeId
    latitude = -6.189
    longitude = 106.840
    visit_type = "CASH_COLLECTION"
}
$farCheckIn = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/force/visits/check-in?sales_rep_id=$repId" -Headers ($authHdr + $jsonHdr) -Body ($farBody | ConvertTo-Json -Depth 5 -Compress)
Write-Host "  distance: $($farCheckIn.distance_meters) m, verified: $($farCheckIn.geofence_verified)"
if ($farCheckIn.geofence_verified -ne $false) { FailMsg "should be out of geofence" } else { OkMsg "out-of-radius flagged" }

# 6. Collect cash (Sprint 3 - anti-cash-kitting)
Step "Collect cash Rp 2.500.000"
$invoiceId = [Guid]::NewGuid().ToString()
$collectBody = @{
    visit_id = $visitId
    store_id = $storeId
    fledger_invoice_id = $invoiceId
    amount = 2500000
    payer_name = "Ibu Siti Fatimah"
    payer_phone = "081298765432"
}
$collect = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/force/collections?sales_rep_id=$repId" -Headers ($authHdr + $jsonHdr) -Body ($collectBody | ConvertTo-Json -Depth 5 -Compress)
Write-Host "  receipt: $($collect.collection.receipt_number)"
Write-Host "  salesman_cash: $($collect.sales_rep_current_cash_held)"
Write-Host "  wa_msg: $($collect.wa_receipt_payload.message)"
if ($collect.collection.amount -ne 2500000) { FailMsg "amount wrong" } else { OkMsg "amount=2,500,000" }
if ($collect.sales_rep_current_cash_held -ne 2500000) { FailMsg "cash not updated" } else { OkMsg "salesman cash=2,500,000" }
if (-not $collect.collection.receipt_number.StartsWith("RCP-")) { FailMsg "receipt format" } else { OkMsg "receipt RCP-202610-XXXXX" }
$collectionId = $collect.collection.id

# 7. Outbox counts
Step "Outbox counts (Core offline - PENDING expected)"
$counts = Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/force/outbox/counts" -Headers $authHdr
Write-Host "  pending=$($counts.pending) sent=$($counts.sent) failed=$($counts.failed)"
if ($counts.pending -lt 1) { FailMsg "no outbox row" } else { OkMsg "outbox PENDING=$($counts.pending)" }

# 8. Cashier inquiry
Step "Cashier inquiry (Sprint 4 - EOD)"
$inquiry = Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/force/settlements/reps/$repId/inquiry" -Headers $authHdr
Write-Host "  wajib_setor: $($inquiry.total_cash_held)"
Write-Host "  collections: $($inquiry.collections_count)"
if ($inquiry.total_cash_held -ne 2500000) { FailMsg "inquiry total wrong" } else { OkMsg "inquiry shows 2,500,000" }

# 9. EOD settlement (cash pas)
Step "EOD settlement (cash pas)"
$settleBody = @{
    sales_rep_id = $repId
    physical_cash_received = 2500000
    cashier_notes = "Uang pas, e2e test"
}
$settle = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/force/settlements" -Headers ($authHdr + $jsonHdr) -Body ($settleBody | ConvertTo-Json -Depth 5 -Compress)
Write-Host "  settlement: $($settle.settlement.settlement_number)"
Write-Host "  discrepancy: $($settle.discrepancy_amount)"
Write-Host "  new_status: $($settle.sales_rep_status)"
Write-Host "  new_cash: $($settle.sales_rep_new_cash_held)"
if ($settle.sales_rep_status -ne "ACTIVE") { FailMsg "rep not restored to ACTIVE" } else { OkMsg "rep restored to ACTIVE" }
if ($settle.sales_rep_new_cash_held -ne 0) { FailMsg "cash not reset" } else { OkMsg "salesman cash reset to 0" }

# 10. Negative: re-collect when salesman is already SETTLED
Step "Negative: max_cash_limit guard"
# Reduce rep limit and try to over-collect
& "C:\Program Files\PostgreSQL\17\bin\psql.exe" -h localhost -U fmcg -d fledger_force -c "UPDATE force_sales_reps SET max_cash_limit = 1000000 WHERE id = '$repId'" 2>&1 | Out-Null
$overBody = @{
    store_id = $storeId
    fledger_invoice_id = [Guid]::NewGuid().ToString()
    amount = 2000000
    payer_name = "Pak X"
}
try {
    Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/force/collections?sales_rep_id=$repId" -Headers ($authHdr + $jsonHdr) -Body ($overBody | ConvertTo-Json -Depth 5 -Compress) | Out-Null
    FailMsg "expected 422 for cash limit, got 200"
} catch {
    if ($_.Exception.Response.StatusCode -eq 422) { OkMsg "correctly rejected with 422" } else { FailMsg "wrong status: $($_.Exception.Response.StatusCode)" }
}

# 11. Web portal reachable
Step "Web PWA portal"
$webResp = Invoke-WebRequest -Method GET -Uri "$BaseUrl/" -UseBasicParsing
if ($webResp.StatusCode -eq 200 -and $webResp.Content -match "Fledger Force") {
    OkMsg "PWA HTML served (len=$($webResp.Content.Length))"
} else {
    FailMsg "PWA broken: status=$($webResp.StatusCode)"
}

# Final
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