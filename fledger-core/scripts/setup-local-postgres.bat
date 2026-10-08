@echo off
REM =============================================================================
REM FMCG Wallet - Postgres setup launcher (auto-elevates to Admin)
REM =============================================================================
REM Double-click. UAC prompt appears -> click Yes.
REM Elevated PowerShell window stays open after script finishes.
REM Full log: %TEMP%\fmcg-postgres-setup.log
REM =============================================================================

cd /d "%~dp0"

echo.
echo === FMCG Wallet Postgres setup ===
echo.
echo Opening elevated PowerShell... (click Yes on UAC prompt)
echo After script finishes, elevated window will stay open.
echo Full log: %TEMP%\fmcg-postgres-setup.log
echo.

powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "Start-Process powershell -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','%~dp0setup-local-postgres.ps1' -Verb RunAs -Wait -WindowStyle Normal"

echo.
echo Done. Check the elevated window output above.
echo Full log: %TEMP%\fmcg-postgres-setup.log
echo.
pause
