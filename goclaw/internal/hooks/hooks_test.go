package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

type captureTransport struct {
	lastBody []byte
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	c.lastBody = b
	if ct := req.Header.Get("Content-Type"); ct != "application/json" {
		return nil, errors.New("want json content-type: " + ct)
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

func TestPreToolUseOnHTTPPostsJSONBody(t *testing.T) {
	tr := &captureTransport{}
	orig := http.DefaultClient.Transport
	http.DefaultClient.Transport = tr
	t.Cleanup(func() { http.DefaultClient.Transport = orig })

	reg := New()
	reg.OnHTTP(PreToolUse, "https://example.com/goclaw-hook-test", 5*time.Second)
	ctx := t.Context()
	err := reg.Fire(ctx, Event{Type: PreToolUse, ToolName: "read_file", Input: `{"path":"src/a.go"}`})
	if err != nil {
		t.Fatalf("Fire: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(tr.lastBody, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got["hook_event_name"] != string(PreToolUse) {
		t.Fatalf("hook_event_name: %v", got)
	}
	if got["tool_name"] != "read_file" {
		t.Fatalf("tool_name: %v", got)
	}
	if got["tool_input"] != `{"path":"src/a.go"}` {
		t.Fatalf("tool_input: %v", got)
	}
}

func TestPreToolUseOnCommandReceivesToolPayloadOnStdin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell for stdin capture")
	}
	dir := t.TempDir()
	outPath := filepath.Join(dir, "captured.json")
	reg := New()
	reg.OnCommand(PreToolUse, "sh", "-c", "cat > \""+outPath+"\"")
	ctx := t.Context()
	if err := reg.Fire(ctx, Event{Type: PreToolUse, ToolName: "bash", Input: `{"command":"echo"}`}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["tool_name"] != "bash" {
		t.Fatalf("tool_name: %v", got)
	}
	if got["tool_input"] != `{"command":"echo"}` {
		t.Fatalf("tool_input: %v", got)
	}
}

func TestPreToolUseExternalCommandExit1Blocks(t *testing.T) {
	reg := New()
	if runtime.GOOS == "windows" {
		reg.OnCommand(PreToolUse, "cmd.exe", "/c", "exit /b 1")
	} else {
		reg.OnCommand(PreToolUse, "/bin/sh", "-c", "exit 1")
	}
	ctx := t.Context()
	err := reg.Fire(ctx, Event{Type: PreToolUse, ToolName: "bash", Input: "{}"})
	if err == nil {
		t.Fatal("expected PreToolUse external hook exit 1 to block")
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
