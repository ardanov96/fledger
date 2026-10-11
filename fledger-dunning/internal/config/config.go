// Package config loads runtime configuration for Fledger Dunning.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv     string
	Port       int
	DBURL      string

	JWTSecret          string
	TokenTTL           time.Duration
	PayWebhookSecret   string
	WAProvider         string
	WAJitterMinSeconds int
	WAJitterMaxSeconds int
	CoreBaseURL        string
	PayBaseURL         string
	FledgerTenantID    string
	OutboxPollInterval time.Duration
	OutboxMaxAttempts  int
}

func Load() (*Config, error) {
	c := &Config{
		AppEnv:             getEnv("APP_ENV", "development"),
		Port:               getEnvInt("PORT", 8086),
		DBURL:              getEnv("DATABASE_URL", "postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_dunning?sslmode=disable"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		TokenTTL:           getEnvDuration("TOKEN_TTL", 24*time.Hour),
		PayWebhookSecret:   getEnv("PAY_WEBHOOK_SECRET", getEnv("WEBHOOK_SECRET", "dunning-super-secret-key-2026")),
		WAProvider:         getEnv("WA_PROVIDER", "MOCK"),
		WAJitterMinSeconds: getEnvInt("WA_JITTER_MIN_SECONDS", 3),
		WAJitterMaxSeconds: getEnvInt("WA_JITTER_MAX_SECONDS", 8),
		CoreBaseURL:        getEnv("CORE_BASE_URL", getEnv("FLEDGER_CORE_URL", "http://localhost:8081")),
		PayBaseURL:         getEnv("PAY_BASE_URL", getEnv("FLEDGER_PAY_URL", "http://localhost:8083")),
		FledgerTenantID:    getEnv("FLEDGER_TENANT_ID", "a0000000-0000-0000-0000-000000000001"),
		OutboxPollInterval: getEnvDuration("OUTBOX_POLL_INTERVAL", 5*time.Second),
		OutboxMaxAttempts:  getEnvInt("OUTBOX_MAX_ATTEMPTS", 10),
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
	case c.JWTSecret == "" || len(c.JWTSecret) < 32:
		return errors.New("config: JWT_SECRET must be at least 32 characters")
	case c.PayWebhookSecret == "":
		return errors.New("config: PAY_WEBHOOK_SECRET is required")
	case c.Port <= 0 || c.Port > 65535:
		return errors.New("config: PORT must be a valid TCP port")
	case c.WAJitterMinSeconds < 0 || c.WAJitterMaxSeconds < c.WAJitterMinSeconds:
		return errors.New("config: WA_JITTER seconds invalid (min must be >= 0 and <= max)")
	}
	return nil
}

func (c *Config) String() string {
	return fmt.Sprintf("Config{env=%s port=%d core=%s wa=%s jitter=%d..%ds}",
		c.AppEnv, c.Port, c.CoreBaseURL, c.WAProvider, c.WAJitterMinSeconds, c.WAJitterMaxSeconds)
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvDuration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
