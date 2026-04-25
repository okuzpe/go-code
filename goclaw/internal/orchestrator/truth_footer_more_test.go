package orchestrator

import "testing"

func TestSanitizeNarratedToolCallTextAssistantTranscript(t *testing.T) {
	input := "No changes yet.\n\n[assistant tool_use read_file]\n{\"path\":\"README.md\"}"
	got := sanitizeNarratedToolCallText(input)
	want := "No changes yet."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSanitizeNarratedToolCallTextStandaloneToolJSON(t *testing.T) {
	input := "{\n  \"name\": \"read_file\",\n  \"arguments\": {\n    \"path\": \"README.md\"\n  }\n}"
	got := sanitizeNarratedToolCallText(input)
	if got != "" {
		t.Fatalf("expected empty sanitized text, got %q", got)
	}
}
