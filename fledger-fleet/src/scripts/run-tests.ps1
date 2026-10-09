# =============================================================================
# FLEDGER FLEET — run-tests.ps1
#
# Cross-platform-friendly wrapper around `go test` that retries each package
# when Windows Defender / Application Control blocks the freshly-built test
# binary (a known issue with go test in sandboxed Windows environments).
# =============================================================================

$ErrorActionPreference = "Stop"
$env:GOCACHE = if ($env:GOCACHE) { $env:GOCACHE } else { Join-Path $PSScriptRoot "..\.gocache" }
$env:GOTMPDIR = if ($env:GOTMPDIR) { $env:GOTMPDIR } else { Join-Path $PSScriptRoot "..\.gobuild" }
$env:CGO_ENABLED = "0"

New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null

Set-Location (Join-Path $PSScriptRoot "..")

$packages = @(
    "internal/auth/jwt",
    "internal/config",
    "internal/domain",
    "internal/usecase",
    "internal/integration",
    "internal/integration/coreclient"
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
    for ($i = 1; $i -le 10; $i++) {
        $blocked = $false
        $output = $null
        try {
            $output = & $bin 2>&1
        } catch {
            $msg = $_.Exception.Message
            if ($msg -match "Application Control policy") { $blocked = $true }
            else { $output = $_.ToString(); break }
        }
        if ($LASTEXITCODE -eq 0) { $ok = $true; break }
        if (-not $blocked) { break }
        Start-Sleep -Milliseconds 700
    }
    if ($ok) { Write-Host "  [OK]" -ForegroundColor Green }
    else     { Write-Host "  [FAIL]" -ForegroundColor Red; $failed++ }
}

if ($failed -gt 0) { exit 1 } else { exit 0 }