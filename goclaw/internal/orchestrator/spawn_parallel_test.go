package orchestrator

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/llm"
)

func TestPendingToolsIncludeSpawnAgent(t *testing.T) {
	t.Parallel()
	if pendingToolsIncludeSpawnAgent([]llm.ToolUse{{Name: "read_file"}, {Name: "bash"}}) {
		t.Fatal("expected false")
	}
	if !pendingToolsIncludeSpawnAgent([]llm.ToolUse{{Name: "read_file"}, {Name: "spawn_agent"}}) {
		t.Fatal("expected true when spawn_agent present")
	}
}
