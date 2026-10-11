# ==============================================================================
# FLEDGER OS - Full Local Production Verification Runner
# Starts 6 Services -> Waits for Healthz -> Seeds Ecosystem -> Runs E2E Golden Flow
# ==============================================================================

$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  FLEDGER OS - Full Verification Pipeline (Host Mode)     " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Start all services using start-all-services.cmd
Write-Host "`n[1/4] Starting Microservice Processes..." -ForegroundColor Yellow
& cmd.exe /c "$PSScriptRoot\start-all-services.cmd"

$services = @(
    @{ Name = "Core Ledger";      Port = 8081 },
    @{ Name = "Fleet POD";        Port = 8082 },
    @{ Name = "Payment Gateway";  Port = 8083 },
    @{ Name = "Force SFA";        Port = 8084 },
    @{ Name = "Order OMS";        Port = 8085 },
    @{ Name = "Dunning WA";       Port = 8086 }
)

# 2. Wait for probes
Write-Host "`n[2/4] Waiting for Microservices to become Healthy..." -ForegroundColor Yellow
Start-Sleep -Seconds 2

$allHealthy = $true
foreach ($svc in $services) {
    $healthy = $false
    for ($i = 0; $i -lt 15; $i++) {
        try {
            $r = Invoke-RestMethod -Uri "http://localhost:$($svc.Port)/healthz" -Method GET -TimeoutSec 1
            $healthy = $true
            break
        } catch {
            Start-Sleep -Seconds 1
        }
    }
    if ($healthy) {
        Write-Host "  [OK] $($svc.Name) (:$( $svc.Port )) is UP and healthy" -ForegroundColor Green
    } else {
        Write-Host "  [FAIL] $($svc.Name) (:$( $svc.Port )) not responding!" -ForegroundColor Red
        $allHealthy = $false
    }
}

if (-not $allHealthy) {
    Write-Host "`n[FATAL] Not all services became healthy. Aborting pipeline." -ForegroundColor Red
    exit 1
}

# 3. Seed Ecosystem
Write-Host "`n[3/4] Seeding Ecosystem Data..." -ForegroundColor Yellow
& powershell.exe -ExecutionPolicy Bypass -File "$PSScriptRoot\seed-ecosystem.ps1"

# 4. Run E2E Integration Test
Write-Host "`n[4/4] Executing Golden FMCG E2E Flow Test..." -ForegroundColor Yellow
& powershell.exe -ExecutionPolicy Bypass -File "$PSScriptRoot\test-ecosystem-e2e.ps1"
