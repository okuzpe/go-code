package tuilog

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttachSlogForTUIDefaultPath(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	dir := t.TempDir()
	logFile := filepath.Join(dir, "logs", "goclaw.log")

	t.Setenv("GOCLAW_LOG_FILE", "")
	restore := AttachSlogForTUI(dir)
	slog.Info("marker_token", "k", "v1")
	restore()

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "marker_token") || !strings.Contains(string(data), "k=v1") {
		t.Fatalf("log file missing expected record: %q", data)
	}
}

func TestAttachSlogForTUIRespectsGOCLAWLogFile(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	dir := t.TempDir()
	custom := filepath.Join(dir, "custom.log")
	t.Setenv("GOCLAW_LOG_FILE", custom)

	restore := AttachSlogForTUI(dir)
	slog.Info("custom_marker")
	restore()

	data, err := os.ReadFile(custom)
	if err != nil {
		t.Fatalf("read custom log: %v", err)
	}
	if !strings.Contains(string(data), "custom_marker") {
		t.Fatalf("custom log: %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "logs", "goclaw.log")); err == nil {
		t.Fatal("default log path should not be created when GOCLAW_LOG_FILE is set")
	}
}

func TestAttachSlogForTUIOpenFailureDiscardsWithoutPanic(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	dir := t.TempDir()
	blocked := filepath.Join(dir, "notadir")
	if err := os.WriteFile(blocked, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCLAW_LOG_FILE", filepath.Join(blocked, "nested.log"))

	restore := AttachSlogForTUI(dir)
	slog.Info("should_not_crash")
	restore()
}
