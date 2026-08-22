// Package logger configures the process-wide structured logger.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Init installs a JSON slog handler on stdout. Verbosity comes from
// TANULOM_LOG_LEVEL (debug|info|warn|error); anything else falls back to info.
func Init() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: levelFromEnv(),
	})))
}

func levelFromEnv() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TANULOM_LOG_LEVEL"))) {
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
