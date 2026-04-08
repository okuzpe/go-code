package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseEventType(t *testing.T) {
	tests := []struct {
		in   string
		want EventType
	}{
		{"pre_tool_use", PreToolUse},
		{"PRE_TOOL_USE", PreToolUse},
		{"session_start", SessionStart},
	}
	for _, tt := range tests {
		got, err := ParseEventType(tt.in)
		if err != nil {
			t.Fatalf("ParseEventType(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("ParseEventType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if _, err := ParseEventType("nope"); err == nil {
		t.Fatal("expected error for unknown event")
	}
}

func TestLoadHooksFile_registersCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	var entry any
	if runtime.GOOS == "windows" {
		entry = map[string]any{"event": "session_start", "command": "cmd.exe", "args": []string{"/c", "exit", "0"}}
	} else {
		entry = map[string]any{"event": "session_start", "command": "/bin/true"}
	}
	raw, err := json.Marshal([]any{entry})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	reg := New()
	if err := LoadHooksFile(reg, path); err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	if err := reg.Fire(ctx, Event{Type: SessionStart}); err != nil {
		t.Fatal(err)
	}
}

func TestPreToolUseGoHandlerBlocks(t *testing.T) {
	reg := New()
	boom := errors.New("policy hook")
	reg.On(PreToolUse, func(ctx context.Context, e Event) error {
		return boom
	})
	ctx := t.Context()
	err := reg.Fire(ctx, Event{Type: PreToolUse, ToolName: "bash", Input: "{}"})
	if !errors.Is(err, boom) {
		t.Fatalf("Fire: got %v want %v", err, boom)
	}
}

func TestPreToolUseExternalCommandExit2Blocks(t *testing.T) {
	reg := New()
	if runtime.GOOS == "windows" {
		reg.OnCommand(PreToolUse, "cmd.exe", "/c", "exit /b 2")
	} else {
		reg.OnCommand(PreToolUse, "/bin/sh", "-c", "exit 2")
	}
	ctx := t.Context()
	err := reg.Fire(ctx, Event{Type: PreToolUse, ToolName: "bash", Input: "{}"})
	if err == nil {
		t.Fatal("expected PreToolUse external hook exit 2 to block")
	}
	msg := err.Error()
	if !strings.Contains(msg, "blocked") && !strings.Contains(msg, "exit 2") {
		t.Fatalf("expected block message, got: %v", err)
	}
}
