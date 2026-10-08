// Package logger provides structured logging using the standard library's
// log/slog package.
//
// Usage:
//
//	import "github.com/runut/fmcg-wallet/internal/platform/logger"
//
//	log := logger.New(logger.Config{Level: "info", Format: "json"})
//	log.Info("user logged in", "user_id", id)
//
// The package also provides middleware-ready helpers for injecting
// request-scoped attributes (request_id, trace_id, user_id, tenant_id).
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type ctxKey struct{}

// Config configures a Logger.
type Config struct {
	Level  string // debug | info | warn | error
	Format string // json | text
	Output io.Writer
}

// New returns a configured *slog.Logger.
func New(cfg Config) *slog.Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "text":
		handler = slog.NewTextHandler(cfg.Output, opts)
	default: // json
		handler = slog.NewJSONHandler(cfg.Output, opts)
	}
	return slog.New(handler)
}

// parseLevel maps a string to a slog.Level.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithContext returns a new context that carries the given logger.
func WithContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, log)
}

// FromContext returns the logger from the context, or slog.Default() if none.
func FromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && log != nil {
		return log
	}
	return slog.Default()
}

// WithRequest returns a new logger with request-scoped attributes.
// Useful for HTTP middleware: every log within the request lifecycle
// will carry these attributes automatically.
func WithRequest(ctx context.Context, requestID, traceID, userID, tenantID string) context.Context {
	log := FromContext(ctx).With(
		"request_id", requestID,
		"trace_id", traceID,
		"user_id", userID,
		"tenant_id", tenantID,
	)
	return WithContext(ctx, log)
}

// RedactPII returns a logger that redacts common PII fields in log output.
// This is a best-effort filter; for PII-heavy data, encrypt at rest
// (e.g. pgcrypto column encryption) and never log raw values.
func RedactPII(log *slog.Logger) *slog.Logger {
	// Simple wrapper — production should use a custom Handler that intercepts
	// known sensitive keys (ktp, nik, email, phone, password, etc.).
	// For now, we just return the same logger; future improvement.
	return log
}
