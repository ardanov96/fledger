param(
    [Parameter(Mandatory=$true)]
    [string]$ServiceName
)

$base = "C:\Dev\fledger"
$logDir = "$base\.gobuild\logs"

$services = @{
    "dunning" = @{
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
            WA_PROVIDER           = if ($env:WA_PROVIDER) { $env:WA_PROVIDER } else { "FONNTE" }
            FONNTE_TOKEN          = if ($env:FONNTE_TOKEN) { $env:FONNTE_TOKEN } else { "EM8i5t8sWe11CpqBhLTx" }
            FONNTE_BASE_URL       = if ($env:FONNTE_BASE_URL) { $env:FONNTE_BASE_URL } else { "https://api.fonnte.com" }
            WA_JITTER_MIN_SECONDS = "0"
            WA_JITTER_MAX_SECONDS = "1"
        }
    }
    "force" = @{
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
    }
    "fleet" = @{
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
    }
    "order" = @{
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
    }
    "pay" = @{
        Port    = 8083
        Exe     = "$base\.gobuild\bin\pay.exe"
        WorkDir = "$base\fledger-pay"
        Env     = @{
            PORT                 = "8083"
            DATABASE_URL         = "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_pay?sslmode=disable"
            FLEDGER_CORE_URL     = "http://localhost:8081"
            FLEDGER_DUNNING_URL  = "http://localhost:8086"
            FLEDGER_TENANT_ID    = "a0000000-0000-0000-0000-000000000001"
            JWT_SECRET           = "fledger-super-secure-jwt-secret-key-2026-min-32"
            WEBHOOK_SECRET       = "SB-Mid-server-XJsYUlLDpouyLjW5E6NxELyI"
            MIDTRANS_SERVER_KEY  = "SB-Mid-server-XJsYUlLDpouyLjW5E6NxELyI"
            MIDTRANS_CLIENT_KEY  = "SB-Mid-client-5c-O12snGFaCx2fj"
            MIDTRANS_MERCHANT_ID = "G426768001"
        }
    }
    "gateway" = @{
        Port    = 8000
        Exe     = "$base\.gobuild\bin\gateway.exe"
        WorkDir = "$base\deployments\gateway"
        Env     = @{
            PORT = "8000"
        }
    }
}

if (-not $services.ContainsKey($ServiceName)) {
    Write-Error "Service $ServiceName not found in registry"
    exit 1
}

$s = $services[$ServiceName]
$port = $s.Port

# 1. Stop existing process on port
Write-Host "Stopping existing process on port :$port..." -ForegroundColor Yellow
$pids = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique
foreach ($procId in $pids) {
    if ($procId -and $procId -ne 0) {
        Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
    }
}
Start-Sleep -Milliseconds 600

# 2. Start process
Write-Host "Starting $ServiceName on port :$port..." -ForegroundColor Yellow
$envCmd = ($s.Env.GetEnumerator() | ForEach-Object { "set ""$($_.Key)=$($_.Value)""" }) -join " && "
$fullCmd = "cmd /c ""$envCmd && `"$($s.Exe)`" >> `"$logDir\$ServiceName.out`" 2>>&1"""

Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{
    CommandLine      = $fullCmd
    CurrentDirectory = $s.WorkDir
} | Out-Null

# 3. Health check
Start-Sleep -Seconds 2
$healthy = $false
for ($i = 0; $i -lt 10; $i++) {
    try {
        $r = Invoke-RestMethod -Uri "http://localhost:$port/healthz" -Method GET -TimeoutSec 1
        $healthy = $true
        break
    } catch {
        Start-Sleep -Seconds 1
    }
}

if ($healthy) {
    Write-Host "[OK] $ServiceName is UP on :$port" -ForegroundColor Green
} else {
    Write-Host "[WARN] $ServiceName failed health check on :$port" -ForegroundColor Red
}
