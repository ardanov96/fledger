package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv  string
	Port    int
	DBURL    string

	DBMaxOpenConns int
	DBMaxIdleConns int

	FledgerCoreURL    string
	FledgerTenantID   string
	FledgerCoreAPIKey string

	FledgerFleetURL string

	JWTSecret            string
	TokenTTL             time.Duration
	ManagerOverridePIN   string

	OutboxPollInterval   time.Duration
	OutboxRequestTimeout time.Duration
	OutboxMaxAttempts    int
}

func Load() (*Config, error) {
	c := &Config{
		AppEnv:                getEnv("APP_ENV", "development"),
		Port:                  getEnvInt("PORT", 8085),
		DBURL:                 os.Getenv("DATABASE_URL"),
		DBMaxOpenConns:        getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:        getEnvInt("DB_MAX_IDLE_CONNS", 5),
		FledgerCoreURL:        os.Getenv("FLEDGER_CORE_URL"),
		FledgerTenantID:       os.Getenv("FLEDGER_TENANT_ID"),
		FledgerCoreAPIKey:     os.Getenv("FLEDGER_CORE_API_KEY"),
		FledgerFleetURL:       os.Getenv("FLEDGER_FLEET_URL"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		TokenTTL:              getEnvDuration("TOKEN_TTL", 24*time.Hour),
		ManagerOverridePIN:   getEnv("MANAGER_OVERRIDE_PIN", "123456"),
		OutboxPollInterval:    getEnvDuration("OUTBOX_POLL_INTERVAL", 3*time.Second),
		OutboxRequestTimeout:  getEnvDuration("OUTBOX_REQUEST_TIMEOUT", 5*time.Second),
		OutboxMaxAttempts:     getEnvInt("OUTBOX_MAX_ATTEMPTS", 10),
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
	case c.FledgerFleetURL == "":
		return errors.New("config: FLEDGER_FLEET_URL is required")
	case c.JWTSecret == "" || len(c.JWTSecret) < 32:
		return errors.New("config: JWT_SECRET must be at least 32 characters")
	case c.ManagerOverridePIN == "":
		return errors.New("config: MANAGER_OVERRIDE_PIN is required")
	case c.Port <= 0 || c.Port > 65535:
		return errors.New("config: PORT must be a valid TCP port")
	}
	return nil
}

func (c *Config) String() string {
	return fmt.Sprintf("Config{env=%s port=%d core=%s fleet=%s tenant=%s outbox_poll=%s}",
		c.AppEnv, c.Port, c.FledgerCoreURL, c.FledgerFleetURL, c.FledgerTenantID, c.OutboxPollInterval)
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