package orchestrator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
)

func TestSessionTokenEstimateByProvider(t *testing.T) {
	msgs := []llm.Message{llm.PlainMessage("user", strings.Repeat("a", 120))}
	if got := sessionTokenEstimate(msgs, "anthropic"); got != 40 {
		t.Fatalf("anthropic: got %d want 40", got)
	}
	if got := sessionTokenEstimate(msgs, "ollama"); got != 30 {
		t.Fatalf("ollama: got %d want 30", got)
	}
}

func TestContextBudgetTokens(t *testing.T) {
	if got := contextBudgetTokens("anthropic", 0); got != anthropicContextTokens {
		t.Fatalf("anthropic budget: got %d want %d", got, anthropicContextTokens)
	}
	if got := contextBudgetTokens("ollama", 0); got != ollamaContextTokens {
		t.Fatalf("ollama budget: got %d want %d", got, ollamaContextTokens)
	}
}

func TestContextBudgetTokensOverride(t *testing.T) {
	// model_context_tokens in settings.json overrides the provider default.
	if got := contextBudgetTokens("ollama", 8_000); got != 8_000 {
		t.Fatalf("override: got %d want 8000", got)
	}
	if got := contextBudgetTokens("anthropic", 50_000); got != 50_000 {
		t.Fatalf("override anthropic: got %d want 50000", got)
	}
}

func TestClearOldToolResults(t *testing.T) {
	makeMsg := func(role, content string, results ...string) llm.Message {
		m := llm.PlainMessage(role, content)
		for _, r := range results {
			m.ToolResults = append(m.ToolResults, llm.ToolResultRecord{
				ToolUseID: "id1",
				ToolName:  "read_file",
				Content:   r,
			})
		}
		return m
	}

	msgs := []llm.Message{
		makeMsg("user", "first", "big tool result content"),
		makeMsg("assistant", "second", "another large payload"),
		makeMsg("user", "third"), // in tail (preserve=2)
		makeMsg("assistant", "fourth"),
	}

	out, changed := clearOldToolResults(msgs, 2)
	if !changed {
		t.Fatal("expected changed=true")
	}
	// Messages outside the tail (indices 0 and 1) should have their tool result content cleared.
	if out[0].ToolResults[0].Content != compactedToolResult {
		t.Errorf("index 0: want %q, got %q", compactedToolResult, out[0].ToolResults[0].Content)
	}
	if out[1].ToolResults[0].Content != compactedToolResult {
		t.Errorf("index 1: want %q, got %q", compactedToolResult, out[1].ToolResults[0].Content)
	}
	// Messages inside the tail must be untouched.
	if out[2].Content != "third" {
		t.Errorf("tail[0] content changed unexpectedly")
	}
	if out[3].Content != "fourth" {
		t.Errorf("tail[1] content changed unexpectedly")
	}
}

func TestClearOldToolResultsNoOp(t *testing.T) {
	// When all messages fit within preserve, nothing is cleared.
	msgs := []llm.Message{
		llm.PlainMessage("user", "only message"),
	}
	_, changed := clearOldToolResults(msgs, 24)
	if changed {
		t.Fatal("expected changed=false when msgs <= preserve")
	}
}

func TestClearOldToolResultsIdempotent(t *testing.T) {
	// Calling twice should not mark changed on the second call.
	m := llm.PlainMessage("user", "msg")
	m.ToolResults = []llm.ToolResultRecord{{ToolUseID: "x", Content: "payload"}}
	msgs := []llm.Message{m, llm.PlainMessage("user", "tail")}

	msgs, _ = clearOldToolResults(msgs, 1)
	_, changed := clearOldToolResults(msgs, 1)
	if changed {
		t.Fatal("second pass should not report changed (already compacted)")
	}
}

func TestMaybeCompactPhase1OnlyAvoidsPhase2(t *testing.T) {
	// Huge tool payloads outside the preserved tail should be cleared first; if that
	// drops the estimated size below the threshold, phase-2 tail collapse must not run.
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.AutoCompactThreshold = 0.85

	sess := session.New()
	huge := strings.Repeat("x", 600_000)
	m0 := llm.PlainMessage("user", "u0")
	m0.ToolResults = []llm.ToolResultRecord{{ToolUseID: "1", ToolName: "read_file", Content: huge}}
	sess.Messages = append(sess.Messages, m0)
	m1 := llm.PlainMessage("assistant", "a1")
	m1.ToolResults = []llm.ToolResultRecord{{ToolUseID: "2", ToolName: "read_file", Content: huge}}
	sess.Messages = append(sess.Messages, m1)
	for i := 2; i < 26; i++ {
		sess.Add("user", fmt.Sprintf("short-%d", i))
	}

	orch := New(cfg, nil, sess, tools.New(), permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose)
	orch.maybeCompact()

	if len(sess.Messages) != 26 {
		t.Fatalf("phase 2 should not run: want 26 messages, got %d", len(sess.Messages))
	}
	if sess.Messages[0].ToolResults[0].Content != compactedToolResult {
		t.Fatalf("phase 1 should replace old tool content: got %q", sess.Messages[0].ToolResults[0].Content)
	}
	for _, m := range sess.Messages {
		if strings.Contains(m.Content, "[compaction]") {
			t.Fatal("unexpected phase-2 compaction summary in session")
		}
	}
}
