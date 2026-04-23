package tools

import (
	"context"
	"strings"
	"testing"
)

func TestToolSearchFindsHiddenMCPTools(t *testing.T) {
	reg := New()
	reg.Register(minimalTool{name: "read_file"})
	// MCP tools registered as hidden — they must not appear in Specs() but must be found by tool_search.
	reg.RegisterHidden(minimalTool{name: "mcp__github__list_prs"})
	reg.RegisterHidden(minimalTool{name: "mcp__slack__send_message"})

	// Specs() must exclude hidden MCP tools.
	for _, s := range reg.Specs() {
		if strings.HasPrefix(s.Name, "mcp__") {
			t.Fatalf("hidden MCP tool %q must not appear in Specs()", s.Name)
		}
	}

	ts, err := NewToolSearch(reg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ts.Execute(context.Background(), `{"query":"github prs","kind":"mcp"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsError {
		t.Fatalf("unexpected tool error: %s", got.Content)
	}
	if !strings.Contains(got.Content, "mcp__github__list_prs") {
		t.Fatalf("expected hidden github MCP tool in output: %s", got.Content)
	}
	if strings.Contains(got.Content, "read_file") {
		t.Fatalf("builtin tool should be filtered out for kind=mcp: %s", got.Content)
	}
}

func TestToolSearchLimitsAndSortsResults(t *testing.T) {
	reg := New()
	reg.RegisterHidden(minimalTool{name: "mcp__gitlab__search"})
	reg.RegisterHidden(minimalTool{name: "mcp__github__search_code"})
	reg.RegisterHidden(minimalTool{name: "mcp__github__search_issues"})

	ts, err := NewToolSearch(reg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ts.Execute(context.Background(), `{"query":"github search","limit":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsError {
		t.Fatalf("unexpected tool error: %s", got.Content)
	}
	if !strings.Contains(got.Content, "tool_search matches (1)") {
		t.Fatalf("expected single match header: %s", got.Content)
	}
	if !strings.Contains(got.Content, "mcp__github__search_code") && !strings.Contains(got.Content, "mcp__github__search_issues") {
		t.Fatalf("expected one github search tool: %s", got.Content)
	}
}

func TestToolSearchRejectsInvalidJSON(t *testing.T) {
	reg := New()
	ts, err := NewToolSearch(reg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ts.Execute(context.Background(), `{`)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsError {
		t.Fatalf("expected tool-facing error result")
	}
	if !strings.Contains(got.Content, "invalid json input") {
		t.Fatalf("unexpected error text: %s", got.Content)
	}
}
