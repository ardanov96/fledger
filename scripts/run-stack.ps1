# =============================================================================
# FMCG Wallet — Start dev stack (API + web)
# =============================================================================
#
# Starts the API server and web dashboard as background processes.
# Logs to %TEMP%\fmcg-{api,web}.log. Idempotent: kills stale processes first.
#
# Usage:
#   .\scripts\run-stack.ps1            # start both
#   .\scripts\run-stack.ps1 -ApiOnly   # API only
#   .\scripts\run-stack.ps1 -WebOnly   # web only
#   .\scripts\run-stack.ps1 -ApiPort 8081 -WebPort 3001
#
# Stop with: .\scripts\stop-stack.ps1
# =============================================================================

[CmdletBinding()]
param(
    [switch]$ApiOnly,
    [switch]$WebOnly,
    [int]$ApiPort = 8080,
    [int]$WebPort = 3000
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path "$PSScriptRoot\..").Path

function Out-Line {
    param([string]$Msg, [string]$Color = "White")
    Write-Host $Msg -ForegroundColor $Color
}

# Stop any existing processes on these ports
Out-Line "Cleaning up old processes..." "Yellow"
Get-NetTCPConnection -LocalPort $ApiPort -ErrorAction SilentlyContinue | ForEach-Object {
    try { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue } catch {}
}
Get-NetTCPConnection -LocalPort $WebPort -ErrorAction SilentlyContinue | ForEach-Object {
    try { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue } catch {}
}
Start-Sleep -Seconds 1

# Load .env values
$envPath = Join-Path $RepoRoot ".env"
if (-not (Test-Path $envPath)) {
    Out-Line ".env missing. Run setup-everything.ps1 first." "Red"
    exit 1
}

# Read .env into $env vars (for the spawned processes)
Get-Content $envPath | ForEach-Object {
    if ($_ -match "^\s*([A-Z_][A-Z0-9_]*)\s*=\s*(.+?)\s*$" -and $_ -notmatch "^#") {
        $key = $Matches[1]
        $val = $Matches[2]
        # Strip optional surrounding quotes
        $val = $val.Trim('"', "'")
        Set-Item -Path "Env:$key" -Value $val
    }
}

# Ensure DB creds
$env:DB_USER = "fmcg"
$env:DB_PASSWORD = "fmcg_dev_password"

# Start API
if (-not $WebOnly) {
    Out-Line "Starting API on :$ApiPort..." "Cyan"
    $apiLog = Join-Path $env:TEMP "fmcg-api.log"
    $apiErr = Join-Path $env:TEMP "fmcg-api.err"
    $env:APP_PORT = $ApiPort
    $apiProc = Start-Process -FilePath "go" -ArgumentList "run", "$RepoRoot\cmd\api" -WorkingDirectory $RepoRoot -RedirectStandardOutput $apiLog -RedirectStandardError $apiErr -WindowStyle Hidden -PassThru
    Out-Line "  PID: $($apiProc.Id), log: $apiLog" "Gray"
}

# Start web
if (-not $ApiOnly) {
    Out-Line "Starting web on :$WebPort..." "Cyan"
    $env:API_BASE_URL = "http://localhost:$ApiPort"
    $webLog = Join-Path $env:TEMP "fmcg-web.log"
    $webErr = Join-Path $env:TEMP "fmcg-web.err"
    $env:PORT = $WebPort
    $webProc = Start-Process -FilePath "node" -ArgumentList "$RepoRoot\web\server.js" -WorkingDirectory $RepoRoot -RedirectStandardOutput $webLog -RedirectStandardError $webErr -WindowStyle Hidden -PassThru
    Out-Line "  PID: $($webProc.Id), log: $webLog" "Gray"
}

# Wait + verify
Out-Line ""
Out-Line "Waiting for services to come up..." "Yellow"
Start-Sleep -Seconds 4

$allOk = $true
if (-not $WebOnly) {
    try {
        $r = Invoke-WebRequest "http://localhost:$ApiPort/healthz" -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
        if ($r.StatusCode -eq 200) {
            Out-Line "  API :$ApiPort  [OK] /healthz" "Green"
        } else {
            Out-Line "  API :$ApiPort  [FAIL] /healthz → $($r.StatusCode)" "Red"
            $allOk = $false
        }
    } catch {
        Out-Line "  API :$ApiPort  [FAIL] not responding" "Red"
        Out-Line "  Check $env:TEMP\fmcg-api.err" "Yellow"
        $allOk = $false
    }
}

if (-not $ApiOnly) {
    try {
        $r = Invoke-WebRequest "http://localhost:$WebPort/" -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
        if ($r.StatusCode -eq 200) {
            Out-Line "  Web :$WebPort  [OK] /" "Green"
        } else {
            Out-Line "  Web :$WebPort  [FAIL] / → $($r.StatusCode)" "Red"
            $allOk = $false
        }
    } catch {
        Out-Line "  Web :$WebPort  [FAIL] not responding" "Red"
        Out-Line "  Check $env:TEMP\fmcg-web.err" "Yellow"
        $allOk = $false
    }
}

Out-Line ""
if ($allOk) {
    Out-Line "=============================================" "Green"
    Out-Line "Stack running" "Green"
    Out-Line "=============================================" "Green"
    Out-Line ""
    Out-Line "  Dashboard:  http://localhost:$WebPort" "Cyan"
    Out-Line "  API:        http://localhost:$ApiPort" "Cyan"
    Out-Line "  Healthz:    http://localhost:$ApiPort/healthz" "Cyan"
    Out-Line "  Readyz:     http://localhost:$ApiPort/readyz" "Cyan"
    Out-Line ""
    Out-Line "  Logs: %TEMP%\fmcg-api.log + fmcg-web.log" "Gray"
    Out-Line "  Stop:  .\scripts\stop-stack.ps1" "Gray"
} else {
    Out-Line "=============================================" "Red"
    Out-Line "Stack started but some services failed" "Red"
    Out-Line "=============================================" "Red"
    Out-Line "Check logs: %TEMP%\fmcg-*.err" "Yellow"
    exit 1
}
