@echo off
set BASE=C:\Dev\fledger
set OUT=%TEMP%\fledger-binaries
if not exist "%OUT%" mkdir "%OUT%"

echo Building all 6 binaries...

set GOFLAGS=-trimpath -ldflags=-s -w
set GOCACHE=%TEMP%\fledger-gocache
set GOTMPDIR=%TEMP%\fledger-gotmp

cd /d "%BASE%\fledger-core"
go build -o "%OUT%\core.exe" ./cmd/api
if errorlevel 1 ( echo BUILD FAIL: core & exit /b 1 )

cd /d "%BASE%\fledger-fleet\src"
go build -o "%OUT%\fleet.exe" ./cmd/api
if errorlevel 1 ( echo BUILD FAIL: fleet & exit /b 1 )

cd /d "%BASE%\fledger-pay\src"
go build -o "%OUT%\pay.exe" ./cmd/api
if errorlevel 1 ( echo BUILD FAIL: pay & exit /b 1 )

cd /d "%BASE%\fledger-force\src"
go build -o "%OUT%\force.exe" ./cmd/api
if errorlevel 1 ( echo BUILD FAIL: force & exit /b 1 )

cd /d "%BASE%\fledger-order\src"
go build -o "%OUT%\order.exe" ./cmd/api
if errorlevel 1 ( echo BUILD FAIL: order & exit /b 1 )

cd /d "%BASE%\fledger-dunning"
go build -o "%OUT%\dunning.exe" ./cmd/server
if errorlevel 1 ( echo BUILD FAIL: dunning & exit /b 1 )

echo.
echo Copying binaries to source roots...
copy /Y "%OUT%\core.exe"    "%BASE%\fledger-core\core.exe"        >nul
copy /Y "%OUT%\fleet.exe"   "%BASE%\fledger-fleet\src\fleet.exe"      >nul
copy /Y "%OUT%\pay.exe"     "%BASE%\fledger-pay\src\pay.exe"          >nul
copy /Y "%OUT%\force.exe"   "%BASE%\fledger-force\src\force.exe"      >nul
copy /Y "%OUT%\order.exe"   "%BASE%\fledger-order\src\order.exe"      >nul
copy /Y "%OUT%\dunning.exe" "%BASE%\fledger-dunning\dunning.exe"  >nul

echo All 6 binaries built and copied.