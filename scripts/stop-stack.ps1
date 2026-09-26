# =============================================================================
# FMCG Wallet — Stop dev stack
# =============================================================================
#
# Kills any processes listening on the API (8080) and web (3000) ports.
# Idempotent: no-op if nothing is running.
# =============================================================================

[CmdletBinding()]
param(
    [int]$ApiPort = 8080,
    [int]$WebPort = 3000,
    [switch]$Force
)

function Out-Line {
    param([string]$Msg, [string]$Color = "White")
    Write-Host $Msg -ForegroundColor $Color
}

$stopped = 0
foreach ($port in @($ApiPort, $WebPort)) {
    $conns = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue
    foreach ($c in $conns) {
        $procId = $c.OwningProcess
        if ($procId -and $procId -ne 0) {
            try {
                if ($Force) {
                    Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
                } else {
                    Stop-Process -Id $procId -ErrorAction SilentlyContinue
                }
                Out-Line "Stopped PID $procId on :$port" "Yellow"
                $stopped++
            } catch {
                Out-Line "Failed to stop PID $procId on :$port" "Red"
            }
        }
    }
}

# Also kill any orphaned go/node processes from our stack
Get-Process -Name "go","node" -ErrorAction SilentlyContinue | Where-Object {
    $_.CommandLine -match "cmd/api|web.server.js" -or $_.Path -match "cmd/api|web.server.js"
} | ForEach-Object {
    try {
        Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
        Out-Line "Stopped orphaned $($_.ProcessName) PID $($_.Id)" "Yellow"
        $stopped++
    } catch {}
}

if ($stopped -eq 0) {
    Out-Line "No running stack found on :$ApiPort or :$WebPort" "Gray"
} else {
    Out-Line ""
    Out-Line "Stopped $stopped process(es)" "Green"
}
