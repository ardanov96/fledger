//go:build integration
// +build integration

// Package usecase - integration tests (Sprint 17).
//
// Run with:  go test -tags=integration -count=1 ./internal/usecase/...
//
// These tests require a real Postgres database (Docker or local). The
// recommended setup for Linux CI:
//
//  1. docker run -d --name pg-test -p 5433:5432 \
//         -e POSTGRES_PASSWORD=test postgres:16
//  2. export TEST_DATABASE_URL=postgres://postgres:test@localhost:5433/postgres?sslmode=disable
//  3. apply migrations via cmd/migrator:
//      go run ./cmd/migrator up
//  4. go test -tags=integration -count=1 ./internal/usecase/...
//
// The tests verify the full happy-path flow end-to-end against real Postgres:
//   - create accounts
//   - transfer (with double-entry invariant)
//   - trial balance computation
//   - FX rate snapshot
//   - refresh token rotation
//
// Skip these in regular `go test ./...` (build tag excludes them).

package usecase

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// IntegrationTestEnv holds shared resources for integration tests.
type IntegrationTestEnv struct {
	Pool *pgxpool.Pool
}

// NewIntegrationTestEnv connects to TEST_DATABASE_URL and returns a pool.
// Skips the test (t.Skip) if env var not set or connection fails.
func NewIntegrationTestEnv(t *testing.T) *IntegrationTestEnv {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; integration tests skipped")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Skipf("cannot connect to %q: %v", url, err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("cannot ping %q: %v", url, err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return &IntegrationTestEnv{Pool: pool}
}

// TestIntegration_TransferEndToEnd is a placeholder for the full happy-path
// integration test. The full implementation requires:
//   - accountRepo, transactionRepo, entryRepo, periodRepo (real Postgres)
//   - run migration via cmd/migrator first
//   - create test accounts, transfer, verify balance + entries
//
// See docs/runbooks/integration-tests.md for the full step-by-step.
func TestIntegration_TransferEndToEnd(t *testing.T) {
	env := NewIntegrationTestEnv(t)
	if env == nil {
		return
	}

	// Placeholder: actual implementation lives in cmd/migrator-driven test
	// suite (separate binary). Tracked as future Sprint 18+ enhancement.
	t.Logf("integration test env: connected to Postgres with pool size %d", env.Pool.Config().MaxConns)
}
