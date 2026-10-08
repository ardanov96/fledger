# =============================================================================
# FMCG Wallet - Show configuration (env + pg + tools + paths)
# =============================================================================
#
# One-shot dump of EVERYTHING relevant for debugging "why doesn't this work":
#   - .env file contents (parsed)
#   - Tool versions (go, node, psql)
#   - Postgres runtime config (relevant settings)
#   - Schema migrations status
#   - Important paths (repo, logs)
#
# Usage:
#   .\scripts\show-config.ps1            # human-readable
#   .\scripts\show-config.ps1 -Json      # structured JSON for AI agents
# =============================================================================

[CmdletBinding()]
param([switch]$Json)

$ErrorActionPreference = "Continue"
$RepoRoot = (Resolve-Path "$PSScriptRoot\..").Path
$psqlPath = "C:\Program Files\PostgreSQL\17\bin\psql.exe"
$JsonMode = $Json.IsPresent

$out = @{
    generated_at = (Get-Date).ToString("o")
    repo_root    = $RepoRoot
    tools        = @{}
    env_file     = @{}
    postgres     = @{}
    paths        = @{}
}

function Out-Line {
    param([string]$Msg, [string]$Color = "White")
    if ($JsonMode) { return }
    Write-Host $Msg -ForegroundColor $Color
}

function Section {
    param([string]$Title)
    if (-not $JsonMode) {
        Write-Host ""
        Write-Host "=== $Title ===" -ForegroundColor Cyan
    }
}

# ============================================================================
# Paths
# ============================================================================
Section "Paths"
Out-Line "Repo root:       $RepoRoot" "Gray"
Out-Line ".env file:       $(if (Test-Path (Join-Path $RepoRoot '.env')) { 'exists' } else { 'MISSING' })" "Gray"
Out-Line "Migrations dir:  $RepoRoot\migrations" "Gray"
Out-Line "Web public dir:  $RepoRoot\web\public" "Gray"
Out-Line "Logs:            %TEMP%\fmcg-{api,web,postgres-setup}.log" "Gray"

$out.paths = @{
    repo_root     = $RepoRoot
    env_file      = if (Test-Path (Join-Path $RepoRoot ".env")) { "exists" } else { "missing" }
    migrations    = "$RepoRoot\migrations"
    web_public    = "$RepoRoot\web\public"
    logs_dir      = $env:TEMP
}

# ============================================================================
# Tools
# ============================================================================
Section "Tools"
$tools = @{}
foreach ($t in @(
    @{ name = "go";       cmd = "go";      args = @("version") }
    @{ name = "node";     cmd = "node";    args = @("--version") }
    @{ name = "psql";     cmd = $psqlPath; args = @("--version") }
    @{ name = "curl";     cmd = "curl";    args = @("--version") }
    @{ name = "git";      cmd = "git";     args = @("--version") }
)) {
    try {
        $v = (& $t.cmd $t.args 2>&1 | Select-Object -First 1).Trim()
        Out-Line ("  {0,-8} {1}" -f $t.name, $v) "Green"
        $tools[$t.name] = $v
    } catch {
        Out-Line ("  {0,-8} NOT FOUND" -f $t.name) "Red"
        $tools[$t.name] = $null
    }
}
$out.tools = $tools

# ============================================================================
# .env file contents
# ============================================================================
Section ".env contents"
$envPath = Join-Path $RepoRoot ".env"
if (Test-Path $envPath) {
    $envFileData = @{}
    Get-Content $envPath | ForEach-Object {
        if ($_ -match "^\s*([A-Z_][A-Z0-9_]*)\s*=\s*(.+?)\s*$" -and $_ -notmatch "^#") {
            $k = $Matches[1]
            $v = $Matches[2].Trim('"', "'")
            # Mask secrets in display
            $displayVal = $v
            if ($k -match "SECRET|PASSWORD|PWD|TOKEN|KEY") {
                $displayVal = if ($v.Length -gt 8) { $v.Substring(0, 4) + "***" + $v.Substring($v.Length - 4) } else { "***" }
            }
            Out-Line ("  {0,-22} = {1}" -f $k, $displayVal) "Gray"
            $envFileData[$k] = $v
        }
    }
    $out.env_file = $envFileData
} else {
    Out-Line "  .env MISSING (run setup-everything.ps1)" "Red"
}

# ============================================================================
# Postgres config + status
# ============================================================================
Section "Postgres"
if (-not (Test-Path $psqlPath)) {
    Out-Line "psql not found - skipping Postgres checks" "Yellow"
} else {
    $env:PGPASSWORD = if ($env:POSTGRES_PASSWORD) { $env:POSTGRES_PASSWORD } else { "Desmone8327" }
    $pgOut = @{}

    # Version + uptime
    $ver = & "$psqlPath" -U postgres -h 127.0.0.1 -w -tA -c "SELECT version();" 2>$null
    Out-Line "  Version:  $ver" "Gray"
    $pgOut.version = $ver

    # Settings of interest
    $settings = & "$psqlPath" -U postgres -h 127.0.0.1 -w -tA -c @"
SELECT name, setting, unit
FROM pg_settings
WHERE name IN ('listen_addresses','port','shared_preload_libraries','max_connections',
               'statement_timeout','log_destination','TimeZone')
ORDER BY name;
"@ 2>$null
    if ($settings) {
        Out-Line "  Settings:" "Yellow"
        $pgOut.settings = @{}
        foreach ($line in $settings) {
            if ($line -match "^\s*(\S+)\s+\|\s+(\S+)\s*\|?\s*(\S*)") {
                $k = $Matches[1]
                $v = $Matches[2]
                $u = $Matches[3]
                Out-Line ("    {0,-30} = {1} {2}" -f $k, $v, $u) "Gray"
                $pgOut.settings[$k] = @{ value = $v; unit = $u }
            }
        }
    } else {
        Out-Line "  Settings: (query returned no rows - tables may not exist yet)" "Yellow"
    }

    # fmcg_wallet db size + table count
    $stats = & "$psqlPath" -U postgres -h 127.0.0.1 -w -tA -c @"
SELECT pg_size_pretty(pg_database_size('fmcg_wallet')) AS db_size,
       (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public') AS tables;
"@ 2>$null
    if ($stats -match "(\S+)\s+\|\s+(\d+)") {
        Out-Line ("  DB size:  {0}" -f $Matches[1]) "Gray"
        Out-Line ("  Tables:   {0}" -f $Matches[2]) "Gray"
        $pgOut.db_size = $Matches[1]
        $pgOut.table_count = [int]$Matches[2]
    }

    # Migrations
    $env:PGPASSWORD = "fmcg_dev_password"
    $ver = (& "$psqlPath" -U fmcg -h 127.0.0.1 -d fmcg_wallet -w -tA -c "SELECT MAX(version) FROM schema_migrations;" 2>$null).Trim()
    if ($ver -match "^\d+$") {
        Out-Line ("  Latest migration: version={0}" -f $ver) "Gray"
        $pgOut.latest_migration = $ver
    } else {
        Out-Line "  Latest migration: (no schema_migrations table - DB not initialized)" "Yellow"
    }

    $out.postgres = $pgOut
}

# ============================================================================
# JSON output
# ============================================================================
if ($JsonMode) {
    $out | ConvertTo-Json -Depth 6 -Compress
}
