// Package tuilog redirects the default slog logger to a file during fullscreen TUI sessions
// so stderr writes do not corrupt the Bubble Tea display.
package tuilog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/loglevel"
)

// AttachSlogForTUI swaps slog.Default() to append to a log file for the TUI session.
// Logs go to GOCLAW_LOG_FILE when set, otherwise filepath.Join(userConfigDir, "logs", "goclaw.log").
// On failure, logs are discarded and one warning is printed to stderr.
// The returned function restores the previous default logger and closes the file when applicable.
func AttachSlogForTUI(userConfigDir string) (restore func()) {
	prev := slog.Default()
	level := loglevel.FromEnv()
	opts := &slog.HandlerOptions{Level: level}

	logPath := strings.TrimSpace(os.Getenv("GOCLAW_LOG_FILE"))
	if logPath == "" {
		logPath = filepath.Join(userConfigDir, "logs", "goclaw.log")
	} else {
		logPath = filepath.Clean(logPath)
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0750); err != nil {
		warnOpenFailed(logPath, err)
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, opts)))
		return func() { slog.SetDefault(prev) }
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		warnOpenFailed(logPath, err)
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, opts)))
		return func() { slog.SetDefault(prev) }
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(f, opts)))
	return func() {
		slog.SetDefault(prev)
		_ = f.Close()
	}
}

func warnOpenFailed(path string, err error) {
	fmt.Fprintf(os.Stderr, "goclaw: cannot open log file %s: %v (log output discarded for this TUI session)\n", path, err)
}
