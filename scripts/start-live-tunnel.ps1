# ==============================================================================
# FLEDGER OS - Live Production Tunnel (Industry Branded / Multi-Provider)
# Subdomain disesuaikan dengan industri FMCG Distribution & Logistics
# ==============================================================================

[CmdletBinding()]
param(
    [ValidateSet("localtunnel", "cloudflared")]
    [string]$Provider = "localtunnel",

    # Pilihan nama industri: fledger-fmcg, fledger-distribusi, fledger-logistik, fmcg-distribusi
    [string]$Subdomain = "fledger-fmcg",

    [int]$Port = 8000
)

$ErrorActionPreference = "Stop"

Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host "  FLEDGER OS - LIVE PRODUCTION TUNNEL (INDUSTRY BRANDED)  " -ForegroundColor Cyan
Write-Host "  Industri : FMCG Distribution, Supply Chain & Logistics  " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Check Gateway Port
$gwConn = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
if (-not $gwConn) {
    Write-Host "[!] Gateway port :$Port belum aktif. Menyalakan seluruh ekosistem Fledger..." -ForegroundColor Yellow
    & "$PSScriptRoot\start-all-local.ps1" -NoSeed
    Start-Sleep -Seconds 2
}

$logDir = "$PSScriptRoot\..\.gobuild\logs"
if (-not (Test-Path $logDir)) {
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}

$tunnelUrl = $null
$tunnelPassword = $null

if ($Provider -eq "localtunnel") {
    Write-Host "`n[1/2] Menyiapkan Branded Tunnel via Localtunnel (Subdomain: $Subdomain)..." -ForegroundColor Yellow

    # Fetch public IP for tunnel password
    try {
        $tunnelPassword = (curl.exe -s --max-time 4 https://loca.lt/mytunnelpassword).Trim()
    } catch {
        $tunnelPassword = "Lihat https://loca.lt/mytunnelpassword"
    }

    $logFile = "$logDir\localtunnel-$Subdomain.log"
    if (Test-Path $logFile) { Remove-Item $logFile -Force }

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = "cmd.exe"
    $psi.Arguments = "/c npx -y localtunnel --port $Port --subdomain $Subdomain"
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true

    $proc = [System.Diagnostics.Process]::Start($psi)

    $timeout = 25
    $timer = [System.Diagnostics.Stopwatch]::StartNew()
    while ($timer.Elapsed.TotalSeconds -lt $timeout -and -not $tunnelUrl) {
        if ($proc.HasExited) {
            Write-Error "Localtunnel berhenti mendadak. Periksa koneksi internet atau ketersediaan node/npx."
            exit 1
        }
        $line = $proc.StandardOutput.ReadLine()
        if ($line) {
            Add-Content -Path $logFile -Value $line
            if ($line -match "https://[a-zA-Z0-9-]+\.loca\.lt") {
                $tunnelUrl = $matches[0]
                break
            }
        }
        Start-Sleep -Milliseconds 100
    }
}
elseif ($Provider -eq "cloudflared") {
    Write-Host "`n[1/2] Menghubungkan ke Cloudflare Global Edge Network..." -ForegroundColor Yellow

    $cloudflared = "C:\Program Files (x86)\cloudflared\cloudflared.exe"
    if (-not (Test-Path $cloudflared)) {
        $cloudflaredCmd = Get-Command cloudflared -ErrorAction SilentlyContinue
        if ($cloudflaredCmd) { $cloudflared = $cloudflaredCmd.Source }
        else {
            Write-Error "cloudflared.exe tidak ditemukan di sistem."
            exit 1
        }
    }

    $logFile = "$logDir\cloudflared-tunnel.log"
    if (Test-Path $logFile) { Remove-Item $logFile -Force }

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $cloudflared
    $psi.Arguments = "tunnel --url http://localhost:$Port"
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true

    $proc = [System.Diagnostics.Process]::Start($psi)

    $timeout = 25
    $timer = [System.Diagnostics.Stopwatch]::StartNew()
    while ($timer.Elapsed.TotalSeconds -lt $timeout -and -not $tunnelUrl) {
        if ($proc.HasExited) {
            Write-Error "Cloudflared berhenti mendadak."
            exit 1
        }
        $line = $proc.StandardError.ReadLine()
        if ($line) {
            Add-Content -Path $logFile -Value $line
            if ($line -match "https://[a-zA-Z0-9-]+\.trycloudflare\.com") {
                $tunnelUrl = $matches[0]
                break
            }
        }
        Start-Sleep -Milliseconds 100
    }
}

if (-not $tunnelUrl) {
    Write-Error "Gagal mendeteksi URL tunnel. Periksa file log: $logFile"
    if ($proc -and -not $proc.HasExited) { $proc.Kill() }
    exit 1
}

# Display Production URLs
Write-Host "`n==========================================================" -ForegroundColor Green
Write-Host "  FLEDGER LIVE GO-LIVE READY! (INDUSTRY BRANDED URL)     " -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
Write-Host "`nPublic Base HTTPS URL:" -ForegroundColor Gray
Write-Host "  $tunnelUrl" -ForegroundColor Cyan

if ($Provider -eq "localtunnel") {
    Write-Host "`n[!] Catatan Akses Pertama Pengunjung Baru di Browser:" -ForegroundColor Yellow
    Write-Host "    Jika muncul halaman verifikasi Localtunnel, masukkan IP berikut:" -ForegroundColor Gray
    Write-Host "    Tunnel Password / IP Host: $tunnelPassword" -ForegroundColor Green
    Write-Host "    (Hanya perlu dimasukkan 1x pada kunjungan awal per browser)`n" -ForegroundColor DarkGray
}

Write-Host "--- URL WEBHOOK PAYMENT GATEWAY (MIDTRANS) ---" -ForegroundColor Yellow
Write-Host "1. Payment Notification URL (Webhook):" -ForegroundColor White
Write-Host "   $tunnelUrl/v1/pay/webhooks/midtrans" -ForegroundColor Green
Write-Host "2. Finish Redirect URL (Pelunasan Sukses):" -ForegroundColor White
Write-Host "   $tunnelUrl/pay/" -ForegroundColor Green
Write-Host "3. Unfinish / Error Redirect URL:" -ForegroundColor White
Write-Host "   $tunnelUrl/pay/" -ForegroundColor Green

Write-Host "`n--- LINK EKOSISTEM LIVE UNTUK PITCHING, DEMO & OPERASIONAL ---" -ForegroundColor Yellow
Write-Host "  Landing Page Utama       : $tunnelUrl/" -ForegroundColor White
Write-Host "  Live Ops Hub (Console)   : $tunnelUrl/hub" -ForegroundColor White
Write-Host "  Closed-Loop Simulator    : $tunnelUrl/simulator" -ForegroundColor White
Write-Host "  Fledger Core (ERP/Buku)  : $tunnelUrl/core/web/" -ForegroundColor White
Write-Host "  Fledger Fleet (Logistik) : $tunnelUrl/fleet/" -ForegroundColor White
Write-Host "  Fledger Pay (QRIS/VA)    : $tunnelUrl/pay/" -ForegroundColor White
Write-Host "  Fledger Force (SFA Sales): $tunnelUrl/force/" -ForegroundColor White
Write-Host "  Fledger Order (Gudang/PO): $tunnelUrl/order/" -ForegroundColor White
Write-Host "  Fledger Dunning (WA/AR)  : $tunnelUrl/dunning/" -ForegroundColor White

Write-Host "`n[i] Tunnel sedang aktif. Tekan [Ctrl + C] untuk menghentikan tunnel.`n" -ForegroundColor DarkGray

try {
    while (-not $proc.HasExited) {
        Start-Sleep -Seconds 1
    }
} finally {
    if ($proc -and -not $proc.HasExited) {
        $proc.Kill()
    }
    Write-Host "`nTunnel telah ditutup." -ForegroundColor Yellow
}
