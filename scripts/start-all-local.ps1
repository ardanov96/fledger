# ==============================================================================
# FLEDGER OS - Start All Services Locally (Host Mode)
# Starts Docker Infrastructure (Postgres, Redis, NATS) + All 6 Go Services
# ==============================================================================

[CmdletBinding()]
param(
    [switch]$Wait
)

$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  FLEDGER OS - Starting Microservices (Local Host Mode)   " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Start Docker Infrastructure (Postgres, Redis, NATS)
Write-Host "`n[1/3] Checking Infrastructure..." -ForegroundColor Yellow
try {
    docker compose up -d postgres redis nats 2>$null
    Write-Host "  Docker infrastructure containers active." -ForegroundColor Green
} catch {
    Write-Host "  Docker daemon not detected; continuing with local host infrastructure (Postgres :5432, etc.)." -ForegroundColor DarkYellow
}

# 2. Setup logs directory
$logDir = "c:\Dev\fledger\.gobuild\logs"
if (-not (Test-Path $logDir)) {
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}

# 3. Launch all microservices using batch orchestrator
Write-Host "`n[2/3] Launching Microservice Processes in Background..." -ForegroundColor Yellow
& cmd.exe /c "$PSScriptRoot\start-all-services.cmd"

$services = @(
    @{ Name = "core";    Port = 8081 },
    @{ Name = "fleet";   Port = 8082 },
    @{ Name = "pay";     Port = 8083 },
    @{ Name = "force";   Port = 8084 },
    @{ Name = "order";   Port = 8085 },
    @{ Name = "dunning"; Port = 8086 }
)

# 4. Wait for probes
Write-Host "`n[3/3] Waiting for Microservices to become Healthy..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

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
        Write-Host "  [WARN] $($svc.Name) (:$( $svc.Port )) not responding yet (check $logDir\$($svc.Name).out)" -ForegroundColor DarkYellow
    }
}

Write-Host "`n==========================================================" -ForegroundColor Green
Write-Host "  ALL 6 MICROSERVICES STARTED!" -ForegroundColor Green
Write-Host "  * Gateway Hub:       http://localhost:80 (if Docker active)"
Write-Host "  * Fledger Core:      http://localhost:8081/web"
Write-Host "  * Fledger Fleet:     http://localhost:8082/web"
Write-Host "  * Fledger Pay:       http://localhost:8083/web"
Write-Host "  * Fledger Force:     http://localhost:8084/web"
Write-Host "  * Fledger Order:     http://localhost:8085/web"
Write-Host "  * Fledger Dunning:   http://localhost:8086/web"
Write-Host "==========================================================" -ForegroundColor Green

if ($Wait) {
    Write-Host "`nRunning in foreground mode (Press Ctrl+C to stop services)..." -ForegroundColor Cyan
    try {
        while ($true) {
            Start-Sleep -Seconds 2
        }
    } finally {
        Write-Host "`nStopping microservices..." -ForegroundColor Yellow
        Get-Process core, fleet, pay, force, order, dunning -ErrorAction SilentlyContinue | Stop-Process -Force
        Write-Host "Services stopped cleanly." -ForegroundColor Green
    }
}
