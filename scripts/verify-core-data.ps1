$loginBody = @{
    tenant_id = '00000000-0000-0000-0000-000000000001'
    username  = '33333333-3333-3333-3333-333333333333'
    password  = 'DemoTest1234!'
} | ConvertTo-Json

$loginRes = Invoke-RestMethod -Uri 'http://localhost:8081/v1/auth/login' -Method Post -Body $loginBody -ContentType 'application/json'
$token = $loginRes.data.access_token
Write-Host "Token acquired successfully!"

$headers = @{
    'Authorization' = "Bearer $token"
    'X-Tenant-ID'   = '00000000-0000-0000-0000-000000000001'
}

$accounts   = (Invoke-RestMethod -Uri 'http://localhost:8081/v1/accounts' -Headers $headers).data
$invoices   = (Invoke-RestMethod -Uri 'http://localhost:8081/v1/invoices?limit=200' -Headers $headers).data
$periods    = (Invoke-RestMethod -Uri 'http://localhost:8081/v1/periods' -Headers $headers).data
$runs       = (Invoke-RestMethod -Uri 'http://localhost:8081/v1/reconciler/runs?limit=200' -Headers $headers).data
$currencies = (Invoke-RestMethod -Uri 'http://localhost:8081/v1/currencies' -Headers $headers).data
$audit      = (Invoke-RestMethod -Uri 'http://localhost:8081/v1/audit?limit=200' -Headers $headers).data

# Transfers: aggregate entries from cash accounts
$cashAccounts = $accounts | Where-Object { $_.type -eq 'cash' }
$txIds = [System.Collections.Generic.HashSet[string]]::new()
foreach ($c in $cashAccounts) {
    try {
        $entries = (Invoke-RestMethod -Uri "http://localhost:8081/v1/accounts/$($c.id)/entries?limit=50" -Headers $headers).data
        foreach ($e in $entries) {
            if ($e.transaction_id) { [void]$txIds.Add($e.transaction_id) }
        }
    } catch {}
}

# Aging: check first customer
$custAccounts = $accounts | Where-Object { $_.type -eq 'customer' }
$firstCustAging = if ($custAccounts.Count -gt 0) {
    (Invoke-RestMethod -Uri "http://localhost:8081/v1/customers/$($custAccounts[0].id)/aging" -Headers $headers).data
} else { @() }

[PSCustomObject]@{
    '1. Accounts (Akun)'            = $accounts.Count
    '2. Transfers (Transaksi Kas)'  = $txIds.Count
    '3. Invoices (Faktur)'          = $invoices.Count
    '4. Aging Summary Buckets'      = $firstCustAging.Count
    '5. Accounting Periods'         = $periods.Count
    '6. Reconciler Runs'            = $runs.Count
    '7. Currencies (Mata Uang)'     = $currencies.Count
    '8. Audit Logs (Log Audit)'     = $audit.Count
} | Format-List
