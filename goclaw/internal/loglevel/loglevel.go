// Package loglevel maps GOCLAW_LOG to slog levels for cmd and internal packages.
package loglevel

import (
	"log/slog"
	"os"
	"strings"
)

// FromEnv returns the slog level for GOCLAW_LOG (debug, warn, error, or default info).
func FromEnv() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GOCLAW_LOG"))) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
