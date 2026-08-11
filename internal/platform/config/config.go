// Package config loads and validates application configuration.
//
// Configuration sources (in priority order):
//  1. Environment variables (highest priority)
//  2. .env file (development only)
//
// The loader is fail-fast: if any required value is missing or invalid,
// the application will panic at startup with a clear error message. We do NOT
// want production to start with default credentials or missing secrets.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	App        AppConfig
	DB         DBConfig
	Redis      RedisConfig
	NATS       NATSConfig
	JWT        JWTConfig
	BcryptCost int
	RateLimit  RateLimitConfig
	Telemetry  TelemetryConfig
	Tenant     TenantConfig
	Web        WebConfig
}

// AppConfig holds core application settings.
type AppConfig struct {
	Name        string
	Env         string // development | staging | production
	Port        int
	Version     string
	LogLevel    string
	LogFormat   string // json | text
	Timezone    string
}

// DBConfig holds PostgreSQL connection settings.
type DBConfig struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	StatementTimeout time.Duration
}

// DSN returns a PostgreSQL connection string suitable for pgx.
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&statement_timeout=%d&application_name=fmcg-wallet",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
		c.StatementTimeout.Milliseconds(),
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
}

// NATSConfig holds NATS JetStream connection settings.
type NATSConfig struct {
	URL             string
	StreamName      string
	StreamSubjects  string
	AckWait         time.Duration
	MaxDeliver      int
}

// JWTConfig holds JWT signing/validation settings.
type JWTConfig struct {
	Secret      string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
	Issuer      string
	Audience    string
}

// RateLimitConfig holds rate-limiting settings.
type RateLimitConfig struct {
	GlobalRPS   int
	GlobalBurst int
	LoginRPS    int
	LoginBurst  int
}

// TelemetryConfig holds observability settings.
type TelemetryConfig struct {
	OTLPEndpoint  string
	ServiceName   string
	SamplerRatio  float64
	MetricsEnabled bool
	MetricsPath    string
}

// TenantConfig holds tenant (multi-tenancy) settings.
type TenantConfig struct {
	DefaultID   string
	MultiTenant bool
}

// WebConfig holds frontend/CORS settings.
type WebConfig struct {
	Origin string
}

// Load reads configuration from .env file (if present) and environment
// variables. It returns a fully populated Config or an error if any required
// value is missing/invalid.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("APP_NAME", "fmcg-wallet")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", 8080)
	v.SetDefault("APP_VERSION", "0.0.0")
	v.SetDefault("APP_LOG_LEVEL", "info")
	v.SetDefault("APP_LOG_FORMAT", "json")
	v.SetDefault("APP_TZ", "Asia/Jakarta")

	v.SetDefault("DB_HOST", "localhost")
	v.SetDefault("DB_PORT", 5432)
	v.SetDefault("DB_NAME", "fmcg_wallet")
	v.SetDefault("DB_USER", "fmcg")
	v.SetDefault("DB_SSLMODE", "disable")
	v.SetDefault("DB_MAX_CONNS", 20)
	v.SetDefault("DB_MIN_CONNS", 2)
	v.SetDefault("DB_MAX_CONN_LIFETIME", "30m")
	v.SetDefault("DB_MAX_CONN_IDLE_TIME", "5m")
	v.SetDefault("DB_STATEMENT_TIMEOUT", "10s")

	v.SetDefault("REDIS_ADDR", "localhost:6379")
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("REDIS_POOL_SIZE", 10)

	v.SetDefault("NATS_URL", "nats://localhost:4222")
	v.SetDefault("NATS_STREAM_NAME", "FMCG")
	v.SetDefault("NATS_STREAM_SUBJECTS", "fmcg.>")
	v.SetDefault("NATS_ACK_WAIT", "30s")
	v.SetDefault("NATS_MAX_DELIVER", 5)

	v.SetDefault("JWT_ACCESS_TTL", "15m")
	v.SetDefault("JWT_REFRESH_TTL", "168h")
	v.SetDefault("JWT_ISSUER", "fmcg-wallet")
	v.SetDefault("JWT_AUDIENCE", "fmcg-wallet-api")

	v.SetDefault("BCRYPT_COST", 12)

	v.SetDefault("RATELIMIT_GLOBAL_RPS", 100)
	v.SetDefault("RATELIMIT_GLOBAL_BURST", 200)
	v.SetDefault("RATELIMIT_LOGIN_RPS", 5)
	v.SetDefault("RATELIMIT_LOGIN_BURST", 10)

	v.SetDefault("OTEL_SERVICE_NAME", "fmcg-wallet-api")
	v.SetDefault("OTEL_SAMPLER_RATIO", 0.1)
	v.SetDefault("METRICS_ENABLED", true)
	v.SetDefault("METRICS_PATH", "/metrics")

	v.SetDefault("TENANT_DEFAULT_ID", "00000000-0000-0000-0000-000000000001")
	v.SetDefault("MULTI_TENANT", false)

	v.SetDefault("WEB_ORIGIN", "http://localhost:3000")

	// Try to load .env file
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	if cwd, err := os.Getwd(); err == nil {
		v.AddConfigPath(filepath.Dir(cwd))
	}

	if err := v.ReadInConfig(); err != nil {
		// .env is optional in production (env vars are injected)
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	// Override with real env vars
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := &Config{
		App: AppConfig{
			Name:      v.GetString("APP_NAME"),
			Env:       v.GetString("APP_ENV"),
			Port:      v.GetInt("APP_PORT"),
			Version:   v.GetString("APP_VERSION"),
			LogLevel:  v.GetString("APP_LOG_LEVEL"),
			LogFormat: v.GetString("APP_LOG_FORMAT"),
			Timezone:  v.GetString("APP_TZ"),
		},
		DB: DBConfig{
			Host:             v.GetString("DB_HOST"),
			Port:             v.GetInt("DB_PORT"),
			Name:             v.GetString("DB_NAME"),
			User:             v.GetString("DB_USER"),
			Password:         v.GetString("DB_PASSWORD"),
			SSLMode:          v.GetString("DB_SSLMODE"),
			MaxConns:         int32(v.GetInt("DB_MAX_CONNS")),
			MinConns:         int32(v.GetInt("DB_MIN_CONNS")),
			MaxConnLifetime:  v.GetDuration("DB_MAX_CONN_LIFETIME"),
			MaxConnIdleTime:  v.GetDuration("DB_MAX_CONN_IDLE_TIME"),
			StatementTimeout: v.GetDuration("DB_STATEMENT_TIMEOUT"),
		},
		Redis: RedisConfig{
			Addr:     v.GetString("REDIS_ADDR"),
			Password: v.GetString("REDIS_PASSWORD"),
			DB:       v.GetInt("REDIS_DB"),
			PoolSize: v.GetInt("REDIS_POOL_SIZE"),
		},
		NATS: NATSConfig{
			URL:            v.GetString("NATS_URL"),
			StreamName:     v.GetString("NATS_STREAM_NAME"),
			StreamSubjects: v.GetString("NATS_STREAM_SUBJECTS"),
			AckWait:        v.GetDuration("NATS_ACK_WAIT"),
			MaxDeliver:     v.GetInt("NATS_MAX_DELIVER"),
		},
		JWT: JWTConfig{
			Secret:     v.GetString("JWT_SECRET"),
			AccessTTL:  v.GetDuration("JWT_ACCESS_TTL"),
			RefreshTTL: v.GetDuration("JWT_REFRESH_TTL"),
			Issuer:     v.GetString("JWT_ISSUER"),
			Audience:   v.GetString("JWT_AUDIENCE"),
		},
		BcryptCost: v.GetInt("BCRYPT_COST"),
		RateLimit: RateLimitConfig{
			GlobalRPS:   v.GetInt("RATELIMIT_GLOBAL_RPS"),
			GlobalBurst: v.GetInt("RATELIMIT_GLOBAL_BURST"),
			LoginRPS:    v.GetInt("RATELIMIT_LOGIN_RPS"),
			LoginBurst:  v.GetInt("RATELIMIT_LOGIN_BURST"),
		},
		Telemetry: TelemetryConfig{
			OTLPEndpoint:   v.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"),
			ServiceName:    v.GetString("OTEL_SERVICE_NAME"),
			SamplerRatio:   v.GetFloat64("OTEL_SAMPLER_RATIO"),
			MetricsEnabled: v.GetBool("METRICS_ENABLED"),
			MetricsPath:    v.GetString("METRICS_PATH"),
		},
		Tenant: TenantConfig{
			DefaultID:   v.GetString("TENANT_DEFAULT_ID"),
			MultiTenant: v.GetBool("MULTI_TENANT"),
		},
		Web: WebConfig{
			Origin: v.GetString("WEB_ORIGIN"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks that all required configuration values are present and
// sensible. Returns the first error encountered.
func (c *Config) Validate() error {
	if c.App.Name == "" {
		return errors.New("APP_NAME is required")
	}
	switch c.App.Env {
	case "development", "staging", "production":
	default:
		return fmt.Errorf("APP_ENV must be development|staging|production, got %q", c.App.Env)
	}
	if c.App.Port < 1 || c.App.Port > 65535 {
		return fmt.Errorf("APP_PORT out of range: %d", c.App.Port)
	}
	if c.DB.Host == "" {
		return errors.New("DB_HOST is required")
	}
	if c.DB.User == "" {
		return errors.New("DB_USER is required")
	}
	if c.App.Env == "production" && c.DB.Password == "" {
		return errors.New("DB_PASSWORD is required in production")
	}
	if c.App.Env == "production" && c.DB.SSLMode == "disable" {
		return errors.New("DB_SSLMODE=disable is not allowed in production")
	}
	if c.JWT.Secret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters (got %d)", len(c.JWT.Secret))
	}
	if c.BcryptCost < 4 || c.BcryptCost > 31 {
		return fmt.Errorf("BCRYPT_COST out of range: %d (must be 4-31)", c.BcryptCost)
	}
	return nil
}

// IsProduction returns true if running in production environment.
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// IsDevelopment returns true if running in development environment.
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// MustGetEnv is a helper for cmd entrypoints that need a single env value.
// Panics if missing. Use only in main.go for build-time vars.
func MustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic("required env var not set: " + key)
	}
	return val
}

// GetEnvInt parses an int env var with a default.
func GetEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return parsed
}
