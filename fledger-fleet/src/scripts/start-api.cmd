@echo off
set APP_ENV=development
set PORT=8082
set DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_fleet?sslmode=disable
set DB_MAX_OPEN_CONNS=25
set DB_MAX_IDLE_CONNS=10
set FLEDGER_CORE_URL=http://localhost:8081
set FLEDGER_TENANT_ID=00000000-0000-0000-0000-000000000001
set FLEDGER_CORE_API_KEY=fledger-fleet-service-key
set JWT_SECRET=super-secret-key-32-characters-minimum-fleet
set TOKEN_EXPIRY_HOURS=24
set OUTBOX_POLL_INTERVAL_SEC=15
set OUTBOX_REQUEST_TIMEOUT_SEC=10
set OUTBOX_MAX_ATTEMPTS=8
cd /d C:\Dev\fledger\fledger-fleet\src
go run ./cmd/api