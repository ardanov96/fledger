# Integration Tests (Sprint 17)

> **Status:** ✅ Implemented. 5 actual E2E scenarios against real Postgres.
> **Build tag:** `integration` (skipped in `go test ./...`)

The integration test suite verifies end-to-end against real Postgres:
- **Defense-in-depth validation**: RLS policies (migration 000014) actually filter rows per tenant
- **Data invariants**: double-entry, conservation, trial balance, period close DB triggers
- **Concurrent safety**: 20 parallel transfers complete without lost updates
- **Tamper detection**: ledger entry immutability trigger from migration 000004

## Quick start (Linux + Docker)

```bash
# 1. Spin up ephemeral Postgres on port 5433
docker run -d --name pg-fmcg-test -p 5433:5432 \
    -e POSTGRES_PASSWORD=test \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_DB=fmcg_test \
    postgres:16

# Wait for it to be ready
until docker exec pg-fmcg-test pg_isready -U postgres; do sleep 1; done

# 2. Set DATABASE_URL + TEST_DATABASE_URL env
export DATABASE_URL="postgres://postgres:test@localhost:5433/fmcg_test?sslmode=disable"
export TEST_DATABASE_URL="$DATABASE_URL"

# 3. Apply migrations (CLI workflow)
go run ./cmd/migrator up

# 4. Run integration tests
go test -tags=integration -count=1 -v ./internal/usecase/...

# Stop and remove when done
docker stop pg-fmcg-test && docker rm pg-fmcg-test
```

## Test scenarios

| Test | Scope | What it verifies |
|---|---|---|
| `TestIntegration_TransferEndToEnd` | 1 transfer | `POST /v1/transfers` equivalent: 2 entries written, source debited, destination credited, RLS isolation between 2 tenants |
| `TestIntegration_ConcurrentTransfers` | 20 parallel transfers | No lost updates under `Sprint 6` concurrent stress pattern; conservation `src + dst = initial` |
| `TestIntegration_RLSIsolation` | Cross-tenant queries | Tenant A cannot list/get tenant B's accounts (defense-in-depth enabled) |
| `TestIntegration_PeriodCloseAndReconciler` | Period close invariant | After posting + manual `status='closed'`, DB trigger `no_post_to_closed_period` rejects new transactions |
| `TestIntegration_TamperDetection` | Direct DB UPDATE attempt | Imm. trigger `prevent_entry_modification()` blocks UPDATE on `ledger_entries`; hash chain verifies if bypassed |

## Files

- `internal/usecase/integration_test.go` — full E2E suite (`build-tag integration`)
- `migrations/000014_tenant_rls.up.sql` — RLS policies tested
- `migrations/000008_period_close.up.sql` — DB triggers tested
- `migrations/000004_create_entries.up.sql` — immutability trigger tested

## CI integration (planned Sprint 18+)

GitHub Actions job `integration-test` with `services.postgres.image = postgres:16`:

```yaml
integration-test:
  runs-on: ubuntu-latest
  services:
    postgres:
      image: postgres:16
      env:
        POSTGRES_PASSWORD: fmcg_test
        POSTGRES_DB: fmcg_test
      ports: [5432:5432]
      options: --health-cmd "pg_isready" --health-interval 5s
  env:
    TEST_DATABASE_URL: postgres://postgres:fmcg_test@localhost:5432/fmcg_test?sslmode=disable
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
    - run: make migrate-up-with-url
    - run: go test -tags=integration -count=1 -v ./...
```

Already partially prepared — see `.github/workflows/ci.yml` which has a `postgres:16-alpine` service. Sprint 18 will wire the `integration-test` step to run after unit tests.

## Why `TEST_DATABASE_URL` env (not testcontainers-go yet)?

`testcontainers-go` adds ~10 transitive deps + Docker SDK dependency. For Sprint 17, the manual Docker setup is sufficient — Docker is also already required for the `services` step in CI. testcontainers will be added in Sprint 18 alongside other CI infrastructure upgrades (`on_failure_docker_logs`, parallel sharding, etc.).

For local dev: `docker run` works on Linux/Mac/Windows-WSL2. Windows users can use `docker desktop` + WSL2 backend or run tests from inside WSL.

## Troubleshooting

### `pq: SSL is not enabled on the server`

`sslmode=disable` in connection string. Set explicitly:
```bash
export TEST_DATABASE_URL="postgres://postgres:test@localhost:5433/fmcg_test?sslmode=disable"
```

### `pq: password authentication failed`

Check `POSTGRES_PASSWORD` matches `TEST_DATABASE_URL` password.

### Test passes locally but fails on CI

Most likely cause: `TEST_DATABASE_URL` env not set in CI runner. Add to `.github/workflows/ci.yml`:
```yaml
env:
  TEST_DATABASE_URL: postgres://postgres:fmcg_test@localhost:5432/fmcg_test?sslmode=disable
```

### Hash chain test fail (Scenario 5)

This indicates the immutability trigger was bypassed (or migration 000004 not applied). Re-apply migrations:
```bash
go run ./cmd/migrator up
```

## Adding new integration tests

```go
//go:build integration

func TestIntegration_NewScenario(t *testing.T) {
    env := NewIntegrationTestEnv(t)
    env.cleanupTenant(t)  // start with clean state
    
    // ... use env.setTenantCtx() to bind GUC variables for RLS ...
    // ... use env.Pool for raw SQL checks ...
    // ... use postgres.NewXxxRepository(env.DB) for repo interactions ...
}
```

The build tag `integration` excludes these from regular `go test`. CI runs them via `-tags=integration`.
