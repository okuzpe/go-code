package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
