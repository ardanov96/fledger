// Package infra provides external service clients and connection helpers.
package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/runut/fmcg-wallet/internal/platform/config"
)

// NewPGXPool creates a tuned pgxpool from the application config.
//
// Tuning rationale:
//   - MaxConns: 20 default is fine for most workloads. Increase for high-concurrency API.
//   - MinConns: 2 keeps a warm pool to avoid cold-start latency on first request.
//   - MaxConnLifetime: 30m rotates connections to avoid Postgres memory bloat
//     and to roll forward in case of server-side state changes.
//   - MaxConnIdleTime: 5m allows unused conns to close (saves resources).
//   - StatementTimeout: enforced at connection level (defense-in-depth; set per
//     statement in service code if you need different timeouts).
func NewPGXPool(ctx context.Context, cfg *config.DBConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	// DefaultQueryExecMode: "cache_statement" — better perf for repeated queries
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement

	// AfterConnect hook: set timezone for each new connection
	// (UTC for consistency; adjust per tenant later)
	originalAfterConnect := poolCfg.AfterConnect
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, "SET timezone = 'UTC'"); err != nil {
			return fmt.Errorf("set timezone: %w", err)
		}
		if originalAfterConnect != nil {
			return originalAfterConnect(ctx, conn)
		}
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Verify connectivity with a ping (fail-fast at startup)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
