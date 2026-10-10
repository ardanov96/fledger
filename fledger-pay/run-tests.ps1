# =============================================================================
# FLEDGER PAY — Root run-tests.ps1
# Delegates to src/scripts/run-tests.ps1
# =============================================================================
$target = Join-Path $PSScriptRoot "src\scripts\run-tests.ps1"
& powershell -ExecutionPolicy Bypass -File $target
exit $LASTEXITCODE
