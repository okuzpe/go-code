package llm

import (
	"strings"
	"testing"
)

func TestBuildOllamaChatRequestTextOnlyFallbackAppendsNote(t *testing.T) {
	t.Parallel()
	req := Request{
		Model:  "m",
		System: "BASE",
		Tools: []ToolSpec{
			{Name: "glob", Description: "x", InputSchema: map[string]any{"type": "object"}},
		},
	}
	withWire := buildOllamaChatRequest(req, true)
	textOnly := buildOllamaChatRequest(req, false)

	if got := withWire.Messages[0].Content; got != "BASE" {
		t.Fatalf("with wire tools: system should be unchanged, got %q", got)
	}
	if !strings.Contains(textOnly.Messages[0].Content, "OLLAMA TEXT-ONLY FALLBACK") {
		t.Fatalf("text-only retry should append tooling note, got %q", textOnly.Messages[0].Content)
	}
	if !strings.Contains(textOnly.Messages[0].Content, "BASE") {
		t.Fatal("expected original system preserved before note")
	}
}

func TestBuildOllamaChatRequestNoNoteWhenToolsEmpty(t *testing.T) {
	t.Parallel()
	req := Request{Model: "m", System: "BASE", Tools: nil}
	got := buildOllamaChatRequest(req, false).Messages[0].Content
	if strings.Contains(got, "OLLAMA TEXT-ONLY") {
		t.Fatalf("unexpected note when no tools: %q", got)
	}
}
