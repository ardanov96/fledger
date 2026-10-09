@echo off
set APP_ENV=development
set PORT=8084
set DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_force?sslmode=disable
set DB_MAX_OPEN_CONNS=25
set DB_MAX_IDLE_CONNS=5
set FLEDGER_CORE_URL=http://localhost:8081
set FLEDGER_TENANT_ID=00000000-0000-0000-0000-000000000001
set FLEDGER_CORE_API_KEY=dev-force-key-fledger-2026
set JWT_SECRET=super-secret-fledger-force-jwt-key-minimum-32-chars!
set TOKEN_TTL=24h
set OUTBOX_POLL_INTERVAL=3s
set OUTBOX_REQUEST_TIMEOUT=5s
set OUTBOX_MAX_ATTEMPTS=10
set GOCACHE=C:\Dev\fledger\fledger-force\.gocache
set GOTMPDIR=C:\Dev\fledger\fledger-force\.gobuild
cd /d C:\Dev\fledger\fledger-force\src
"C:\Dev\fledger\fledger-force\.gobuild\force-server.exe"