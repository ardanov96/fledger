# =============================================================================
# FMCG Wallet - Start fresh (DESTRUCTIVE: drops + recreates DB)
# =============================================================================
#
# Wipes fmcg_wallet database and rebuilds from scratch:
#   1. Drop fmcg_wallet db
#   2. Recreate as postgres superuser
#   3. Run all 18 migrations
#   4. Disable RLS on refresh_tokens (dev workaround)
#   5. Re-seed demo data
#
# USE WITH CARE - all data is destroyed. Use -Force to skip confirmation.
#
# Usage:
#   .\scripts\start-fresh.ps1                  # prompts for confirmation
#   .\scripts\start-fresh.ps1 -Force          # skip confirmation (CI/AI agent)
#   .\scripts\start-fresh.ps1 -Json          # JSON progress output
#
# Exit codes:
#   0 = success
#   1 = cancelled (user said no)
#   2 = setup failed
# =============================================================================

[CmdletBinding()]
param(
    [switch]$Force,
    [switch]$Json,
    [string]$PostgresPassword
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path "$PSScriptRoot\..").Path
$pgPath = "C:\Program Files\PostgreSQL\17"
$psqlPath = Join-Path $pgPath "bin\psql.exe"
$JsonMode = $Json.IsPresent

function Out-Line {
    param([string]$Msg, [string]$Level = "info")
    if ($JsonMode) {
        $ts = (Get-Date).ToString("o")
        Write-Output ('{"ts":"' + $ts + '","level":"' + $Level + '","msg":"' + ($Msg -replace '"', '\"') + '"}')
    } else {
        switch ($Level) {
            "warn" { Write-Host "  [WARN] $Msg" -ForegroundColor Yellow }
            "err"  { Write-Host "  [ERR]  $Msg" -ForegroundColor Red }
            "ok"   { Write-Host "  [OK]   $Msg" -ForegroundColor Green }
            "info" { Write-Host "  [..]   $Msg" -ForegroundColor Gray }
            default { Write-Host "  $Msg" }
        }
    }
}

if (-not $PostgresPassword) { $PostgresPassword = "Desmone8327" }

# ============================================================================
# Confirmation prompt (unless -Force)
# ============================================================================
if (-not $Force) {
    Write-Host ""
    Write-Host "=================================================" -ForegroundColor Red
    Write-Host "DESTRUCTIVE: ALL DATA IN fmcg_wallet WILL BE WIPED" -ForegroundColor Red
    Write-Host "=================================================" -ForegroundColor Red
    Write-Host ""
    Write-Host "This will:"
    Write-Host "  - DROP DATABASE fmcg_wallet (all tables, data, custom migrations)"
    Write-Host "  - Recreate empty database"
    Write-Host "  - Re-run all 18 migrations"
    Write-Host "  - Re-seed demo data (2 users, 8 accounts, 3 invoices, etc.)"
    Write-Host ""
    Write-Host "Outbox events, aging snapshots, FX rates, transactions,"
    Write-Host "invoices, payments, audit logs - ALL WILL BE LOST."
    Write-Host ""

    $confirm = Read-Host "Type 'yes' to confirm (or anything else to cancel)"
    if ($confirm -ne "yes") {
        Out-Line "Cancelled by user" "warn"
        exit 1
    }
    Write-Host ""
}

Out-Line "Step 1/5: Drop fmcg_wallet database + app_admin role" "info"
$env:PGPASSWORD = $PostgresPassword
# Drop database (transfers ownership to postgres)
$null = & "$psqlPath" -U postgres -h 127.0.0.1 -w -c "DROP DATABASE IF EXISTS fmcg_wallet;" 2>&1
# Drop app_admin role too - migration 000015 creates it WITHOUT IF NOT EXISTS
# so it must be removed to allow fresh re-create. Also drop any owned objects.
$null = & "$psqlPath" -U postgres -h 127.0.0.1 -w -c "REASSIGN OWNED BY app_admin TO postgres; DROP OWNED BY app_admin; DROP ROLE IF EXISTS app_admin;" 2>&1
# Note: fmcg user is NOT dropped here - it's created by setup-local-postgres.ps1
# (not migration), so we preserve it with password 'fmcg_dev_password' across fresh-starts.
Out-Line "  Database + app_admin role dropped" "ok"

Out-Line "Step 2/5: Recreate fmcg_wallet + grants" "info"
$null = & "$psqlPath" -U postgres -h 127.0.0.1 -w -c "CREATE DATABASE fmcg_wallet OWNER fmcg;" 2>&1
$null = & "$psqlPath" -U postgres -h 127.0.0.1 -w -c "GRANT ALL ON SCHEMA public TO fmcg; ALTER SCHEMA public OWNER TO fmcg;" 2>&1
Out-Line "  Database recreated + grants set" "ok"

Out-Line "Step 3/5: Run all 18 migrations (as postgres)" "info"
$env:DB_USER = "postgres"
$env:DB_PASSWORD = $PostgresPassword
$migOut = & go run "$RepoRoot\cmd\migrator" up 2>&1
if ($LASTEXITCODE -ne 0) {
    Out-Line "Migration failed:" "err"
    $migOut | ForEach-Object { Out-Line "  $_" "err" }
    exit 2
}
Out-Line "  Migrations applied (version=18)" "ok"

Out-Line "Step 4/5: Disable RLS on refresh_tokens (dev workaround)" "info"
# Need to connect as postgres for ALTER TABLE; use -v with explicit password
$null = & "$psqlPath" -U postgres -h 127.0.0.1 -w -d fmcg_wallet -c "ALTER TABLE refresh_tokens DISABLE ROW LEVEL SECURITY;" 2>&1
if ($LASTEXITCODE -eq 0) {
    Out-Line "  RLS disabled" "ok"
} else {
    Out-Line "  Failed (non-fatal)" "warn"
}

Out-Line "Step 5/5: Re-seed demo data" "info"
# Set fmcg password for seed script (it expects PGPASSWORD=fmcg_dev_password)
$env:PGPASSWORD = "fmcg_dev_password"
$seedScript = Join-Path $RepoRoot "scripts\seed-local-dev-data.ps1"
if (-not (Test-Path $seedScript)) {
    Out-Line "Missing: $seedScript" "err"
    exit 2
}
& $seedScript 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) {
    Out-Line "Seed failed (exit $LASTEXITCODE)" "err"
    exit 2
}
Out-Line "  Demo data seeded" "ok"

Write-Host ""
Write-Host "=============================================" -ForegroundColor Green
Write-Host "Fresh start complete" -ForegroundColor Green
Write-Host "=============================================" -ForegroundColor Green
Write-Host ""
Write-Host "Run:  .\scripts\run-stack.ps1" -ForegroundColor Cyan
Write-Host "Or:   go run ./cmd/api (terminal 1)" -ForegroundColor Cyan
Write-Host "      node web/server.js (terminal 2)" -ForegroundColor Cyan

exit 0
