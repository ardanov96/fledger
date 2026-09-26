# =============================================================================
# FMCG Wallet — Master setup script (idempotent, AI-agent friendly)
# =============================================================================
#
# Run once from a fresh checkout to get a working local dev environment.
# Idempotent: safe to re-run. Will detect existing state and skip done phases.
#
# Usage:
#   .\scripts\setup-everything.ps1                       # full setup (needs admin UAC for Postgres phase)
#   .\scripts\setup-everything.ps1 -CheckOnly           # dry-run: report state, no changes
#   .\scripts\setup-everything.ps1 -SkipPhase 2         # skip phase 2 (Postgres setup)
#   .\scripts\setup-everything.ps1 -PostgresPassword X  # provide postgres password non-interactively
#   .\scripts\setup-everything.ps1 -ApiOnly             # only run migrations + seed (skip Postgres setup)
#   .\scripts\setup-everything.ps1 -Json                # output progress as JSON lines (for AI agents)
#
# Phases:
#   1. Preflight       — verify Go, Node, Postgres, git, write perms
#   2. Postgres setup  — modify pg_hba.conf, restart, create user/db (NEEDS ADMIN)
#   3. Env setup       — copy .env.example → .env, generate JWT secret
#   4. Migrations      — apply all 18 schema migrations
#   5. RLS workaround  — disable RLS on refresh_tokens (dev-only)
#   6. Seed data       — populate demo accounts/invoices/etc.
#   7. Self-test       — start API + web, hit endpoints, verify responses
#
# Exit codes:
#   0 = success
#   1 = preflight failed
#   2 = setup phase failed
#   3 = self-test failed
#   64 = preflight missing tools (AI agent: install missing tools and re-run)
#
# =============================================================================

[CmdletBinding()]
param(
    [switch]$CheckOnly,
    [int[]]$SkipPhase = @(),
    [switch]$ApiOnly,
    [string]$PostgresPassword,
    [switch]$Json,
    [int]$ApiPort = 8080,
    [int]$WebPort = 3000
)

$ErrorActionPreference = "Continue"
$Script:RepoRoot = (Resolve-Path "$PSScriptRoot\..").Path
$Script:JsonOutput = $Json.IsPresent
$Script:PhaseResults = @()
$Script:ExitCode = 0

function Out-Line {
    param([string]$Msg, [string]$Level = "info")
    if ($Script:JsonOutput) {
        $ts = (Get-Date).ToString("o")
        Write-Output "{`"ts`":`"$ts`",`"level`":`"$Level`",`"msg`":`"$($Msg -replace '"', '\\"')`"}"
    } else {
        switch ($Level) {
            "ok"     { Write-Host "  [OK] $Msg" -ForegroundColor Green }
            "warn"   { Write-Host "  [WARN] $Msg" -ForegroundColor Yellow }
            "err"    { Write-Host "  [ERR] $Msg" -ForegroundColor Red }
            "info"   { Write-Host "  [..] $Msg" -ForegroundColor Gray }
            "skip"   { Write-Host "  [SKIP] $Msg" -ForegroundColor DarkYellow }
            "phase"  { Write-Host "" ; Write-Host "=== $Msg ===" -ForegroundColor Cyan }
            default  { Write-Host "  $Msg" }
        }
    }
}

function Phase-Start {
    param([int]$Num, [string]$Name)
    Out-Line "Phase $Num/$Total : $Name" "phase"
}

function Phase-End {
    param([int]$Num, [string]$Status, [string]$Detail = "")
    $Script:PhaseResults += @{ phase = $Num; status = $Status; detail = $Detail }
    if ($Status -eq "FAIL") { $Script:ExitCode = 2 }
}

function Fail {
    param([string]$Msg, [int]$Code = 2)
    Out-Line $Msg "err"
    exit $Code
}

# Compute total phases
$Script:Total = if ($ApiOnly) { 6 } else { 7 }

# =============================================================================
# Phase 1: Preflight
# =============================================================================
$phase = 1
if (-not ($SkipPhase -contains $phase)) {
    Phase-Start $phase "Preflight"
    $toolMissing = @()

    # Go
    try { $goVer = (& go version) 2>&1 | Select-Object -First 1; Out-Line "Go: $goVer" "ok" }
    catch { $toolMissing += "Go (https://go.dev/dl/)"; Out-Line "Go not found" "err" }

    # Node
    try { $nodeVer = (& node --version) 2>&1 | Select-Object -First 1; Out-Line "Node: $nodeVer" "ok" }
    catch { $toolMissing += "Node.js (https://nodejs.org/)"; Out-Line "Node not found" "err" }

    # Postgres
    try {
        $pgVer = (& "C:\Program Files\PostgreSQL\17\bin\psql.exe" --version) 2>&1 | Select-Object -First 1
        Out-Line "psql: $pgVer" "ok"
    } catch {
        $toolMissing += "PostgreSQL 16+"
        Out-Line "psql not found" "err"
    }

    # Postgres service running
    $svc = Get-Service postgresql-x64-17 -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status -eq "Running") {
        Out-Line "Postgres service: Running" "ok"
    } else {
        Out-Line "Postgres service: not running. Start with: Start-Service postgresql-x64-17" "err"
        $toolMissing += "Postgres service (start with: Start-Service postgresql-x64-17)"
    }

    # git
    try { $gitVer = (& git --version) 2>&1 | Select-Object -First 1; Out-Line "git: $gitVer" "ok" }
    catch { $toolMissing += "git"; Out-Line "git not found" "err" }

    # .env file writable
    if (Test-Path "$Script:RepoRoot\.env") {
        if (Test-Path "$Script:RepoRoot\.env" -PathType Leaf) {
            Out-Line ".env exists" "ok"
        }
    } else {
        Out-Line ".env does not exist (will create from .env.example)" "info"
    }

    # Check repo root
    if (Test-Path "$Script:RepoRoot\go.mod") {
        Out-Line "Repo root: $Script:RepoRoot" "ok"
    } else {
        Out-Line "Not in fledger repo root: $Script:RepoRoot" "err"
        Fail "go.mod not found. cd to repo root first."
    }

    if ($toolMissing.Count -gt 0) {
        Out-Line "Missing tools:" "err"
        $toolMissing | ForEach-Object { Out-Line "  - $_" "err" }
        if ($CheckOnly) { exit 1 }
        Fail "Install missing tools and re-run." 64
    } else {
        Phase-End $phase "PASS"
    }
} else {
    Out-Line "Phase 1 (Preflight) skipped" "skip"
}

# =============================================================================
# Phase 2: Postgres setup (needs admin — modify pg_hba.conf + restart)
# =============================================================================
$phase = 2
if (-not ($SkipPhase -contains $phase) -and -not $ApiOnly) {
    Phase-Start $phase "Postgres setup"

    # Run the dedicated setup script which handles admin UAC itself
    $setupScript = Join-Path $Script:RepoRoot "scripts\setup-local-postgres.ps1"
    if (-not (Test-Path $setupScript)) {
        Out-Line "Missing: $setupScript" "err"
        Fail "scripts/setup-local-postgres.ps1 not found"
    }

    if ($PostgresPassword) {
        Out-Line "Running setup with non-interactive password"
        & $setupScript -NewPostgresPassword $PostgresPassword -FmcgPassword "fmcg_dev_password"
    } else {
        Out-Line "Running setup (uses default password: Desmone8327)"
        & $setupScript
    }

    if ($LASTEXITCODE -eq 0) {
        Out-Line "Postgres setup completed" "ok"
        Phase-End $phase "PASS"
    } else {
        Out-Line "Postgres setup failed (exit $LASTEXITCODE). See output above." "err"
        Phase-End $phase "FAIL"
        if (-not $CheckOnly) { exit 2 }
    }
} else {
    Out-Line "Phase 2 (Postgres setup) skipped" "skip"
}

# =============================================================================
# Phase 3: .env setup
# =============================================================================
$phase = 3
if (-not ($SkipPhase -contains $phase)) {
    Phase-Start $phase "Env setup"

    $envPath = Join-Path $Script:RepoRoot ".env"
    $envExamplePath = Join-Path $Script:RepoRoot ".env.example"

    if (-not (Test-Path $envPath)) {
        if (Test-Path $envExamplePath) {
            Copy-Item $envExamplePath $envPath -Force
            Out-Line "Copied .env.example → .env" "ok"
        } else {
            Out-Line ".env.example not found" "err"
            Fail ".env.example missing from repo"
        }
    } else {
        Out-Line ".env already exists" "ok"
    }

    # Verify critical fields
    $envContent = Get-Content $envPath -Raw
    $needsFix = $false

    if ($envContent -notmatch "DB_USER=fmcg") {
        Out-Line "DB_USER not set to fmcg" "warn"
        $needsFix = $true
    }
    if ($envContent -notmatch "DB_PASSWORD=") {
        Out-Line "DB_PASSWORD missing" "warn"
        $needsFix = $true
    }

    # Check JWT_SECRET is set and >= 32 chars
    if ($envContent -match "JWT_SECRET=(\S+)") {
        $jwt = $Matches[1]
        if ($jwt.Length -lt 32) {
            Out-Line "JWT_SECRET too short ($($jwt.Length) chars). Regenerating..." "warn"
            $needsFix = $true
        }
    } else {
        Out-Line "JWT_SECRET missing or placeholder" "warn"
        $needsFix = $true
    }

    if ($needsFix) {
        # Generate a fresh JWT secret (48 bytes base64url = 64 chars)
        $newSecret = & node -e "console.log(require('crypto').randomBytes(48).toString('base64url'))" 2>$null
        if (-not $newSecret -or $newSecret.Length -lt 32) {
            # PowerShell fallback
            $newSecret = -join ((1..64) | ForEach-Object { [char[]](65..90,97..122,48..57) | Get-Random })
        }

        # Update or insert JWT_SECRET
        if ($envContent -match "JWT_SECRET=.*") {
            $envContent = $envContent -replace "JWT_SECRET=.*", "JWT_SECRET=$newSecret"
        } else {
            $envContent += "`nJWT_SECRET=$newSecret"
        }
        # Ensure fmcg DB credentials
        if ($envContent -notmatch "DB_USER=fmcg") {
            $envContent = $envContent -replace "DB_USER=.*", "DB_USER=fmcg"
        }
        if ($envContent -notmatch "DB_PASSWORD=") {
            $envContent = $envContent -replace "DB_PASSWORD=.*", "DB_PASSWORD=fmcg_dev_password"
        }

        [System.IO.File]::WriteAllText($envPath, $envContent, [System.Text.Encoding]::UTF8)
        Out-Line "Updated .env with fresh JWT_SECRET + DB creds" "ok"
    }

    Phase-End $phase "PASS"
} else {
    Out-Line "Phase 3 (Env setup) skipped" "skip"
}

# =============================================================================
# Phase 4: Run migrations (as postgres superuser for CREATE EXTENSION)
# =============================================================================
$phase = 4
if (-not ($SkipPhase -contains $phase)) {
    Phase-Start $phase "Migrations"

    $envContent = Get-Content "$Script:RepoRoot\.env" -Raw
    if ($envContent -match "DB_PASSWORD=(\S+)") { $dbPwd = $Matches[1] } else { $dbPwd = "fmcg_dev_password" }

    # Drop and recreate db to ensure clean state (handles dirty migration state)
    $env:PGPASSWORD = $PostgresPassword
    if (-not $env:PGPASSWORD) { $env:PGPASSWORD = "Desmone8327" }

    Out-Line "Dropping + recreating fmcg_wallet (clean slate)..."
    & "C:\Program Files\PostgreSQL\17\bin\psql.exe" -U postgres -h 127.0.0.1 -w -c "DROP DATABASE IF EXISTS fmcg_wallet;" 2>&1 | Out-Null
    & "C:\Program Files\PostgreSQL\17\bin\psql.exe" -U postgres -h 127.0.0.1 -w -c "CREATE DATABASE fmcg_wallet OWNER fmcg;" 2>&1 | Out-Null
    & "C:\Program Files\PostgreSQL\17\bin\psql.exe" -U postgres -h 127.0.0.1 -w -c "GRANT ALL ON SCHEMA public TO fmcg; ALTER SCHEMA public OWNER TO fmcg;" 2>&1 | Out-Null

    # Run migrations as postgres (needs CREATE EXTENSION privilege)
    Out-Line "Running migrations as postgres..."
    $env:DB_USER = "postgres"
    $env:DB_PASSWORD = $env:PGPASSWORD
    $migOut = & go run "$Script:RepoRoot\cmd\migrator" up 2>&1
    $migExitCode = $LASTEXITCODE

    if ($migExitCode -eq 0) {
        Out-Line "Migrations applied (version=18)" "ok"
        # Restore .env DB user to fmcg for app runtime
        $env:DB_USER = "fmcg"
        $env:DB_PASSWORD = $dbPwd
        Phase-End $phase "PASS"
    } else {
        Out-Line "Migration failed (exit $migExitCode):" "err"
        $migOut | ForEach-Object { Out-Line "  $_" "err" }
        Phase-End $phase "FAIL"
        if (-not $CheckOnly) { exit 2 }
    }
} else {
    Out-Line "Phase 4 (Migrations) skipped" "skip"
}

# =============================================================================
# Phase 5: RLS workaround (disable RLS on refresh_tokens for dev)
# =============================================================================
$phase = 5
if (-not ($SkipPhase -contains $phase)) {
    Phase-Start $phase "RLS workaround"

    $env:PGPASSWORD = $PostgresPassword
    if (-not $env:PGPASSWORD) { $env:PGPASSWORD = "Desmone8327" }

    Out-Line "Disabling RLS on refresh_tokens (dev-only workaround)"
    $rlsOut = & "C:\Program Files\PostgreSQL\17\bin\psql.exe" -U postgres -h 127.0.0.1 -w -d fmcg_wallet -c "ALTER TABLE refresh_tokens DISABLE ROW LEVEL SECURITY;" 2>&1
    if ($LASTEXITCODE -eq 0) {
        Out-Line "RLS disabled" "ok"
        Phase-End $phase "PASS"
    } else {
        Out-Line "Failed to disable RLS" "warn"
        Phase-End $phase "WARN"
    }
} else {
    Out-Line "Phase 5 (RLS workaround) skipped" "skip"
}

# =============================================================================
# Phase 6: Seed demo data
# =============================================================================
$phase = 6
if (-not ($SkipPhase -contains $phase)) {
    Phase-Start $phase "Seed data"

    $envContent = Get-Content "$Script:RepoRoot\.env" -Raw
    if ($envContent -match "DB_PASSWORD=(\S+)") { $dbPwd = $Matches[1] } else { $dbPwd = "fmcg_dev_password" }
    $env:PGPASSWORD = $dbPwd

    $seedScript = Join-Path $Script:RepoRoot "scripts\seed-local-dev-data.ps1"
    if (-not (Test-Path $seedScript)) {
        Out-Line "Missing: $seedScript" "err"
        Phase-End $phase "FAIL"
    } else {
        & $seedScript
        if ($LASTEXITCODE -eq 0) {
            Phase-End $phase "PASS"
        } else {
            Phase-End $phase "FAIL"
        }
    }
} else {
    Out-Line "Phase 6 (Seed) skipped" "skip"
}

# =============================================================================
# Phase 7: Self-test (start API + web, hit endpoints, stop)
# =============================================================================
$phase = 7
if (-not ($SkipPhase -contains $phase)) {
    if ($CheckOnly) {
        Out-Line "Phase 7 (Self-test) skipped (CheckOnly mode)" "skip"
    } else {
        Phase-Start $phase "Self-test"

        # Find a free port for API (in case 8080 is busy)
        $env:DB_USER = "fmcg"
        $env:DB_PASSWORD = "fmcg_dev_password"
        $env:APP_PORT = $ApiPort
        $env:API_BASE_URL = "http://localhost:$ApiPort"

        # Clean up any leftovers from prior runs
        Get-NetTCPConnection -LocalPort $ApiPort -ErrorAction SilentlyContinue | ForEach-Object {
            try { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue } catch {}
        }

        Out-Line "Starting API on :$ApiPort..."
        $apiProc = Start-Process -FilePath "go" -ArgumentList "run", "$Script:RepoRoot\cmd\api" -WorkingDirectory $Script:RepoRoot -RedirectStandardOutput "$env:TEMP\fmcg-api-test.log" -RedirectStandardError "$env:TEMP\fmcg-api-test.err" -WindowStyle Hidden -PassThru
        Start-Sleep -Seconds 5

        # Test healthz
        try {
            $r = Invoke-WebRequest "http://localhost:$ApiPort/healthz" -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
            if ($r.StatusCode -eq 200) {
                Out-Line "GET /healthz → 200 OK" "ok"
            } else {
                Out-Line "GET /healthz → $($r.StatusCode)" "err"
            }
        } catch {
            Out-Line "GET /healthz failed: $($_.Exception.Message)" "err"
            Out-Line "API log: $env:TEMP\fmcg-api-test.err" "info"
        }

        # Test /v1/ping
        try {
            $r = Invoke-WebRequest "http://localhost:$ApiPort/v1/ping" -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
            if ($r.StatusCode -eq 200) {
                Out-Line "GET /v1/ping → 200 OK" "ok"
            } else {
                Out-Line "GET /v1/ping → $($r.StatusCode)" "err"
            }
        } catch {
            Out-Line "GET /v1/ping failed" "err"
        }

        # Test login (with proper ASCII body to avoid BOM)
        $body = '{"tenant_id":"00000000-0000-0000-0000-000000000001","username":"33333333-3333-3333-3333-333333333333","password":"DemoTest1234!"}'
        $tmpBody = Join-Path $env:TEMP "login.json"
        [System.IO.File]::WriteAllText($tmpBody, $body, [System.Text.Encoding]::ASCII)
        try {
            $r = & curl.exe -s -X POST -H "Content-Type: application/json" --data-binary "@$tmpBody" "http://localhost:$ApiPort/v1/auth/login"
            if ($r -match '"access_token"') {
                Out-Line "POST /v1/auth/login → token issued" "ok"
            } else {
                Out-Line "POST /v1/auth/login → $r" "err"
            }
        } catch {
            Out-Line "POST /v1/auth/login failed" "err"
        }
        Remove-Item $tmpBody -Force -ErrorAction SilentlyContinue

        # Stop API
        Out-Line "Stopping API..."
        try { Stop-Process -Id $apiProc.Id -Force -ErrorAction SilentlyContinue } catch {}
        Get-NetTCPConnection -LocalPort $ApiPort -ErrorAction SilentlyContinue | ForEach-Object {
            try { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue } catch {}
        }

        Phase-End $phase "PASS"
    }
} else {
    Out-Line "Phase 7 (Self-test) skipped" "skip"
}

# =============================================================================
# Final report
# =============================================================================
Write-Host ""
Write-Host "=============================================" -ForegroundColor Green
Write-Host "Setup complete" -ForegroundColor Green
Write-Host "=============================================" -ForegroundColor Green
Write-Host ""
Write-Host "Phase results:" -ForegroundColor Cyan
foreach ($r in $Script:PhaseResults) {
    $color = switch ($r.status) { "PASS" { "Green" } "FAIL" { "Red" } "WARN" { "Yellow" } default { "Gray" } }
    Write-Host ("  Phase {0}: {1,-5} {2}" -f $r.phase, $r.status, $r.detail) -ForegroundColor $color
}

Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "  cd '$Script:RepoRoot'"
Write-Host ""
Write-Host "  # Start API (terminal 1)"
Write-Host "  go run ./cmd/api"
Write-Host ""
Write-Host "  # Start web (terminal 2)"
Write-Host "  `$env:API_BASE_URL = 'http://localhost:$ApiPort'"
Write-Host "  node web/server.js"
Write-Host ""
Write-Host "  # Open browser"
Write-Host "  http://localhost:$WebPort"
Write-Host ""
Write-Host "Login credentials:" -ForegroundColor Cyan
Write-Host "  Tenant ID:  00000000-0000-0000-0000-000000000001"
Write-Host "  Admin UUID: 33333333-3333-3333-3333-333333333333"
Write-Host "  Sales UUID: 44444444-4444-4444-4444-444444444444"
Write-Host "  Password:   DemoTest1234!"
Write-Host ""
Write-Host "For daily use: run .\scripts\run-stack.ps1 (start) + stop-stack.ps1 (stop)" -ForegroundColor DarkGray

if ($Script:ExitCode -ne 0) { exit $Script:ExitCode }
exit 0
