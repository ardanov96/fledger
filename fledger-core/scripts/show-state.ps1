# =============================================================================
# FMCG Wallet - Show detailed state (read-only, no changes)
# =============================================================================
#
# Inspects current runtime + database state in detail. Useful for:
#   - Debugging ("what's running, what failed")
#   - AI agent context (full snapshot of system state)
#   - Operational health checks
#
# Usage:
#   .\scripts\show-state.ps1                  # human-readable
#   .\scripts\show-state.ps1 -Json            # structured JSON for AI agents
#   .\scripts\show-state.ps1 -ApiPort 8081   # custom port
# =============================================================================

[CmdletBinding()]
param(
    [switch]$Json,
    [int]$ApiPort = 8080,
    [int]$WebPort = 3000
)

$ErrorActionPreference = "Continue"
$RepoRoot = (Resolve-Path "$PSScriptRoot\..").Path
$pgPath = "C:\Program Files\PostgreSQL\17"
$psqlPath = Join-Path $pgPath "bin\psql.exe"
$JsonMode = $Json.IsPresent

# Collect state as object
$state = @{
    generated_at = (Get-Date).ToString("o")
    repo_root    = $RepoRoot
    processes    = @()
    api          = $null
    web          = $null
    database     = $null
    recent_logs  = @()
}

function Out-Line {
    param([string]$Msg, [string]$Color = "White")
    if ($JsonMode) { return } else { Write-Host $Msg -ForegroundColor $Color }
}

function Section {
    param([string]$Title)
    if (-not $JsonMode) {
        Write-Host ""
        Write-Host "=== $Title ===" -ForegroundColor Cyan
    }
}

# ============================================================================
# Running processes (API + web)
# ============================================================================
Section "Running processes"
foreach ($port in @($ApiPort, $WebPort)) {
    $conn = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    $procInfo = $null
    if ($conn -and $conn.OwningProcess -gt 0) {
        try {
            $proc = Get-Process -Id $conn.OwningProcess -ErrorAction Stop
            $procInfo = @{
                port       = $port
                pid        = $proc.Id
                name       = $proc.ProcessName
                started    = $proc.StartTime.ToString("o")
                cpu_sec    = [math]::Round($proc.CPU, 1)
                mem_mb     = [math]::Round($proc.WorkingSet64 / 1MB, 1)
            }
            Out-Line "  :$port  $($proc.ProcessName) (PID $($proc.Id))  CPU: $($procInfo.cpu_sec)s  Mem: $($procInfo.mem_mb) MB" "Green"
        } catch {
            $procInfo = @{ port = $port; pid = $conn.OwningProcess; status = "not accessible" }
            Out-Line "  :$port  (process not accessible)" "Yellow"
        }
    } else {
        $procInfo = @{ port = $port; status = "not running" }
        Out-Line "  :$port  not running" "Gray"
    }
    if ($port -eq $ApiPort) { $state.api = $procInfo } else { $state.web = $procInfo }
}

# ============================================================================
# API health (if running)
# ============================================================================
Section "API health"
$apiOk = $false
try {
    $r = Invoke-WebRequest "http://localhost:$ApiPort/healthz" -UseBasicParsing -TimeoutSec 3 -ErrorAction Stop
    if ($r.StatusCode -eq 200) {
        Out-Line "  /healthz : 200 OK" "Green"
        $apiOk = $true
    } else {
        Out-Line "  /healthz : $($r.StatusCode)" "Yellow"
    }
} catch { Out-Line "  /healthz : not responding" "Gray" }

try {
    $r = Invoke-WebRequest "http://localhost:$ApiPort/readyz" -UseBasicParsing -TimeoutSec 3 -ErrorAction Stop
    $readyzData = ($r.Content | ConvertFrom-Json).data
    Out-Line "  /readyz : status=$($readyzData.status)" "Green"
    foreach ($k in $readyzData.checks.Keys) {
        Out-Line "    $k : $($readyzData.checks[$k])" "Gray"
    }
    $state.api_readyz = $readyzData
} catch { Out-Line "  /readyz : not responding" "Gray" }

# ============================================================================
# Database state
# ============================================================================
Section "Database state"
$env:PGPASSWORD = "fmcg_dev_password"

# Connection + size
try {
    $sizeR = & "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -tA -c "SELECT pg_size_pretty(pg_database_size('fmcg_wallet'));" 2>&1
    Out-Line "  DB size: $sizeR" "Gray"
    $state.db_size = $sizeR

    # Migrations
    $verR = (& "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -tA -c "SELECT MAX(version) FROM schema_migrations;" 2>&1).Trim()
    Out-Line "  Latest migration: version=$verR" "Gray"
    $state.latest_migration = $verR
} catch { Out-Line "  DB query failed" "Red" }

# Row counts
$tableCounts = @()
$tables = @(
    "currencies", "user_credentials", "accounts", "accounting_periods",
    "fx_rates", "invoices", "invoice_payments", "transactions",
    "ledger_entries", "outbox_events", "aging_snapshots", "audit_logs"
)
Out-Line "  Row counts:" "Yellow"
foreach ($t in $tables) {
    try {
        $r = (& "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -tA -c "SELECT COUNT(*) FROM $t" 2>&1).Trim()
        if ($r -match "^\d+$") {
            Out-Line "    $($t.PadRight(22)) $($r.PadLeft(6)) rows" "Gray"
            $tableCounts += @{ table = $t; rows = [int]$r }
        }
    } catch { Out-Line "    $($t.PadRight(22)) (query failed)" "Red" }
}
$state.table_counts = $tableCounts

# Outbox unprocessed (if table exists)
try {
    $unpub = (& "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -tA -c "SELECT COUNT(*) FROM outbox_events WHERE published_at IS NULL" 2>&1).Trim()
    if ($unpub -match "^\d+$") {
        $n = [int]$unpub
        $color = if ($n -gt 100) { "Red" } elseif ($n -gt 10) { "Yellow" } else { "Green" }
        Out-Line "  Unpublished outbox events: $n" $color
        $state.unpublished_outbox = $n
    }
} catch { }

# RLS status
try {
    $rls = (& "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -tA -c "SELECT relname, rowsecurity FROM pg_class WHERE relname = 'refresh_tokens'" 2>&1).Trim()
    Out-Line "  RLS on refresh_tokens: $rls" "Gray"
} catch { }

# Active queries
try {
    $activeQ = & "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -c "SELECT pid, state, query_start, LEFT(query, 60) FROM pg_stat_activity WHERE state != 'idle' AND datname='fmcg_wallet' LIMIT 5" 2>&1
    if ($activeQ -notmatch "0 rows") {
        Out-Line "  Active queries:" "Yellow"
        $activeQ | Select-Object -Skip 2 | ForEach-Object {
            if ($_.Trim()) { Out-Line "    $_" "Gray" }
        }
    } else {
        Out-Line "  Active queries: none" "Gray"
    }
} catch { }

# ============================================================================
# Recent log tail (errors only)
# ============================================================================
Section "Recent log tails"
$logFiles = @(
    @{ name = "API";   path = Join-Path $env:TEMP "fmcg-api.log" }
    @{ name = "Web";   path = Join-Path $env:TEMP "fmcg-web.log" }
    @{ name = "Setup"; path = Join-Path $env:TEMP "fmcg-postgres-setup.log" }
)
foreach ($lf in $logFiles) {
    if (Test-Path $lf.path) {
        $tail = Get-Content $lf.path -Tail 50 -ErrorAction SilentlyContinue
        $errors = $tail | Where-Object { $_ -match "(?i)error|panic|fatal" } | Select-Object -Last 5
        if ($errors.Count -gt 0) {
            Out-Line "  $($lf.name) (recent errors):" "Yellow"
            $errors | ForEach-Object { Out-Line "    $_" "Red" }
        } else {
            Out-Line "  $($lf.name): no recent errors (log: $($lf.path))" "Gray"
        }
    } else {
        Out-Line "  $($lf.name): log not found ($($lf.path))" "Gray"
    }
}

# ============================================================================
# Recent activity (last 5 records across key tables)
# ============================================================================
Section "Recent activity"
$queries = @(
    @{ table = "audit_logs";  filter = "" },
    @{ table = "transactions"; filter = "" }
)
foreach ($q in $queries) {
    try {
        $r = & "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -c "SELECT id, created_at FROM $($q.table) ORDER BY created_at DESC LIMIT 5" 2>&1
        Out-Line "  Last 5 $($q.table):" "Yellow"
        $r | Select-Object -Skip 2 | Where-Object { $_.Trim() -and $_ -notmatch "rows)" } | ForEach-Object {
            Out-Line "    $($_.Trim())" "Gray"
        }
    } catch { }
}

# ============================================================================
# JSON output
# ============================================================================
if ($JsonMode) {
    # Convert all properties to clean JSON
    $state | ConvertTo-Json -Depth 5 -Compress
}
