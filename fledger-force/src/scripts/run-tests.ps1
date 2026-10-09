# =============================================================================
# FLEDGER FORCE — run-tests.ps1
# =============================================================================

$ErrorActionPreference = "Stop"
$env:GOCACHE = if ($env:GOCACHE) { $env:GOCACHE } else { Join-Path $PSScriptRoot "..\.gocache" }
$env:GOTMPDIR = if ($env:GOTMPDIR) { $env:GOTMPDIR } else { Join-Path $PSScriptRoot "..\.gobuild" }
$env:CGO_ENABLED = "0"
if (-not $env:FLEDGER_FORCE_TEST_DB) {
    $env:FLEDGER_FORCE_TEST_DB = "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_force_test?sslmode=disable"
}

New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null

Set-Location (Join-Path $PSScriptRoot "..")

$packages = @(
    "internal/platform/geo",
    "internal/integration"
)

$failed = 0
foreach ($p in $packages) {
    $name = ($p -split "/")[-1]
    $bin = Join-Path $env:GOTMPDIR "$name.test.exe"
    Write-Host "==> $p" -NoNewline
    & go test -c -o $bin "./$p/" 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  [BUILD FAILED]" -ForegroundColor Red
        $failed++
        continue
    }
    $ok = $false
    $output = $null
    $lastCode = -1
    for ($i = 1; $i -le 10; $i++) {
        $blocked = $false
        $output = $null
        try {
            $proc = Start-Process -FilePath $bin -PassThru -NoNewWindow -Wait -RedirectStandardOutput "$env:GOTMPDIR\$name.stdout" -RedirectStandardError "$env:GOTMPDIR\$name.stderr" -ErrorAction Stop
            $lastCode = $proc.ExitCode
            $output = (Get-Content "$env:GOTMPDIR\$name.stderr" -Raw -ErrorAction SilentlyContinue) + "`n" + (Get-Content "$env:GOTMPDIR\$name.stdout" -Raw -ErrorAction SilentlyContinue)
        } catch {
            $msg = $_.Exception.Message
            if ($msg -match "Application Control policy") { $blocked = $true; $output = $msg; $lastCode = -1 }
            else { $output = $_.ToString(); $lastCode = 1; break }
        }
        if ($lastCode -eq 0) { $ok = $true; break }
        if (-not $blocked) { break }
        Start-Sleep -Milliseconds 700
    }
    if ($ok) { Write-Host "  [OK]" -ForegroundColor Green }
    else     {
        Write-Host "  [FAIL lastCode=$lastCode]" -ForegroundColor Red
        if ($output) { Write-Host ("  " + ($output | Out-String)) }
        $failed++
    }
}

if ($failed -gt 0) { exit 1 } else { exit 0 }