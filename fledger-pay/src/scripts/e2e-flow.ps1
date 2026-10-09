$ErrorActionPreference = "Stop"
$BaseUrl = if ($env:FLEDGER_PAY_URL) { $env:FLEDGER_PAY_URL } else { "http://localhost:8083" }
$runTag = [DateTime]::UtcNow.ToString("HHmmssfff")

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

# 3. Create payment request
Step "Create payment request"
$invoiceId = [Guid]::NewGuid().ToString()
$customerId = "cccccccc-0001-0000-0000-000000000001"
$prBody = @{
    fledger_invoice_id = $invoiceId
    customer_id = $customerId
    customer_name = "Toko Sumber Rezeki"
    customer_phone = "081298765432"
    amount = 4000000
    expiry_minutes = 60
    enabled_banks = @("BCA", "MANDIRI")
    enable_qris = $true
    merchant_city = "JAKARTA"
}
$pr = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/pay/requests" -Headers ($authHdr + $jsonHdr) -Body ($prBody | ConvertTo-Json -Depth 6 -Compress)
Write-Host "  request_number: $($pr.request.request_number)"
Write-Host "  amount:         $($pr.request.amount)"
Write-Host "  VA count:       $($pr.virtual_accounts.Count)"
Write-Host "  QRIS present:   $($null -ne $pr.qris)"
if ($pr.request.amount -ne 4000000) { FailMsg "amount wrong: $($pr.request.amount)" } else { OkMsg "amount=4,000,000" }
if ($pr.virtual_accounts.Count -ne 2) { FailMsg "VA count wrong" } else { OkMsg "2 VAs created" }
if (-not $pr.qris) { FailMsg "QRIS missing" } else { OkMsg "QRIS generated" }
$requestNumber = $pr.request.request_number

# 4. List payments
Step "List payment requests"
$list = Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/pay/requests" -Headers $authHdr
if ($list.Count -lt 1) { FailMsg "list empty" } else { OkMsg "listed $($list.Count) requests" }

# 5. Simulate settlement (BCA VA)
Step "Simulate settlement via BCA VA"
$settleBody = @{
    request_number = $requestNumber
    channel = "VA_BCA"
    amount = 4000000
    payer_name = "Ibu Siti Fatimah (BCA Mobile Demo)"
}
$settle = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/pay/simulator/settle" -Headers ($authHdr + $jsonHdr) -Body ($settleBody | ConvertTo-Json -Depth 6 -Compress)
Write-Host "  settled: $($settle.status)"
Write-Host "  tx_id:   $($settle.transaction_id)"
if ($settle.status -ne "SETTLED") { FailMsg "settle status wrong: $($settle.status)" } else { OkMsg "status=SETTLED" }

# 6. Idempotency replay
Step "Idempotency replay (duplicate simulator call)"
try {
    $settle2 = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/pay/simulator/settle" -Headers ($authHdr + $jsonHdr) -Body ($settleBody | ConvertTo-Json -Depth 6 -Compress)
    if ($settle2.idempotent_replay -ne $true) { FailMsg "expected idempotent_replay=true" } else { OkMsg "replay flagged correctly" }
} catch { FailMsg "replay threw: $_" }

# 7. Get payment (status should now be SETTLED)
Step "Get payment after settle"
$detail = Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/pay/requests/$($pr.request.id)" -Headers $authHdr
if ($detail.request.status -ne "SETTLED") { FailMsg "DO not SETTLED: $($detail.request.status)" } else { OkMsg "DO is SETTLED" }

# 8. Outbox counts (Core offline → PENDING expected)
Step "Outbox counts"
$counts = Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/pay/outbox/counts" -Headers $authHdr
Write-Host "  pending=$($counts.pending) processing=$($counts.processing) sent=$($counts.sent) failed=$($counts.failed)"
if ($counts.pending -lt 1) { FailMsg "expected at least 1 pending outbox row" } else { OkMsg "outbox has $($counts.pending) pending rows" }

# 9. Negative: POD without auth should be 401
Step "Negative: list without auth"
try {
    Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/pay/requests" | Out-Null
    FailMsg "expected 401, got 200"
} catch {
    if ($_.Exception.Response.StatusCode -eq 401) { OkMsg "correctly rejected with 401" } else { FailMsg "wrong status: $($_.Exception.Response.StatusCode)" }
}

# 10. Web portal reachable
Step "Web simulator portal"
$webResp = Invoke-WebRequest -Method GET -Uri "$BaseUrl/" -UseBasicParsing
$webOk = ($webResp.StatusCode -eq 200) -and ($webResp.Content -match "Fledger Pay")
if ($webOk) { OkMsg "portal HTML returned" } else { FailMsg "portal broken: status=$($webResp.StatusCode)" }

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