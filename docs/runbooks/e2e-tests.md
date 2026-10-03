# E2E Test Runbook (Sprint 58)

**Goal:** Walk the full HTTP stack (router + middleware + repos + real Postgres) end-to-end via HTTP requests. Verifies the integration of all layers, not just individual units.

## When to use

- **Before deployment** — confirm auth + tenant isolation + double-entry transfer still work after any backend change
- **PR review** — required check for changes touching `cmd/api/`, `internal/handler/`, or `internal/middleware/`
- **Live demo confidence** — these 5 tests are the smoke set we run before a customer demo

## Prerequisites

```bash
# 1. Ephemeral Postgres
docker run -d --name pg-fmcg-e2e \
  -e POSTGRES_PASSWORD=test \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_DB=fmcg_test \
  -p 5433:5432 \
  postgres:16

# 2. Wait for ready
until docker exec pg-fmcg-e2e pg_isready -U postgres; do sleep 1; done

# 3. Apply migrations
export DATABASE_URL="postgres://postgres:test@localhost:5433/fmcg_test?sslmode=disable"
go run ./cmd/migrator up

# 4. Tell tests where to find it
export TEST_DATABASE_URL=$DATABASE_URL
```

## Run

```bash
# All E2E (Sprint 58: 6 tests)
make test-e2e

# Or directly:
go test -tags=integration -race -count=1 -v ./cmd/api/...

# Combined with other integration tests:
make test-integration
```

Expected output: 6 tests pass in ~3s total (skipped if TEST_DATABASE_URL unset).

## Test cases (cmd/api/e2e_test.go)

| Test | What it verifies |
|---|---|
| `TestE2E_LoginAndMe` | POST /v1/auth/login → 200 + httpOnly cookie. GET /v1/auth/me with Bearer → 200 + Principal. |
| `TestE2E_LoginWrongPassword_401` | Wrong password → 401 (no user enumeration leak). |
| `TestE2E_MeWithoutToken_401` | GET /v1/auth/me without Authorization → 401 (Sprint 57 wire-up). |
| `TestE2E_ListAccounts_TenantScoped` | GET /v1/accounts with Bearer → 200, returns only the caller's tenant's accounts (RLS Sprint 25-29). |
| `TestE2E_TransferDebitAndCredit` | POST /v1/transfers → 201, then verify accounts.cached_balance reflects debit+credit atomically. |
| `TestE2E_ProtectedRouteWithoutAuth_401` | Skipped (Sprint 58.1: RBAC enforcement test — current test harness skips the verify so we hit the DB-write path instead). |

## What's NOT covered (Sprint 58.1+ follow-ups)

- **Refresh token rotation** — Sprint 13: would need to assert old token rejected after rotation
- **MFA setup + verify** — Sprint 13: would need TOTP setup + verify round-trip
- **Period close workflow** — Sprint 9: 2-step approval
- **Concurrent transfers from same account** — Sprint 43 (SKIP LOCKED): two concurrent transfers don't double-spend
- **RBAC enforcement** — Sprint 5: sales_rep can't access admin endpoints (currently bypassed via test middleware)
- **Production cookie name config** — verify access_token httpOnly cookie is set by cmd/api/main.go's login handler

## CI integration

The `make test-integration` target runs all integration + E2E. Recommended GitHub Actions job:

```yaml
test-e2e:
  runs-on: ubuntu-latest
  services:
    postgres:
      image: postgres:16
      env:
        POSTGRES_PASSWORD: test
        POSTGRES_USER: postgres
        POSTGRES_DB: fmcg_test
      ports: ['5432:5432']
      options: --health-cmd "pg_isready" --health-interval 5s
  env:
    TEST_DATABASE_URL: postgres://postgres:test@localhost:5432/fmcg_test?sslmode=disable
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
    - run: go run ./cmd/migrator up
    - run: make test-integration
```

## Local dev loop

```bash
# Terminal 1: Postgres
docker run --rm -p 5433:5432 -e POSTGRES_PASSWORD=test postgres:16

# Terminal 2: backend (after migrations applied)
export TEST_DATABASE_URL="postgres://postgres:test@localhost:5433/fmcg_test?sslmode=disable"
make test-e2e
```

## Debugging tips

```bash
# Verbose output
go test -tags=integration -race -v -run TestE2E_LoginAndMe ./cmd/api/...

# Run with shorter timeout if a test hangs
go test -tags=integration -race -timeout 30s ./cmd/api/...

# Print SQL queries (via pg log) — set in deployment/docker-compose.yml
```

## Known limitations (Sprint 58.1+ follow-ups)

1. **No parallel test execution** — each test mutates the same tenant. If you add `-parallel N`, you'll get RLS conflicts. Use `-p 1` until Sprint 58.2 adds tenant-per-test.
2. **Hardcoded test user** — every test seeds the same `e2e_user` / `e2e_password_123`. If two tests run in the same DB they conflict. Each test creates its own tenant in the seed, so tenant_id varies.
3. **No cleanup** — tenant rows accumulate. Run `cleanup_tenants.sql` (TODO: create in Sprint 58.2) periodically.

## References

- Sprint 58 commit: `653be3f` (E2E test setup)
- Sprint 57: `/v1/auth/me` endpoint
- Sprint 25-29: RLS policies
- Sprint 43: FOR UPDATE SKIP LOCKED on outbox (not yet tested via E2E)