package mcp_test

import (
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/mcp"
)

func TestNormalizeMCPToolName(t *testing.T) {
	got := mcp.NormalizeMCPToolName("my-server", "do_thing")
	if got != "mcp__my-server__do_thing" {
		t.Fatalf("got %q", got)
	}
	got2 := mcp.NormalizeMCPToolName("a/b", "x")
	if !strings.HasPrefix(got2, "mcp__") || !strings.Contains(got2, "__x") {
		t.Fatalf("sanitize failed: %q", got2)
	}
}
