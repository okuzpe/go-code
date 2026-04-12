package loglevel

import (
	"log/slog"
	"os"
	"testing"
)

func TestFromEnv(t *testing.T) {
	prev := os.Getenv("GOCLAW_LOG")
	t.Cleanup(func() {
		if prev == "" {
			_ = os.Unsetenv("GOCLAW_LOG")
		} else {
			_ = os.Setenv("GOCLAW_LOG", prev)
		}
	})

	tests := []struct {
		env  string
		want slog.Level
	}{
		{"", slog.LevelInfo},
		{"DEBUG", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			if tt.env == "" {
				_ = os.Unsetenv("GOCLAW_LOG")
			} else {
				t.Setenv("GOCLAW_LOG", tt.env)
			}
			if got := FromEnv(); got != tt.want {
				t.Fatalf("FromEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
