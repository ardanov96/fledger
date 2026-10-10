# ==============================================================================
# FLEDGER OS — Stop All Local Services
# ==============================================================================

Write-Host "Stopping all local Fledger OS processes..." -ForegroundColor Yellow

$ports = @(8081, 8082, 8083, 8084, 8085, 8086)

foreach ($port in $ports) {
    try {
        $pids = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique
        foreach ($p in $pids) {
            if ($p -and $p -ne 0) {
                Stop-Process -Id $p -Force -ErrorAction SilentlyContinue
                Write-Host "  Terminated process $p listening on port :$port" -ForegroundColor Green
            }
        }
    } catch {
        # ignore
    }
}

Write-Host "All local microservice processes stopped cleanly." -ForegroundColor Green
