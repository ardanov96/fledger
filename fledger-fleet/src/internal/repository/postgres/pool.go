// Package postgres implements the persistence layer for Fledger Fleet using
// the pgx/v5 driver. Repositories here wrap the database/sql-like API of pgx
// and translate rows to domain models.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fledger/fledger-fleet/internal/config"
)

// NewPool builds and validates a pgx connection pool with the limits from cfg.
func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DBURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	pcfg.MaxConns = int32(cfg.DBMaxOpenConns)
	pcfg.MinConns = int32(cfg.DBMaxIdleConns)
	pcfg.HealthCheckPeriod = 30 * time.Second
	pcfg.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}