// Package config loads runtime configuration for the Fledger Fleet service
// from environment variables. All sensitive values (DB URL, JWT secret,
// Fledger Core credentials) MUST be supplied through env vars — never
// hard-coded in source.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the immutable, validated runtime configuration for a single
// process start-up.
type Config struct {
	AppEnv  string
	Port    int
	DBURL    string

	DBMaxOpenConns int
	DBMaxIdleConns int

	FledgerCoreURL    string
	FledgerTenantID   string
	FledgerCoreAPIKey string

	JWTSecret string
	TokenTTL  time.Duration

	OutboxPollInterval     time.Duration
	OutboxRequestTimeout   time.Duration
	OutboxMaxAttempts      int
}

// Load reads the configuration from the process environment. It is intentionally
// strict: missing required values produce a hard error rather than a
// silently-degraded fallback.
func Load() (*Config, error) {
	c := &Config{
		AppEnv:                getEnv("APP_ENV", "development"),
		Port:                  getEnvInt("PORT", 8082),
		DBURL:                 os.Getenv("DATABASE_URL"),
		DBMaxOpenConns:        getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:        getEnvInt("DB_MAX_IDLE_CONNS", 10),
		FledgerCoreURL:        os.Getenv("FLEDGER_CORE_URL"),
		FledgerTenantID:       os.Getenv("FLEDGER_TENANT_ID"),
		FledgerCoreAPIKey:     os.Getenv("FLEDGER_CORE_API_KEY"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		TokenTTL:              time.Duration(getEnvInt("TOKEN_EXPIRY_HOURS", 24)) * time.Hour,
		OutboxPollInterval:    time.Duration(getEnvInt("OUTBOX_POLL_INTERVAL_SEC", 30)) * time.Second,
		OutboxRequestTimeout:  time.Duration(getEnvInt("OUTBOX_REQUEST_TIMEOUT_SEC", 10)) * time.Second,
		OutboxMaxAttempts:     getEnvInt("OUTBOX_MAX_ATTEMPTS", 8),
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	switch {
	case c.DBURL == "":
		return errors.New("config: DATABASE_URL is required")
	case c.FledgerCoreURL == "":
		return errors.New("config: FLEDGER_CORE_URL is required")
	case c.FledgerTenantID == "":
		return errors.New("config: FLEDGER_TENANT_ID is required")
	case c.JWTSecret == "" || len(c.JWTSecret) < 32:
		return errors.New("config: JWT_SECRET must be at least 32 characters")
	case c.Port <= 0 || c.Port > 65535:
		return errors.New("config: PORT must be a valid TCP port")
	case c.DBMaxOpenConns <= 0:
		return errors.New("config: DB_MAX_OPEN_CONNS must be > 0")
	case c.DBMaxIdleConns < 0:
		return errors.New("config: DB_MAX_IDLE_CONNS must be >= 0")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// String redacts sensitive fields. Safe to log at startup.
func (c *Config) String() string {
	return fmt.Sprintf("Config{env=%s port=%d core=%s tenant=%s outbox_poll=%s}",
		c.AppEnv, c.Port, c.FledgerCoreURL, c.FledgerTenantID, c.OutboxPollInterval)
}