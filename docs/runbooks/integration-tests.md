# Integration Tests (Sprint 17)

> **Status:** Skeleton ready. Full E2E suite deferred to Sprint 18+ (requires testcontainers-go + CI Docker infrastructure).

## Quick start (Linux with Docker)

```bash
# 1. Spin up ephemeral Postgres on port 5433
docker run -d --name pg-fmcg-test -p 5433:5432 \
    -e POSTGRES_PASSWORD=test \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_DB=fmcg_test \
    postgres:16

# Wait for it to be ready
until docker exec pg-fmcg-test pg_isready -U postgres; do sleep 1; done

# 2. Export DATABASE_URL
export TEST_DATABASE_URL="postgres://postgres:test@localhost:5433/fmcg_test?sslmode=disable"

# 3. Apply migrations
make migrate-up-with-url  # (or: DATABASE_URL=$TEST_DATABASE_URL go run ./cmd/migrator up)

# 4. Run integration tests
go test -tags=integration -count=1 ./internal/usecase/...
```

## What the suite covers (when fully implemented)

The full integration test suite verifies end-to-end against real Postgres:

1. **Happy-path transfer** — create 2 accounts, transfer 100 IDR, verify:
   - Both entries written correctly (debit source, credit destination)
   - Cached balance updated
   - Double-entry invariant `SUM(debit) == SUM(credit)` holds per period
   - Transaction status = `posted`

2. **Concurrent transfers** — 50 parallel transfers from same source, verify no lost updates (uses Sprint 6 stress test pattern but against real DB).

3. **Cross-currency transfer (Sprint 12)** — create USD + IDR account, transfer 100 USD with rate snapshot, verify FX fields populated and asymmetric entries.

4. **Period close + reconcile (Sprints 9 + 10)** — generate transactions, close period, run reconciler, verify `balanced` status + correct snapshots.

5. **Refresh token rotation (Sprint 13)** — login → rotate → reuse-detection revokes family.

6. **Tenant RLS (Sprint 15)** — set GUC variables, verify SELECT from tenant A cannot see tenant B rows.

## CI integration (future Sprint 18+)

GitHub Actions job (`integration-test`):

```yaml
integration-test:
  runs-on: ubuntu-latest
  services:
    postgres:
      image: postgres:16
      env:
        POSTGRES_PASSWORD: test
        POSTGRES_DB: fmcg_test
      ports: [5432:5432]
      options: --health-cmd "pg_isready -U postgres" --health-interval 5s
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
    - run: make migrate-up
      env:
        DATABASE_URL: postgres://postgres:test@localhost:5432/fmcg_test?sslmode=disable
    - run: go test -tags=integration -count=1 ./...
      env:
        TEST_DATABASE_URL: postgres://postgres:test@localhost:5432/fmcg_test?sslmode=disable
```

## Sprint 17 deliverables (current)

- `internal/usecase/integration_test.go` (build-tag `integration`) — placeholder + helper for connecting to test DB
- This runbook — describes setup + scope
- (Future) Full E2E suite — tracked as follow-up

## Why not testcontainers-go (yet)?

`testcontainers-go` adds ~10 transitive deps + Docker SDK dependency. For Sprint 17, the manual Docker setup above is sufficient. testcontainers will be added in Sprint 18 (alongside other CI infrastructure upgrades).
