// Package main is a simple migrator for Fledger Pay. Applies SQL files
// from migrations/ in order, tracking state in pay_schema_migrations.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fledger/fledger-pay/internal/config"
	"github.com/fledger/fledger-pay/internal/platform/log"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fledger-pay migrator fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	logger := log.New("info")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DBURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	migrationsDir := "migrations"
	if v := os.Getenv("MIGRATIONS_DIR"); v != "" {
		migrationsDir = v
	}

	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS pay_schema_migrations (
			version INT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			description TEXT
		)`); err != nil {
		return fmt.Errorf("bootstrap migrations table: %w", err)
	}

	switch cmd {
	case "up":
		return runUp(ctx, pool, migrationsDir, logger)
	case "status":
		return runStatus(ctx, pool, migrationsDir, logger)
	case "down":
		steps := 1
		if len(os.Args) > 2 {
			if n, err := strconv.Atoi(os.Args[2]); err == nil {
				steps = n
			}
		}
		return runDown(ctx, pool, migrationsDir, logger, steps)
	default:
		return fmt.Errorf("unknown command %q (use up|down|status)", cmd)
	}
}

type migration struct {
	Version int
	Name    string
	Path    string
}

func listMigrations(dir string) ([]migration, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	out := make([]migration, 0, len(files))
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		name := f.Name()
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		out = append(out, migration{Version: v, Name: name, Path: filepath.Join(dir, name)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[int]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM pay_schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func runUp(ctx context.Context, pool *pgxpool.Pool, dir string, logger *slog.Logger) error {
	migs, err := listMigrations(dir)
	if err != nil {
		return err
	}
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}
	for _, m := range migs {
		if applied[m.Version] {
			logger.Info("skip", "version", m.Version, "name", m.Name)
			continue
		}
		logger.Info("apply", "version", m.Version, "name", m.Name)
		raw, err := os.ReadFile(m.Path)
		if err != nil {
			return fmt.Errorf("read %s: %w", m.Path, err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			return fmt.Errorf("apply %s: %w", m.Name, err)
		}
	}
	logger.Info("migrations: up-to-date")
	return nil
}

func runStatus(ctx context.Context, pool *pgxpool.Pool, dir string, logger *slog.Logger) error {
	migs, err := listMigrations(dir)
	if err != nil {
		return err
	}
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}
	for _, m := range migs {
		state := "PENDING"
		if applied[m.Version] {
			state = "APPLIED"
		}
		logger.Info("status", "version", m.Version, "name", m.Name, "state", state)
	}
	return nil
}

func runDown(ctx context.Context, pool *pgxpool.Pool, dir string, logger *slog.Logger, steps int) error {
	rows, err := pool.Query(ctx,
		`SELECT version FROM pay_schema_migrations ORDER BY version DESC LIMIT $1`, steps)
	if err != nil {
		return err
	}
	defer rows.Close()
	var versions []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return err
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, v := range versions {
		if _, err := pool.Exec(ctx, `DELETE FROM pay_schema_migrations WHERE version = $1`, v); err != nil {
			return err
		}
		logger.Info("rolled back tracking row (no DDL drop)", "version", v)
	}
	if len(versions) == 0 {
		return errors.New("nothing to roll back")
	}
	return nil
}