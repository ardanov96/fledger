@echo off
echo ======================================
echo  Starting all 6 Fledger OS services
echo ======================================

set BASE=C:\Dev\fledger
set LOGDIR=%TEMP%\fledger-logs
if not exist "%LOGDIR%" mkdir "%LOGDIR%"

REM Cleanup any leftovers
for /f "tokens=5" %%a in ('netstat -aon ^| findstr /R "LISTENING" ^| findstr ":808[1-6]"') do (
    taskkill /F /PID %%a >nul 2>&1
)

REM 1. Core
set "APP_ENV=development"
set "PORT=8081"
set "DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fmcg_wallet?sslmode=disable"
set "REDIS_URL=redis://localhost:6379/0"
set "NATS_URL=nats://localhost:4222"
set "JWT_SECRET=fledger-super-secure-jwt-secret-key-2026-min-32"
start /B "C:\Dev\fledger\fledger-core\src\core.exe" > "%LOGDIR%\core.out" 2>&1
echo Core started

REM 2. Fleet
set "PORT=8082"
set "DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_fleet?sslmode=disable"
set "FLEDGER_CORE_URL=http://localhost:8081"
set "FLEDGER_TENANT_ID=a0000000-0000-0000-0000-000000000001"
set "JWT_SECRET=fledger-super-secure-jwt-secret-key-2026-min-32"
start /B "C:\Dev\fledger\fledger-fleet\src\fleet.exe" > "%LOGDIR%\fleet.out" 2>&1
echo Fleet started

REM 3. Pay
set "PORT=8083"
set "DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_pay?sslmode=disable"
set "FLEDGER_CORE_URL=http://localhost:8081"
set "FLEDGER_DUNNING_URL=http://localhost:8086"
set "FLEDGER_TENANT_ID=a0000000-0000-0000-0000-000000000001"
set "JWT_SECRET=fledger-super-secure-jwt-secret-key-2026-min-32"
set "WEBHOOK_SECRET=dunning-super-secret-key-2026"
start /B "C:\Dev\fledger\fledger-pay\src\pay.exe" > "%LOGDIR%\pay.out" 2>&1
echo Pay started

REM 4. Force
set "PORT=8084"
set "DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_force?sslmode=disable"
set "FLEDGER_CORE_URL=http://localhost:8081"
set "FLEDGER_TENANT_ID=a0000000-0000-0000-0000-000000000001"
set "JWT_SECRET=fledger-super-secure-jwt-secret-key-2026-min-32"
start /B "C:\Dev\fledger\fledger-force\src\force.exe" > "%LOGDIR%\force.out" 2>&1
echo Force started

REM 5. Order
set "PORT=8085"
set "DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_order?sslmode=disable"
set "FLEDGER_CORE_URL=http://localhost:8081"
set "FLEDGER_FLEET_URL=http://localhost:8082"
set "FLEDGER_TENANT_ID=a0000000-0000-0000-0000-000000000001"
set "JWT_SECRET=fledger-super-secure-jwt-secret-key-2026-min-32"
start /B "C:\Dev\fledger\fledger-order\src\order.exe" > "%LOGDIR%\order.out" 2>&1
echo Order started

REM 6. Dunning
set "PORT=8086"
set "DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_dunning?sslmode=disable"
set "FLEDGER_CORE_URL=http://localhost:8081"
set "FLEDGER_PAY_URL=http://localhost:8083"
set "FLEDGER_TENANT_ID=a0000000-0000-0000-0000-000000000001"
set "JWT_SECRET=fledger-super-secure-jwt-secret-key-2026-min-32"
set "PAY_WEBHOOK_SECRET=dunning-super-secret-key-2026"
set "WA_PROVIDER=MOCK"
set "WA_JITTER_MIN_SECONDS=0"
set "WA_JITTER_MAX_SECONDS=1"
start /B "C:\Dev\fledger\fledger-dunning\src\dunning.exe" > "%LOGDIR%\dunning.out" 2>&1
echo Dunning started

echo.
echo Waiting 3s for services to be ready...
timeout /t 3 /nobreak >nul
echo Done.