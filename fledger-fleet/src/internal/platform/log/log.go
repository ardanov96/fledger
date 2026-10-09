// Package log configures the process-wide structured logger.
package log

import (
	"log/slog"
	"os"
)

// New returns a JSON slog logger writing to stderr at the supplied level.
// Level strings: "debug", "info", "warn", "error".
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}