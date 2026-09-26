# =============================================================================
# FMCG Wallet - Verify setup state (read-only, no changes)
# =============================================================================
#
# Checks all prerequisites and current state. Exits 0 if ready to run,
# non-zero otherwise. Useful for CI / pre-flight / AI agent validation.
#
# Usage:
#   .\scripts\verify-setup.ps1            # human-readable output
#   .\scripts\verify-setup.ps1 -Json      # JSON output for AI agents
#
# Exit codes:
#   0 = ready
#   1 = missing tool
#   2 = setup incomplete
#   64 = fatal (not in repo root)
# =============================================================================

[CmdletBinding()]
param([switch]$Json)

$ErrorActionPreference = "Continue"
$RepoRoot = (Resolve-Path "$PSScriptRoot\..").Path
$JsonMode = $Json.IsPresent

$Checks = @()

function AddCheck {
    param([string]$Name, [string]$Status, [string]$Detail = "")
    $script:Checks += @{ name = $Name; status = $Status; detail = $Detail }
}

function OutLine {
    param([string]$Msg, [string]$Level = "info")
    if ($JsonMode) {
        $ts = (Get-Date).ToString("o")
        $safeMsg = $Msg -replace '"', '\"'
        Write-Output ('{"ts":"' + $ts + '","level":"' + $Level + '","msg":"' + $safeMsg + '"}')
    } else {
        switch ($Level) {
            "ok"   { Write-Host "  [OK]   $Msg" -ForegroundColor Green }
            "warn" { Write-Host "  [WARN] $Msg" -ForegroundColor Yellow }
            "err"  { Write-Host "  [FAIL] $Msg" -ForegroundColor Red }
            default { Write-Host "  $Msg" }
        }
    }
}

function Pass($name, $detail = "") { AddCheck $name "PASS" $detail; OutLine $name "ok" }
function Warn($name, $detail = "") { AddCheck $name "WARN" $detail; OutLine "$name - $detail" "warn" }
function Fail($name, $detail = "") { AddCheck $name "FAIL" $detail; OutLine "$name - $detail" "err" }

# Repo check
if (-not (Test-Path (Join-Path $RepoRoot "go.mod"))) {
    if ($JsonMode) {
        Write-Output ('{"error":"go.mod not found","path":"' + $RepoRoot + '"}')
    } else {
        Write-Host "go.mod not found at $RepoRoot" -ForegroundColor Red
        Write-Host "cd to repo root first" -ForegroundColor Red
    }
    exit 64
}

Write-Host ""
Write-Host "=== FMCG Wallet - Setup verification ===" -ForegroundColor Cyan
Write-Host "Repo: $RepoRoot"
Write-Host ""

# --- Tools ---
Write-Host "Tools:" -ForegroundColor Yellow

# Go
try {
    $v = (& go version) 2>&1 | Select-Object -First 1
    if ($v) { Pass "go installed" $v } else { Fail "go" "not found" }
} catch { Fail "go" "not found (install from https://go.dev/dl/)" }

# Node
try {
    $v = (& node --version) 2>&1 | Select-Object -First 1
    if ($v) { Pass "node installed" $v } else { Fail "node" "not found" }
} catch { Fail "node" "not found (install from https://nodejs.org/)" }

# curl (preferred for API testing)
try {
    $v = (& curl.exe --version) 2>&1 | Select-Object -First 1
    if ($v) { Pass "curl installed" "" } else { Warn "curl" "not found - PowerShell Invoke-WebRequest will be used instead" }
} catch { Warn "curl" "not found" }

# --- Postgres ---
Write-Host ""
Write-Host "PostgreSQL:" -ForegroundColor Yellow

$pgPath = "C:\Program Files\PostgreSQL\17"
$psqlPath = Join-Path $pgPath "bin\psql.exe"
if (Test-Path $psqlPath) {
    Pass "psql found" $psqlPath
} else {
    Fail "psql" "not found at $psqlPath (install PostgreSQL 16+)"
}

$svc = Get-Service postgresql-x64-17 -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -eq "Running") {
    Pass "Postgres service" "Running"
} else {
    $st = if ($svc) { $svc.Status } else { "not found" }
    Fail "Postgres service" "$st - start with: Start-Service postgresql-x64-17"
}

# Test connection as postgres
$pgConnected = $false
$env:PGPASSWORD = "Desmone8327"
try {
    $r = & "$psqlPath" -U postgres -h 127.0.0.1 -w -c "SELECT 1" 2>&1 | Out-String
    if ($LASTEXITCODE -eq 0) {
        Pass "postgres connect" "OK (postgres:Desmone8327)"
        $pgConnected = $true
    } else {
        Fail "postgres connect" "failed - check password"
    }
} catch { Fail "postgres connect" "exception" }

# Test fmcg user + db
if ($pgConnected) {
    $env:PGPASSWORD = "fmcg_dev_password"
    try {
        $r = & "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -c "SELECT 1" 2>&1 | Out-String
        if ($LASTEXITCODE -eq 0) {
            Pass "fmcg user / db" "OK"
        } else {
            Fail "fmcg user / db" "run setup-everything.ps1 phase 2"
        }
    } catch { Fail "fmcg user / db" "exception" }
}

# --- .env ---
Write-Host ""
Write-Host ".env file:" -ForegroundColor Yellow

$envPath = Join-Path $RepoRoot ".env"
if (Test-Path $envPath) {
    Pass "env file" "exists"
    $envContent = Get-Content $envPath -Raw
    $required = @{
        "DB_HOST"     = "localhost"
        "DB_PORT"     = "5432"
        "DB_NAME"     = "fmcg_wallet"
        "DB_USER"     = "fmcg"
        "DB_PASSWORD" = ""
        "JWT_SECRET"  = ""
        "APP_PORT"    = ""
    }
    foreach ($k in $required.Keys) {
        $pattern = "(?m)^" + [regex]::Escape($k) + "=(.+)$"
        if ($envContent -match $pattern) {
            $val = $Matches[1].Trim().Trim('"', "'")
            $preview = if ($val.Length -gt 20) { $val.Substring(0, 20) + "..." } else { $val }
            if ($k -eq "JWT_SECRET" -and $val.Length -lt 32) {
                Fail "env_$k" "too short ($($val.Length) chars, need 32+)"
            } elseif ($val -eq "") {
                Fail "env_$k" "empty"
            } else {
                Pass "env_$k" $preview
            }
        } else {
            Fail "env_$k" "missing"
        }
    }
} else {
    Fail "env_file" "missing - run setup-everything.ps1"
}

# --- Migrations ---
Write-Host ""
Write-Host "Migrations:" -ForegroundColor Yellow

if (Test-Path (Join-Path $RepoRoot "cmd/migrator")) {
    Pass "migrator binary" "exists"
} else {
    Fail "migrator binary" "missing"
}

if ((Test-Path $envPath) -and $pgConnected) {
    $env:PGPASSWORD = "fmcg_dev_password"
    try {
        $r = (& "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -tA -c "SELECT MAX(version) FROM schema_migrations;" 2>&1).Trim()
        if ($r -match "^\d+$") {
            $ver = [int]$r
            Pass "migrations applied" "version=$ver"
            if ($ver -lt 18) {
                Warn "migrations count" "expected 18, got $ver - run setup-everything.ps1 phase 4"
            }
        } else {
            Fail "migrations applied" "no version found"
        }
    } catch { Fail "migrations applied" "query failed" }
}

# --- Seed data ---
Write-Host ""
Write-Host "Seed data:" -ForegroundColor Yellow

if ((Test-Path $envPath) -and $pgConnected) {
    $env:PGPASSWORD = "fmcg_dev_password"
    $queries = @{
        "currencies"       = "SELECT COUNT(*) FROM currencies"
        "users"            = "SELECT COUNT(*) FROM user_credentials"
        "accounts"         = "SELECT COUNT(*) FROM accounts"
        "periods"          = "SELECT COUNT(*) FROM accounting_periods"
        "fx_rates"         = "SELECT COUNT(*) FROM fx_rates"
        "invoices"         = "SELECT COUNT(*) FROM invoices"
        "invoice_payments" = "SELECT COUNT(*) FROM invoice_payments"
    }
    foreach ($k in $queries.Keys) {
        try {
            $r = (& "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -tA -c $queries[$k] 2>&1).Trim()
            if ($r -match "^\d+$") {
                $n = [int]$r
                if ($n -gt 0) {
                    Pass "seed_$k" "$n rows"
                } else {
                    Fail "seed_$k" "0 rows - run setup-everything.ps1 phase 6"
                }
            } else {
                Fail "seed_$k" "query: $r"
            }
        } catch { Fail "seed_$k" "exception" }
    }
}

# --- Summary ---
Write-Host ""
Write-Host "=============================================" -ForegroundColor Cyan
$passCount = ($Checks | Where-Object { $_.status -eq "PASS" }).Count
$warnCount = ($Checks | Where-Object { $_.status -eq "WARN" }).Count
$failCount = ($Checks | Where-Object { $_.status -eq "FAIL" }).Count
$summaryColor = if ($failCount -gt 0) { "Red" } elseif ($warnCount -gt 0) { "Yellow" } else { "Green" }
Write-Host "Summary: $passCount PASS, $warnCount WARN, $failCount FAIL" -ForegroundColor $summaryColor

if ($failCount -gt 0) {
    Write-Host ""
    Write-Host "Run .\scripts\setup-everything.ps1 to fix" -ForegroundColor Cyan
    exit 2
} elseif ($warnCount -gt 0) {
    Write-Host ""
    Write-Host "Setup OK with warnings" -ForegroundColor Yellow
    exit 0
} else {
    Write-Host ""
    Write-Host "Ready to run! Start with: .\scripts\run-stack.ps1" -ForegroundColor Green
    exit 0
}
