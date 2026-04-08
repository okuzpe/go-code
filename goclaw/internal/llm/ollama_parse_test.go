package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseOllamaContentAsToolUse(t *testing.T) {
	specs := []ToolSpec{{Name: "web_search"}, {Name: "read_file"}}
	body := `{
  "name": "web_search",
  "arguments": {
    "query": "Hola que tal?"
  }
}`
	tu, ok := parseOllamaContentAsToolUse(body, specs)
	if !ok {
		t.Fatal("expected tool use from content JSON")
	}
	if tu.Name != "web_search" {
		t.Fatalf("name: %q", tu.Name)
	}
	if tu.Input == "" {
		t.Fatal("empty input")
	}
}

func TestParseOllamaContentAsToolUseRejectsUnknownTool(t *testing.T) {
	specs := []ToolSpec{{Name: "read_file"}}
	_, ok := parseOllamaContentAsToolUse(`{"name":"web_search","arguments":{"q":"x"}}`, specs)
	if ok {
		t.Fatal("expected reject for tool not in spec list")
	}
}

// Ollama /api/chat expects tool results as {"role":"tool","tool_name":"...","content":"..."}.
// Using "name" breaks the round-trip: the model never sees the result and prints fake JSON instead.
func TestOllamaWireToolMessageJSONUsesToolNameField(t *testing.T) {
	msgs := messageToOllama(Message{
		Role: "user",
		ToolResults: []ToolResultRecord{
			{ToolUseID: "x", ToolName: "web_search", Content: "search results here"},
		},
	})
	if len(msgs) != 1 || msgs[0].Role != "tool" {
		t.Fatalf("msgs: %+v", msgs)
	}
	raw, err := json.Marshal(msgs[0])
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"tool_name"`) || !strings.Contains(s, `"web_search"`) {
		t.Fatalf("expected tool_name in JSON: %s", s)
	}
}
