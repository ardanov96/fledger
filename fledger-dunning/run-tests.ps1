# ==============================================================================
# FLEDGER DUNNING — Automated Test Runner
# Service: fledger-dunning (:8086)
# ==============================================================================

$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  FLEDGER DUNNING - Running Test Suite" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# Set environment
$env:CGO_ENABLED = "0"
$env:APP_ENV = "test"

# Execute Go unit and integration tests directly
Write-Host ""
Write-Host "[1/2] Running Go tests (unit and integration)..." -ForegroundColor Yellow
$testOutput = go test -v -count=1 ./... 2>&1
$testExitCode = $LASTEXITCODE

$testOutput | ForEach-Object {
    if ($_ -match "PASS:") {
        Write-Host $_ -ForegroundColor Green
    } elseif ($_ -match "FAIL:") {
        Write-Host $_ -ForegroundColor Red
    } else {
        Write-Host $_
    }
}

if ($testExitCode -ne 0) {
    Write-Host ""
    Write-Host "[FAIL] Test suite encountered errors (exit code: $testExitCode)" -ForegroundColor Red
    exit $testExitCode
}

Write-Host ""
Write-Host "[2/2] Verifying binary compilation..." -ForegroundColor Yellow
go build -o .gobuild/dunning-test.exe ./cmd/server
if ($LASTEXITCODE -ne 0) {
    Write-Host ""
    Write-Host "[FAIL] Compilation failed!" -ForegroundColor Red
    exit $LASTEXITCODE
}
Remove-Item -Path ".gobuild/dunning-test.exe" -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "==========================================================" -ForegroundColor Green
Write-Host "  ALL TESTS PASSED - FLEDGER DUNNING IS READY" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
exit 0
