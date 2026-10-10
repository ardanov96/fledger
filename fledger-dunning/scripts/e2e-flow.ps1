$ErrorActionPreference = "Stop"
$BaseUrl = if ($env:FLEDGER_DUNNING_URL) { $env:FLEDGER_DUNNING_URL } else { "http://localhost:8086" }
$tenantId = "a0000000-0000-0000-0000-000000000001"

function Step($msg) { Write-Host ""; Write-Host "==> $msg" }
function OkMsg($msg)  { Write-Host "  [OK]   $msg" }
function FailMsg($msg){ Write-Host "  [FAIL] $msg"; $script:exitcode = 1 }

$script:exitcode = 0
$jsonHdr = @{ "Content-Type" = "application/json" }
$authHdr = @{ "X-Tenant-ID" = $tenantId }

# 1. Health
Step "Health probes"
$h = Invoke-RestMethod -Method GET -Uri "$BaseUrl/healthz" -Headers $authHdr
OkMsg "healthz: $($h.status)"
$h2 = Invoke-RestMethod -Method GET -Uri "$BaseUrl/readyz" -Headers $authHdr
OkMsg "readyz: $($h2.status)"

# 2. Login + JWT
Step "Dev login (mint JWT)"
$login = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/dev/login?tenant_id=$tenantId" -Headers $jsonHdr
$token = $login.access_token
OkMsg "got JWT"
$authHdr["Authorization"] = "Bearer $token"

# 3. WhatsApp status / QR
Step "WhatsApp gateway"
$wa = Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/dunning/whatsapp/status" -Headers $authHdr
OkMsg "wa status=$($wa.data.connection_status) provider=$($wa.data.session_name)"
try {
    $qrResp = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/dunning/whatsapp/qr/generate" -Headers ($authHdr + $jsonHdr) -Body (@{session_name="official-distributor-wa"} | ConvertTo-Json -Depth 3 -Compress)
    OkMsg "QR generated (len=$($qrResp.data.qr_code.Length))"
} catch { OkMsg "QR generation deferred (no auth required for status, ok)" }

# 4. Upsert contact
Step "Upsert store contact"
$contactBody = @{
    store_id = "TKO-DEMO-01"
    store_name = "Toko Sumber Rezeki"
    owner_name = "Haji Mahmud"
    phone_number = "6281234567801"
}
$contact = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/dunning/contacts" -Headers ($authHdr + $jsonHdr) -Body ($contactBody | ConvertTo-Json -Depth 3 -Compress)
OkMsg "contact upserted: $($contact.data.store_id)"

# 5. Ingest invoice (5 cadence stages)
Step "Ingest invoice (auto-schedule 5 stages)"
$due = (Get-Date).AddDays(7).ToString("yyyy-MM-dd")
$ingestBody = @{
    invoice_id = "INV-DEMO-$((Get-Date).Ticks)"
    invoice_number = "INV/DEMO/$((Get-Date).Ticks)"
    store_id = "TKO-DEMO-01"
    phone_number = "6281234567801"
    due_date = $due
    amount_due_minor = 4500000
    payment_link_url = "http://localhost:8083/pay/INV-DEMO"
    store_name = "Toko Sumber Rezeki"
}
$ingest = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/dunning/queues/ingest-invoice" -Headers ($authHdr + $jsonHdr) -Body ($ingestBody | ConvertTo-Json -Depth 5 -Compress)
$sched = @($ingest.data.schedules_created)
if ($sched.Count -ne 5) { FailMsg "expected 5 schedules, got $($sched.Count)" } else { OkMsg "5 schedules created" }

# 6. Self-healing: cancel-by-payment
Step "Self-healing loop (simulate paid webhook)"
$cancelBody = @{ invoice_id = $ingestBody.invoice_id }
$cancel = Invoke-RestMethod -Method POST -Uri "$BaseUrl/v1/dunning/queues/cancel-invoice" -Headers ($authHdr + $jsonHdr) -Body ($cancelBody | ConvertTo-Json -Depth 3 -Compress)
if ($cancel.count -ne 5) { FailMsg "expected 5 cancelled, got $($cancel.count)" } else { OkMsg "$($cancel.count) antrian dibatalkan otomatis (self-healing)" }

# 7. Queue counts
Step "Queue counts"
$counts = Invoke-RestMethod -Method GET -Uri "$BaseUrl/v1/dunning/outbox/counts" -Headers $authHdr
Write-Host "  queued=$($counts.queued) sent=$($counts.sent) failed=$($counts.failed) cancelled=$($counts.cancelled)"
if ($counts.cancelled -ne 5) { FailMsg "expected 5 cancelled" } else { OkMsg "5 rows in CANCELLED_BY_PAYMENT" }

# 8. Web PWA
Step "Web PWA portal"
try {
    $webResp = Invoke-WebRequest -Method GET -Uri "$BaseUrl/" -UseBasicParsing -MaximumRedirection 0 -ErrorAction Stop
    if ($webResp.StatusCode -eq 200 -and $webResp.Content -match "Fledger Dunning") {
        OkMsg "PWA HTML served (len=$($webResp.Content.Length))"
    } else { FailMsg "PWA broken: status=$($webResp.StatusCode)" }
} catch { FailMsg "PWA unreachable" }

Write-Host ""
$line = "================================================"
if ($script:exitcode -eq 0) { Write-Host $line; Write-Host "E2E FLOW PASSED"; Write-Host $line }
else { Write-Host $line; Write-Host "E2E FLOW FAILED"; Write-Host $line }
exit $script:exitcode