# =============================================================================
# FMCG Wallet - Interactive psql shell (with fmcg_wallet defaults)
# =============================================================================
#
# Wrapper around psql with sensible defaults for the local dev DB.
# Sets PGPASSWORD from .env automatically; no need to remember it.
#
# Usage:
#   .\scripts\db-shell.ps1                  # interactive, user fmcg
#   .\scripts\db-shell.ps1 -AsPostgres      # as postgres superuser
#   .\scripts\db-shell.ps1 -Query "SELECT 1"
#   .\scripts\db-shell.ps1 -Query "\dt"      # list tables
#   .\scripts\db-shell.ps1 -File myscript.sql
#
# On startup prints a banner + table list so you can start exploring immediately.
# Type \q to quit, \? for psql help.
# =============================================================================

[CmdletBinding()]
param(
    [switch]$AsPostgres,
    [string]$Query,
    [string]$File,
    [string]$DbHost = "127.0.0.1",
    [int]$DbPort = 5432,
    [string]$DbName = "fmcg_wallet"
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path "$PSScriptRoot\..").Path
$psqlPath = "C:\Program Files\PostgreSQL\17\bin\psql.exe"

if (-not (Test-Path $psqlPath)) {
    Write-Host "psql.exe not found at $psqlPath" -ForegroundColor Red
    exit 1
}

# Load .env to get passwords
$envPath = Join-Path $RepoRoot ".env"
if (Test-Path $envPath) {
    Get-Content $envPath | ForEach-Object {
        if ($_ -match "^\s*([A-Z_][A-Z0-9_]*)\s*=\s*(.+?)\s*$" -and $_ -notmatch "^#") {
            $key = $Matches[1]
            $val = $Matches[2].Trim('"', "'")
            if ($key -in @("DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD", "POSTGRES_PASSWORD")) {
                Set-Item -Path "Env:$key" -Value $val -ErrorAction SilentlyContinue
            }
        }
    }
}

# Set PGPASSWORD
if ($AsPostgres) {
    $env:PGPASSWORD = if ($env:POSTGRES_PASSWORD) { $env:POSTGRES_PASSWORD } else { "Desmone8327" }
    $user = "postgres"
} else {
    $env:PGPASSWORD = "fmcg_dev_password"
    $user = "fmcg"
}

# Build psql args (using semicolons instead of commas to avoid PowerShell array issue)
$psqlArgs = @(
    "-U"; $user;
    "-h"; $DbHost;
    "-p"; "$DbPort";
    "-d"; $DbName;
    "--no-psqlrc";
    "--set"; "PROMPT1=%n@%m/%> %R%X ";
    "--set"; "PROMPT2=%n@%m/%> %R%X "
)

# Non-interactive: -Query or -File
if ($Query) {
    Write-Host "[$user@$DbHost/$DbName] $Query" -ForegroundColor Gray
    & $psqlPath @psqlArgs -c $Query
    exit $LASTEXITCODE
}
if ($File) {
    Write-Host "[$user@$DbHost/$DbName] Executing file: $File" -ForegroundColor Gray
    & $psqlPath @psqlArgs -f $File
    exit $LASTEXITCODE
}

# Interactive mode: print banner + auto-execute helpful queries
Write-Host ""
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "  psql -U $user -d $DbName -h $DbHost" -ForegroundColor Cyan
Write-Host "  Type \q to quit, \? for psql help" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""

# Init queries: show banner + tables. Use a here-string to avoid quote conflicts.
$initSql = @"
\echo 'Database: $DbName as $user@$DbHost'
\echo ''
\echo 'Tables (top 15 by row count):'
SELECT schemaname || '.' || tablename AS table_name, n_live_tup AS rows
   FROM pg_stat_user_tables
   ORDER BY n_live_tup DESC LIMIT 15;
\echo ''
\echo 'Ready. Try: \dt; SELECT 1; \q'
"@

# Run init (don't fail shell if stats views missing)
& $psqlPath @psqlArgs -v ON_ERROR_STOP=0 -c $initSql 2>$null

# Hand off to interactive psql
& $psqlPath @psqlArgs
exit $LASTEXITCODE
