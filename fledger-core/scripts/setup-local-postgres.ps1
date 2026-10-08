# =============================================================================
# FMCG Wallet - Postgres setup script for local development
# =============================================================================
# Purpose: Reset postgres password, then create fmcg user + fmcg_wallet db
# Requires: Run as Administrator (auto-elevated by .bat wrapper)
#
# What it does (7 steps, all logged to file):
#   1. Pre-flight checks (psql, pg_hba.conf, service)
#   2. Backup current pg_hba.conf
#   3. Add temporary trust entry for postgres user
#   4. Restart Postgres service
#   5. Reset postgres superuser password
#   6. Create fmcg user + fmcg_wallet db + grants
#   7. Restore pg_hba.conf, restart service, test connection
#
# All output is duplicated to: %TEMP%\fmcg-postgres-setup.log
# =============================================================================

[CmdletBinding()]
param(
    [string]$NewPostgresPassword = "Desmone8327",
    [string]$FmcgPassword = "fmcg_dev_password"
)

$ErrorActionPreference = "Continue"
$pg = "C:\Program Files\PostgreSQL\17"
$pgData = "$pg\data"
$pgHba = "$pgData\pg_hba.conf"
$pgHbaBak = "$pgData\pg_hba.conf.fmcg-bak"
$serviceName = "postgresql-x64-17"
$logFile = Join-Path $env:TEMP "fmcg-postgres-setup.log"

# Force log file to start fresh (ASCII, no BOM — BOM confuses some readers)
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($logFile, "", $utf8NoBom)

function Log($msg, $color = "White") {
    $line = "[$(Get-Date -Format 'HH:mm:ss')] $msg"
    Write-Host $line -ForegroundColor $color
    Add-Content -Path $logFile -Value $line
}

Log "=== FMCG Wallet Postgres setup started ===" "Cyan"
Log "Log file: $logFile"
Log "Postgres install path: $pg"
Log "Target postgres password: $NewPostgresPassword"
Log "Target fmcg password:    $FmcgPassword"

# --- Pre-flight checks ---
Log "[0/7] Pre-flight checks..."
if (-not (Test-Path "$pg\bin\psql.exe")) {
    Log "  ERROR: psql.exe not found at $pg\bin\psql.exe" "Red"
    pause; exit 1
}
if (-not (Test-Path $pgHba)) {
    Log "  ERROR: pg_hba.conf not found at $pgHba" "Red"
    pause; exit 1
}
$svc = Get-Service $serviceName -ErrorAction SilentlyContinue
if (-not $svc) {
    Log "  ERROR: service '$serviceName' not found" "Red"
    pause; exit 1
}
Log "  psql: OK, pg_hba.conf: OK, service '$serviceName': $($svc.Status)" "Green"

# --- Step 1: Backup pg_hba.conf ---
# Backup as ASCII-clean to avoid propagating a corrupt BOM through the backup.
Log "[1/7] Backing up pg_hba.conf..."
$bakContent = Get-Content $pgHba -Raw
$ascii = New-Object System.Text.ASCIIEncoding
[System.IO.File]::WriteAllText($pgHbaBak, $bakContent, $ascii)
Log "  Backup at: $pgHbaBak (ASCII-clean)" "Green"

# --- Step 2: Add trust entry (INSERT at top, after comment header) ---
# IMPORTANT: pg_hba.conf is first-match-wins. Existing rules like
# `host all all 127.0.0.1/32 scram-sha-256` match BEFORE our trust
# entry if we just append. We must INSERT after the comment header
# but BEFORE the existing rules.
Log "[2/7] Adding temporary trust entry..."
$trustEntries = @"
# TEMP: trust for postgres from localhost (added by fmcg setup script)
host all postgres 127.0.0.1/32 trust
host all postgres ::1/128 trust
"@
$content = Get-Content $pgHba -Raw
# Insert right after the line that ends with "actual configuration here"
# (i.e. after the comment block). Simpler: insert after the first
# non-comment, non-blank rule marker line.
$lines = Get-Content $pgHba
$inserted = $false
$out = New-Object System.Collections.Generic.List[string]
foreach ($line in $lines) {
    $out.Add($line)
    # Insert right BEFORE the first existing "host" or "local" rule
    if (-not $inserted -and ($line -match '^\s*(host|local)\s')) {
        $out.Add($trustEntries.TrimEnd("`r","`n"))
        $inserted = $true
    }
}
if (-not $inserted) {
    # Fallback: prepend
    $prependContent = $trustEntries + $content
    $content = $prependContent
} else {
    $content = $out -join "`n"
}
# Preserve trailing newline
if (-not $content.EndsWith("`n")) { $content += "`n" }
# IMPORTANT: write as ASCII (no BOM). A UTF-8 BOM (EF BB BF) at line 1 is
# interpreted by Postgres as 3 separate bytes = "invalid connection type"
# which crashes the server on startup.
$ascii = New-Object System.Text.ASCIIEncoding
[System.IO.File]::WriteAllText($pgHba, $content, $ascii)
Log "  Trust entry inserted BEFORE existing rules" "Green"

# --- Step 3: Restart Postgres ---
Log "[3/7] Restarting Postgres service..."
try {
    Restart-Service $serviceName -Force -ErrorAction Stop
    Start-Sleep -Seconds 3
    $svc.Refresh()
    Log "  Service status: $($svc.Status)" "Green"
} catch {
    Log "  ERROR restarting service: $_" "Red"
    Log "  Restoring pg_hba.conf before exit..."
    if (Test-Path $pgHbaBak) { Move-Item $pgHbaBak $pgHba -Force }
    pause; exit 1
}

# --- Step 4: Reset postgres password ---
Log "[4/7] Resetting postgres superuser password..."
$env:PGPASSWORD = ""
$out = & "$pg\bin\psql.exe" -U postgres -h 127.0.0.1 -w -c "ALTER USER postgres WITH PASSWORD '$NewPostgresPassword';" 2>&1
$out | ForEach-Object { Log "  $_" }
if ($LASTEXITCODE -ne 0) {
    Log "  ERROR: ALTER USER failed (exit $LASTEXITCODE)" "Red"
    pause; exit 1
}
Log "  postgres password reset OK" "Green"

# --- Step 5: Create fmcg user + db ---
Log "[5/7] Creating fmcg user + fmcg_wallet db..."

# 5a. fmcg user (create or update password)
# Use two separate statements (one for each branch) with string interpolation.
# Single-quoted here-strings escape single quotes by doubling them which broke
# the SQL; double-quoted strings interpolate $FmcgPassword cleanly.
$userExists = & "$pg\bin\psql.exe" -U postgres -h 127.0.0.1 -w -tA -c "SELECT 1 FROM pg_roles WHERE rolname = 'fmcg'" 2>&1
$env:PGPASSWORD = ""
if ($userExists -match '^1$') {
    Log "  fmcg user exists, updating password..."
    & "$pg\bin\psql.exe" -U postgres -h 127.0.0.1 -w -c "ALTER USER fmcg WITH PASSWORD '$FmcgPassword';" 2>&1 | ForEach-Object { Log "  $_" }
} else {
    Log "  creating fmcg user..."
    & "$pg\bin\psql.exe" -U postgres -h 127.0.0.1 -w -c "CREATE USER fmcg WITH PASSWORD '$FmcgPassword' CREATEDB;" 2>&1 | ForEach-Object { Log "  $_" }
}

# 5b. fmcg_wallet db
$dbExists = & "$pg\bin\psql.exe" -U postgres -h 127.0.0.1 -w -tA -c "SELECT 1 FROM pg_database WHERE datname = 'fmcg_wallet'" 2>&1
if ($dbExists -match "^1$") {
    Log "  fmcg_wallet db already exists" "Yellow"
} else {
    & "$pg\bin\psql.exe" -U postgres -h 127.0.0.1 -w -c "CREATE DATABASE fmcg_wallet OWNER fmcg;" 2>&1 | ForEach-Object { Log "  $_" }
    Log "  fmcg_wallet created" "Green"
}

# 5c. grants
& "$pg\bin\psql.exe" -U postgres -h 127.0.0.1 -w -d fmcg_wallet -c "GRANT ALL ON SCHEMA public TO fmcg; ALTER SCHEMA public OWNER TO fmcg;" 2>&1 | ForEach-Object { Log "  $_" }
Log "  fmcg user + db ready" "Green"

# --- Step 6: Restore pg_hba.conf ---
Log "[6/7] Restoring pg_hba.conf..."
if (Test-Path $pgHbaBak) {
    # Use Copy + Delete to avoid preserving any corrupt encoding from the backup
    $bakContent = Get-Content $pgHbaBak -Raw
    $ascii = New-Object System.Text.ASCIIEncoding
    [System.IO.File]::WriteAllText($pgHba, $bakContent, $ascii)
    Remove-Item $pgHbaBak -Force
    Log "  pg_hba.conf restored (trust entries removed, ASCII-clean)" "Green"
} else {
    Log "  WARNING: backup not found, manual cleanup needed" "Yellow"
}

# --- Step 7: Restart + test ---
Log "[7/7] Restarting Postgres + testing connection as fmcg..."
Restart-Service $serviceName -Force
Start-Sleep -Seconds 3

$env:PGPASSWORD = $FmcgPassword
$testOut = & "$pg\bin\psql.exe" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -c "SELECT current_user || ' @ ' || current_database() AS ok;" 2>&1
$testOut | ForEach-Object { Log "  $_" }

if ($testOut -match "fmcg @ fmcg_wallet") {
    Log "" "Green"
    Log "=========================================" "Green"
    Log "SUCCESS!" "Green"
    Log "  postgres superuser password: $NewPostgresPassword" "Green"
    Log "  fmcg user can connect to fmcg_wallet" "Green"
    Log "=========================================" "Green"
    Log ""
    Log "Next steps (in regular PowerShell, NOT admin):" "Yellow"
    Log "  cd C:\Dev\fledger"
    Log "  go run ./cmd/migrator up"
    Log "  go run ./cmd/api"
} else {
    Log "" "Red"
    Log "FAILED - see output above" "Red"
    Log "Full log: $logFile" "Red"
}

Log ""
Log "Press any key to close..."
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
