# ==============================================================================
# FLEDGER OS - Start All Services Locally (Host Production Mode)
# Starts All 6 Go Microservices + Central Gateway Hub (:8000)
# ==============================================================================

[CmdletBinding()]
param(
    [switch]$NoSeed,
    [switch]$Wait
)

$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  FLEDGER OS - Microservices Local Production Runner      " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

$base = "C:\Dev\fledger"
$logDir = "$base\.gobuild\logs"
if (-not (Test-Path $logDir)) {
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}

# 1. Stop any existing instances on ports 8000, 8081-8086
Write-Host "`n[1/4] Cleaning up existing processes..." -ForegroundColor Yellow
$ports = @(8000, 8081, 8082, 8083, 8084, 8085, 8086)
foreach ($p in $ports) {
    try {
        $pids = Get-NetTCPConnection -LocalPort $p -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique
        foreach ($procId in $pids) {
            if ($procId -and $procId -ne 0) {
                Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
            }
        }
    } catch {}
}
Start-Sleep -Milliseconds 500

# 2. Define all microservices
$services = @(
    @{
        Name    = "core"
        Title   = "Fledger Core Ledger"
        Port    = 8081
        Exe     = "$base\.gobuild\bin\core.exe"
        WorkDir = $base
        Env     = @{
            APP_ENV         = "development"
            PORT            = "8081"
            DATABASE_URL    = "postgres://fmcg:fmcg_dev_password@localhost:5432/fmcg_wallet?sslmode=disable"
            REDIS_URL       = "redis://localhost:6379/0"
            NATS_URL        = "nats://localhost:4222"
            JWT_SECRET      = "fledger-super-secure-jwt-secret-key-2026-min-32"
            RBAC_POLICY_DIR = "$base\fledger-core\internal\auth\rbac\policies"
        }
    },
    @{
        Name    = "fleet"
        Title   = "Fledger Fleet Logistics"
        Port    = 8082
        Exe     = "$base\.gobuild\bin\fleet.exe"
        WorkDir = "$base\fledger-fleet"
        Env     = @{
            PORT              = "8082"
            DATABASE_URL      = "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_fleet?sslmode=disable"
            FLEDGER_CORE_URL  = "http://localhost:8081"
            FLEDGER_TENANT_ID = "a0000000-0000-0000-0000-000000000001"
            JWT_SECRET        = "fledger-super-secure-jwt-secret-key-2026-min-32"
        }
    },
    @{
        Name    = "pay"
        Title   = "Fledger Pay Gateway"
        Port    = 8083
        Exe     = "$base\.gobuild\bin\pay.exe"
        WorkDir = "$base\fledger-pay"
        Env     = @{
            PORT                = "8083"
            DATABASE_URL        = "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_pay?sslmode=disable"
            FLEDGER_CORE_URL    = "http://localhost:8081"
            FLEDGER_DUNNING_URL = "http://localhost:8086"
            FLEDGER_TENANT_ID   = "a0000000-0000-0000-0000-000000000001"
            JWT_SECRET          = "fledger-super-secure-jwt-secret-key-2026-min-32"
            WEBHOOK_SECRET      = "dunning-super-secret-key-2026"
        }
    },
    @{
        Name    = "force"
        Title   = "Fledger Force SFA"
        Port    = 8084
        Exe     = "$base\.gobuild\bin\force.exe"
        WorkDir = "$base\fledger-force"
        Env     = @{
            PORT              = "8084"
            DATABASE_URL      = "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_force?sslmode=disable"
            FLEDGER_CORE_URL  = "http://localhost:8081"
            FLEDGER_TENANT_ID = "a0000000-0000-0000-0000-000000000001"
            JWT_SECRET        = "fledger-super-secure-jwt-secret-key-2026-min-32"
        }
    },
    @{
        Name    = "order"
        Title   = "Fledger Order OMS"
        Port    = 8085
        Exe     = "$base\.gobuild\bin\order.exe"
        WorkDir = "$base\fledger-order"
        Env     = @{
            PORT                 = "8085"
            DATABASE_URL         = "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_order?sslmode=disable"
            FLEDGER_CORE_URL     = "http://localhost:8081"
            FLEDGER_FLEET_URL    = "http://localhost:8082"
            FLEDGER_TENANT_ID    = "a0000000-0000-0000-0000-000000000001"
            JWT_SECRET           = "fledger-super-secure-jwt-secret-key-2026-min-32"
            MANAGER_OVERRIDE_PIN = "123456"
        }
    },
    @{
        Name    = "dunning"
        Title   = "Fledger Dunning WA"
        Port    = 8086
        Exe     = "$base\.gobuild\bin\dunning.exe"
        WorkDir = "$base\fledger-dunning"
        Env     = @{
            PORT                  = "8086"
            DATABASE_URL          = "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_dunning?sslmode=disable"
            FLEDGER_CORE_URL      = "http://localhost:8081"
            FLEDGER_PAY_URL       = "http://localhost:8083"
            FLEDGER_TENANT_ID     = "a0000000-0000-0000-0000-000000000001"
            JWT_SECRET            = "fledger-super-secure-jwt-secret-key-2026-min-32"
            PAY_WEBHOOK_SECRET    = "dunning-super-secret-key-2026"
            WA_PROVIDER           = "MOCK"
            WA_JITTER_MIN_SECONDS = "0"
            WA_JITTER_MAX_SECONDS = "1"
        }
    },
    @{
        Name    = "gateway"
        Title   = "Unified API Gateway"
        Port    = 8000
        Exe     = "$base\.gobuild\bin\gateway.exe"
        WorkDir = "$base\deployments\gateway"
        Env     = @{
            PORT = "8000"
        }
    }
)

# 3. Launch each process detached
Write-Host "`n[2/4] Launching Microservices & Gateway..." -ForegroundColor Yellow
foreach ($s in $services) {
    $envCmd = ($s.Env.GetEnumerator() | ForEach-Object { "set ""$($_.Key)=$($_.Value)""" }) -join " && "
    $fullCmd = "cmd /c ""$envCmd && `"$($s.Exe)`" >> `"$logDir\$($s.Name).out`" 2>>&1"""
    Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{
        CommandLine      = $fullCmd
        CurrentDirectory = $s.WorkDir
    } | Out-Null
    Write-Host "  Started $($s.Title) on port :$($s.Port)" -ForegroundColor Gray
}

# 4. Wait for healthz probes
Write-Host "`n[3/4] Probing Service Health..." -ForegroundColor Yellow
Start-Sleep -Seconds 2

$allHealthy = $true
foreach ($s in $services) {
    $healthy = $false
    for ($i = 0; $i -lt 15; $i++) {
        try {
            $r = Invoke-RestMethod -Uri "http://localhost:$($s.Port)/healthz" -Method GET -TimeoutSec 1
            $healthy = $true
            break
        } catch {
            Start-Sleep -Seconds 1
        }
    }
    if ($healthy) {
        Write-Host "  [OK] $($s.Title) (:$( $s.Port )) is UP and healthy" -ForegroundColor Green
    } else {
        Write-Host "  [WARN] $($s.Title) (:$( $s.Port )) not responding yet (check $logDir\$($s.Name).out)" -ForegroundColor DarkYellow
        $allHealthy = $false
    }
}

# 5. Optional Auto-Seed
if (-not $NoSeed -and $allHealthy) {
    Write-Host "`n[4/4] Seeding Demo FMCG Data..." -ForegroundColor Yellow
    try {
        & powershell.exe -ExecutionPolicy Bypass -File "$PSScriptRoot\seed-ecosystem.ps1"
    } catch {
        Write-Host "  Seed warning: $($_.Exception.Message)" -ForegroundColor DarkYellow
    }
}

Write-Host "`n==========================================================" -ForegroundColor Green
Write-Host "  FLEDGER OS ECOSYSTEM READY FOR MANUAL TESTING!          " -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
Write-Host "  * Master Control Hub: http://localhost:8000"
Write-Host "  * [Core Ledger]       http://localhost:8081/web"
Write-Host "  * [Fleet Logistics]   http://localhost:8082/web"
Write-Host "  * [Payment Gateway]   http://localhost:8083"
Write-Host "  * [Force SFA]         http://localhost:8084"
Write-Host "  * [Order OMS]         http://localhost:8085"
Write-Host "  * [Dunning WA]        http://localhost:8086"
Write-Host "==========================================================" -ForegroundColor Green
Write-Host "  Logs: $logDir"
Write-Host "  Stop all: powershell -ExecutionPolicy Bypass -File .\scripts\stop-all-local.ps1"
Write-Host "==========================================================" -ForegroundColor Green

if ($Wait) {
    Write-Host "`nRunning in foreground mode (Press Ctrl+C to stop)..." -ForegroundColor Cyan
    try {
        while ($true) { Start-Sleep -Seconds 2 }
    } finally {
        & "$PSScriptRoot\stop-all-local.ps1"
    }
}
