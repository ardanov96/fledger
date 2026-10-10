# ==============================================================================
# FLEDGER OS — Start All Services Locally (Host Mode)
# Starts Docker Infrastructure (Postgres, Redis, NATS) + All 6 Go Services
# ==============================================================================

$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  FLEDGER OS — Starting Microservices (Local Host Mode)   " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Start Docker Infrastructure (Postgres, Redis, NATS)
Write-Host "`n[1/3] Ensuring Infrastructure Containers are Running..." -ForegroundColor Yellow
docker compose up -d postgres redis nats

# 2. Setup logs directory
$logDir = "c:\Dev\fledger\.gobuild\logs"
if (-not (Test-Path $logDir)) {
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}

# 3. Define microservices to launch
$services = @(
    @{ Name = "core";    Dir = "c:\Dev\fledger\fledger-core";       Cmd = "go run ./cmd/api";     Port = 8081 },
    @{ Name = "fleet";   Dir = "c:\Dev\fledger\fledger-fleet\src";  Cmd = "go run ./cmd/api";     Port = 8082 },
    @{ Name = "pay";     Dir = "c:\Dev\fledger\fledger-pay\src";    Cmd = "go run ./cmd/api";     Port = 8083 },
    @{ Name = "force";   Dir = "c:\Dev\fledger\fledger-force\src";  Cmd = "go run ./cmd/api";     Port = 8084 },
    @{ Name = "order";   Dir = "c:\Dev\fledger\fledger-order\src";  Cmd = "go run ./cmd/api";     Port = 8085 },
    @{ Name = "dunning"; Dir = "c:\Dev\fledger\fledger-dunning";    Cmd = "go run ./cmd/server";  Port = 8086 }
)

Write-Host "`n[2/3] Launching Microservice Processes in Background..." -ForegroundColor Yellow

foreach ($svc in $services) {
    $logFile = "$logDir\$($svc.Name).log"
    $errFile = "$logDir\$($svc.Name).err.log"
    Write-Host "  Starting $($svc.Name) on port :$($svc.Port)..." -ForegroundColor DarkCyan

    Start-Process powershell -ArgumentList "-NoProfile", "-Command", "cd '$($svc.Dir)'; $($svc.Cmd) *> '$logFile'" -WindowStyle Hidden
}

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
        Write-Host "  [WARN] $($svc.Name) (:$( $svc.Port )) not responding yet (check $logDir\$($svc.Name).log)" -ForegroundColor DarkYellow
    }
}

Write-Host "`n==========================================================" -ForegroundColor Green
Write-Host "  ALL 6 MICROSERVICES STARTED!" -ForegroundColor Green
Write-Host "  • Gateway Hub:       http://localhost:80 (if Docker active)"
Write-Host "  • Fledger Core:      http://localhost:8081/web"
Write-Host "  • Fledger Fleet:     http://localhost:8082/web"
Write-Host "  • Fledger Pay:       http://localhost:8083/web"
Write-Host "  • Fledger Force:     http://localhost:8084/web"
Write-Host "  • Fledger Order:     http://localhost:8085/web"
Write-Host "  • Fledger Dunning:   http://localhost:8086/web"
Write-Host "==========================================================" -ForegroundColor Green
