// Command migrator is the database migration tool for FMCG Wallet.
//
// It uses golang-migrate/migrate v4 to apply versioned SQL migrations from
// the migrations/ directory. Compatible with both file:// and embed://
// sources, and works against any PostgreSQL DSN.
//
// Usage:
//
//	migrator up                 — apply all pending migrations
//	migrator down [N]           — roll back N migrations (default: 1)
//	migrator status             — show current migration state
//	migrator version            — print current schema version
//	migrator force <version>    — force DB to a specific version (recovery only)
//	migrator goto <version>     — migrate to a specific version
//	migrator drop               — drop all tables (DANGEROUS; dev only)
//
// Environment:
//
//	DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD  — required
//	DB_SSLMODE                                     — optional (default: disable)
//	MIGRATIONS_DIR                                  — optional (default: ./migrations)
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/runut/fmcg-wallet/internal/platform/config"
	"github.com/runut/fmcg-wallet/internal/platform/logger"
)

// fileURLEscape converts an OS-native absolute path into a file:// URL that
// golang-migrate's `file` source parses correctly.
//
// Linux/Mac:    /abs/path           → file:///abs/path
// Windows:      D:\path             → file://D:/path   (legacy form)
//
// KNOWN ISSUE: golang-migrate v4's `file` source has trouble with Windows
// drive letters — the path component ends up empty after URL parsing and
// the library calls os.Open("."). The robust workaround for Windows dev is
// to run inside WSL2 / Docker (Linux) where this code works as-is. CI runs
// on Linux so production paths are unaffected.
func fileURLEscape(p string) string {
	return "file://" + filepath.ToSlash(p)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrator: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(logger.Config{Level: cfg.App.LogLevel, Format: cfg.App.LogFormat})
	slog.SetDefault(log)

	if len(os.Args) < 2 {
		usage()
		return errors.New("missing command")
	}

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		// Default: ./migrations relative to cwd; try a few common locations.
		candidates := []string{
			"./migrations",
			"../migrations",
			filepath.Join(filepath.Dir(os.Args[0]), "migrations"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				migrationsDir = c
				break
			}
		}
		if migrationsDir == "" {
			return errors.New("could not locate migrations directory; set MIGRATIONS_DIR")
		}
	}

	absDir, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("resolve migrations path: %w", err)
	}

	// Build the database URL for golang-migrate (uses the same config as the app).
	dbURL := buildMigrateURL(&cfg.DB)
	// Convert OS-native path into a file:// URL that golang-migrate parses
	// correctly on both Linux and Windows (see fileURLEscape comment).
	sourceURL := fileURLEscape(absDir)

	log.Info("migrator starting",
		"db", maskPassword(dbURL),
		"source", sourceURL,
	)

	m, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Warn("source close error", "error", srcErr)
		}
		if dbErr != nil {
			log.Warn("db close error", "error", dbErr)
		}
	}()

	cmd := os.Args[1]
	switch cmd {
	case "up":
		return runUp(m, log)
	case "down":
		return runDown(m, log)
	case "status":
		return runStatus(m, log)
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			return fmt.Errorf("version: %w", err)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
		return nil
	case "force":
		if len(os.Args) < 3 {
			return errors.New("force requires a version argument")
		}
		v, err := strconv.ParseUint(os.Args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid version: %w", err)
		}
		if err := m.Force(int(v)); err != nil {
			return fmt.Errorf("force: %w", err)
		}
		log.Info("forced version", "version", v)
		return nil
	case "goto":
		if len(os.Args) < 3 {
			return errors.New("goto requires a version argument")
		}
		v, err := strconv.ParseUint(os.Args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid version: %w", err)
		}
		if err := m.Migrate(uint(v)); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("goto: %w", err)
		}
		log.Info("migrated to version", "version", v)
		return nil
	case "drop":
		log.Warn("dropping all tables (DANGEROUS)")
		if err := m.Drop(); err != nil {
			return fmt.Errorf("drop: %w", err)
		}
		log.Info("all tables dropped")
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func runUp(m *migrate.Migrate, log *slog.Logger) error {
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("no migrations to apply (already at latest)")
			return nil
		}
		return fmt.Errorf("up: %w", err)
	}
	v, dirty, _ := m.Version()
	log.Info("migrations applied", "version", v, "dirty", dirty)
	return nil
}

func runDown(m *migrate.Migrate, log *slog.Logger) error {
	n := 1
	if len(os.Args) >= 3 {
		parsed, err := strconv.Atoi(os.Args[2])
		if err != nil || parsed < 1 {
			return fmt.Errorf("down requires positive integer, got %q", os.Args[2])
		}
		n = parsed
	}
	if err := m.Steps(-n); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("no migrations to roll back (already at base)")
			return nil
		}
		return fmt.Errorf("down: %w", err)
	}
	v, _, _ := m.Version()
	log.Info("migrations rolled back", "count", n, "version", v)
	return nil
}

func runStatus(m *migrate.Migrate, log *slog.Logger) error {
	v, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("no migrations applied yet (schema_migrations table empty)")
			return nil
		}
		return fmt.Errorf("status: %w", err)
	}
	fmt.Printf("current version: %d\n", v)
	fmt.Printf("dirty:             %v\n", dirty)
	return nil
}

// buildMigrateURL builds a postgres:// URL for golang-migrate.
// Format: postgres://user:pass@host:port/db?sslmode=...
func buildMigrateURL(c *config.DBConfig) string {
	host := c.Host
	if host == "" {
		host = "localhost"
	}
	port := c.Port
	if port == 0 {
		port = 5432
	}
	user := c.User
	if user == "" {
		user = "fmcg"
	}
	dbName := c.Name
	if dbName == "" {
		dbName = "fmcg_wallet"
	}
	sslmode := c.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user, c.Password, host, port, dbName, sslmode)
}

// maskPassword hides the password in a postgres URL for safe logging.
func maskPassword(url string) string {
	// Replace "user:pass@" with "user:***@"
	atIdx := strings.LastIndex(url, "@")
	if atIdx < 0 {
		return url
	}
	schemeIdx := strings.Index(url, "://")
	if schemeIdx < 0 {
		return url
	}
	credPart := url[schemeIdx+3 : atIdx]
	colonIdx := strings.Index(credPart, ":")
	if colonIdx < 0 {
		return url
	}
	return url[:schemeIdx+3] + credPart[:colonIdx] + ":***" + url[atIdx:]
}

func usage() {
	fmt.Fprintf(os.Stderr, `migrator — FMCG Wallet database migration tool

Usage:
  %s <command> [args]

Commands:
  up                 Apply all pending migrations
  down [N]           Roll back N migrations (default: 1)
  status             Show current migration state
  version            Print current schema version
  goto <version>     Migrate to a specific version
  force <version>    Force DB to a specific version (recovery only)
  drop               Drop all tables (DANGEROUS; dev only)
  help               Show this help

Environment:
  DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD   — required
  DB_SSLMODE                                       — optional (default: disable)
  MIGRATIONS_DIR                                   — optional (default: ./migrations)

Examples:
  migrator up
  migrator down 1
  migrator status
  migrator force 3
`, os.Args[0])
}
