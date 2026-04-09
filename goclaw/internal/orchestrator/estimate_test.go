package orchestrator

import (
	"context"
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
	"github.com/stretchr/testify/require"
)

func TestSessionTokenEstimateByProvider(t *testing.T) {
	t.Parallel()
	msgs := []llm.Message{llm.PlainMessage("user", strings.Repeat("a", 120))}
	require.Equal(t, 40, sessionTokenEstimate(msgs, "anthropic"))
	require.Equal(t, 30, sessionTokenEstimate(msgs, "ollama"))
}

func TestContextBudgetTokens(t *testing.T) {
	t.Parallel()
	require.Equal(t, anthropicContextTokens, contextBudgetTokens("anthropic", 0))
	require.Equal(t, ollamaContextTokens, contextBudgetTokens("ollama", 0))
}

func TestContextBudgetTokensOverride(t *testing.T) {
	t.Parallel()
	// model_context_tokens in settings.json overrides the provider default.
	require.Equal(t, 8_000, contextBudgetTokens("ollama", 8_000))
	require.Equal(t, 50_000, contextBudgetTokens("anthropic", 50_000))
}

func TestClearOldToolResults(t *testing.T) {
	t.Parallel()
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
	require.True(t, changed)
	// Messages outside the tail (indices 0 and 1) should have their tool result content cleared.
	require.Equal(t, compactedToolResult, out[0].ToolResults[0].Content)
	require.Equal(t, compactedToolResult, out[1].ToolResults[0].Content)
	// Messages inside the tail must be untouched.
	require.Equal(t, "third", out[2].Content)
	require.Equal(t, "fourth", out[3].Content)
}

func TestClearOldToolResultsNoOp(t *testing.T) {
	t.Parallel()
	// When all messages fit within preserve, nothing is cleared.
	msgs := []llm.Message{
		llm.PlainMessage("user", "only message"),
	}
	_, changed := clearOldToolResults(msgs, 24)
	require.False(t, changed)
}

func TestClearOldToolResultsIdempotent(t *testing.T) {
	t.Parallel()
	// Calling twice should not mark changed on the second call.
	m := llm.PlainMessage("user", "msg")
	m.ToolResults = []llm.ToolResultRecord{{ToolUseID: "x", Content: "payload"}}
	msgs := []llm.Message{m, llm.PlainMessage("user", "tail")}

	msgs, _ = clearOldToolResults(msgs, 1)
	_, changed := clearOldToolResults(msgs, 1)
	require.False(t, changed)
}

type stubTokenCounter struct {
	value     int
	err       error
	callCount int
}

func (s *stubTokenCounter) CountInputTokens(_ context.Context, _ llm.Request) (int, error) {
	s.callCount++
	return s.value, s.err
}

func TestEstimatedSessionTokensSkipsAPIBelowSoftThreshold(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.ModelContextTokens = 1000
	cfg.AutoCompactThreshold = 0.85
	cfg.TokenCountMode = "auto"

	sess := session.New()
	sess.Add("user", strings.Repeat("a", 100))

	counter := &stubTokenCounter{value: 999}
	orch := New(cfg, nil, sess, tools.New(), permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose,
		WithInputTokenCounter(counter))

	limit := int(float64(1000) * 0.85)
	got := orch.estimatedSessionTokens(context.Background(), limit)
	require.Equal(t, sessionTokenEstimate(sess.Messages, "anthropic"), got)
	require.Zero(t, counter.callCount)
}

func TestEstimatedSessionTokensUsesCounterAboveSoftThreshold(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.ModelContextTokens = 1000
	cfg.AutoCompactThreshold = 0.85
	cfg.TokenCountMode = "auto"

	sess := session.New()
	sess.Add("user", strings.Repeat("a", 2500))

	counter := &stubTokenCounter{value: 812}
	orch := New(cfg, nil, sess, tools.New(), permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose,
		WithInputTokenCounter(counter))

	limit := int(float64(1000) * 0.85)
	got := orch.estimatedSessionTokens(context.Background(), limit)
	require.Equal(t, 812, got)
	require.Equal(t, 1, counter.callCount)
}

func TestEstimatedSessionTokensHeuristicMode(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.ModelContextTokens = 1000
	cfg.TokenCountMode = "heuristic"

	sess := session.New()
	sess.Add("user", strings.Repeat("a", 2500))

	counter := &stubTokenCounter{value: 812}
	orch := New(cfg, nil, sess, tools.New(), permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose,
		WithInputTokenCounter(counter))

	limit := int(float64(1000) * 0.85)
	got := orch.estimatedSessionTokens(context.Background(), limit)
	require.Equal(t, sessionTokenEstimate(sess.Messages, "anthropic"), got)
	require.Zero(t, counter.callCount)
}

func TestMaybeCompactPhase1OnlyAvoidsPhase2(t *testing.T) {
	t.Parallel()
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
	orch.maybeCompact(context.Background())

	require.Len(t, sess.Messages, 26, "phase 2 should not run")
	require.Equal(t, compactedToolResult, sess.Messages[0].ToolResults[0].Content)
	for _, m := range sess.Messages {
		if strings.Contains(m.Content, "[compaction]") {
			require.Fail(t, "unexpected phase-2 compaction summary in session")
		}
	}
}
