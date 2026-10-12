$endpoints = @(
    @{ Name = "Gateway Hub";       Url = "http://localhost:8000/" },
    @{ Name = "Core Ledger Web";   Url = "http://localhost:8081/web/" },
    @{ Name = "Fleet Logistics";   Url = "http://localhost:8082/" },
    @{ Name = "Pay Checkout";      Url = "http://localhost:8083/" },
    @{ Name = "Force SFA PWA";     Url = "http://localhost:8084/" },
    @{ Name = "Order OMS Gate";    Url = "http://localhost:8085/" },
    @{ Name = "Dunning WA Hub";    Url = "http://localhost:8086/" }
)

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  FLEDGER OS - Microservices Frontend UI Health Prober   " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

$allOk = $true
foreach ($ep in $endpoints) {
    try {
        $res = Invoke-WebRequest -Uri $ep.Url -UseBasicParsing -TimeoutSec 3
        if ($res.StatusCode -eq 200) {
            Write-Host "  [200 OK] $($ep.Name.PadRight(22)) => $($ep.Url)" -ForegroundColor Green
        } else {
            Write-Host "  [$($res.StatusCode)] $($ep.Name.PadRight(22)) => $($ep.Url)" -ForegroundColor Yellow
            $allOk = $false
        }
    } catch {
        Write-Host "  [FAIL]   $($ep.Name.PadRight(22)) => $($_.Exception.Message)" -ForegroundColor Red
        $allOk = $false
    }
}

Write-Host "==========================================================" -ForegroundColor Cyan
if ($allOk) {
    Write-Host "ALL 7 FRONTEND UIs ARE SERVING 200 OK WITH LAUNCH UI!" -ForegroundColor Green
} else {
    Write-Host "SOME ENDPOINTS HAD ISSUES" -ForegroundColor Red
}
