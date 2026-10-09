# =============================================================================
# FLEDGER FLEET — One-shot local setup
# 1. Ensures the fleet database exists.
# 2. Runs migrations against it.
# 3. (Optional) seeds demo vehicles + drivers.
# =============================================================================

$ErrorActionPreference = "Stop"

$env:PGPASSWORD = "fmcg_dev_password"
$PSQL = "C:\Program Files\PostgreSQL\17\bin\psql.exe"
$DB_USER = "fmcg"
$DB_NAME = "fledger_fleet"
$FLWROOT = Split-Path -Parent $PSScriptRoot
$MIGDIR = Join-Path $FLWROOT "migrations"

Write-Host "[1/4] Creating database $DB_NAME (if missing)..."
& $PSQL -h localhost -U $DB_USER -d postgres -c "CREATE DATABASE $DB_NAME OWNER $DB_USER;" 2>&1 | Out-Null

Write-Host "[2/4] Applying migrations from $MIGDIR ..."
Get-ChildItem -Path $MIGDIR -Filter *.sql | Sort-Object Name | ForEach-Object {
    Write-Host "  -> $($_.Name)"
    & $PSQL -h localhost -U $DB_USER -d $DB_NAME -f $_.FullName 2>&1 | Out-Null
}

Write-Host "[3/4] Verifying schema..."
& $PSQL -h localhost -U $DB_USER -d $DB_NAME -c "\dt fleet_*" 2>&1 | Out-String | Write-Host

Write-Host "[4/4] Done. You can now run: go run ./cmd/api"