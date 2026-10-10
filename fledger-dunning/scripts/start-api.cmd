@echo off
set APP_ENV=development
set PORT=8086
set LOG_LEVEL=debug
set DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_dunning?sslmode=disable
set DB_MAX_OPEN_CONNS=25
set DB_MIN_CONNS=5
set JWT_SECRET=fledger-super-secure-jwt-secret-key-2026-min-32
set PAY_WEBHOOK_SECRET=dunning-super-secret-key-2026
set WA_PROVIDER=MOCK
set WA_JITTER_MIN_SECONDS=3
set WA_JITTER_MAX_SECONDS=8
set FLEDGER_CORE_URL=http://localhost:8081
set FLEDGER_TENANT_ID=a0000000-0000-0000-0000-000000000001
set FLEDGER_PAY_URL=http://localhost:8083
set OUTBOX_POLL_INTERVAL=5s
set OUTBOX_MAX_ATTEMPTS=10
set TOKEN_TTL=24h
set GOCACHE=C:\Dev\fledger\fledger-dunning\.gocache
set GOTMPDIR=C:\Dev\fledger\fledger-dunning\.gobuild
cd /d C:\Dev\fledger\fledger-dunning
go run ./cmd/server