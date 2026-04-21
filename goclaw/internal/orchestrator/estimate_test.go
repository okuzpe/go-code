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

func TestSessionTokenEstimateHeuristic(t *testing.T) {
	tests := []struct {
		name         string
		contentChars int
		wantTok      int
	}{
		{name: "char count divides by four", contentChars: 120, wantTok: 30},
		{name: "single byte rounds up token bucket", contentChars: 1, wantTok: 1},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msgs := []llm.Message{llm.PlainMessage("user", strings.Repeat("a", tt.contentChars))}
			require.Equal(t, tt.wantTok, sessionTokenEstimate(msgs))
			require.Equal(t, tt.wantTok, SessionMessagesTokenEstimate(msgs))
		})
	}
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

func TestClearOldToolResultsEdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		preserve        int
		build           func() []llm.Message
		wantFirstClear  bool
		runSecondClear  bool
		wantSecondClear bool
	}{
		{
			name:           "no change when all messages fit within preserve window",
			preserve:       24,
			build:          func() []llm.Message { return []llm.Message{llm.PlainMessage("user", "only message")} },
			wantFirstClear: false,
			runSecondClear: false,
		},
		{
			name:     "second clear is no-op after payloads are already compacted",
			preserve: 1,
			build: func() []llm.Message {
				head := llm.PlainMessage("user", "msg")
				head.ToolResults = []llm.ToolResultRecord{{ToolUseID: "x", Content: "payload"}}
				return []llm.Message{head, llm.PlainMessage("user", "tail")}
			},
			wantFirstClear:  true,
			runSecondClear:  true,
			wantSecondClear: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msgs := tt.build()
			_, first := clearOldToolResults(msgs, tt.preserve)
			require.Equal(t, tt.wantFirstClear, first)
			if tt.runSecondClear {
				_, second := clearOldToolResults(msgs, tt.preserve)
				require.Equal(t, tt.wantSecondClear, second)
			}
		})
	}
}

func TestEstimatedSessionTokensHeuristicOnly(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Provider = "ollama"
	sess := session.New()
	sess.Add("user", strings.Repeat("a", 400))
	orch := New(cfg, nil, sess, tools.New(), permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose)
	got := orch.estimatedSessionTokens(context.Background(), 0)
	require.Equal(t, sessionTokenEstimate(sess.Messages), got)
}

func TestMaybeCompactPhase1OnlyAvoidsPhase2(t *testing.T) {
	t.Parallel()
	// Huge tool payloads outside the preserved tail should be cleared first; if that
	// drops the estimated size below the threshold, phase-2 tail collapse must not run.
	cfg := config.Default()
	cfg.Provider = "ollama"
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
	orch.maybeCompact(context.Background(), nil)

	require.Len(t, sess.Messages, 26, "phase 2 should not run")
	require.Equal(t, compactedToolResult, sess.Messages[0].ToolResults[0].Content)
	for _, m := range sess.Messages {
		if strings.Contains(m.Content, "[compaction]") {
			require.Fail(t, "unexpected phase-2 compaction summary in session")
		}
	}
}

func TestMaybeCompactMicroBandOnly(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Provider = "ollama"
	cfg.ModelContextTokens = 5000
	cfg.AutoCompactThreshold = 0.85
	config.NormalizeCompactionThresholds(&cfg)

	sess := session.New()
	for i := 0; i < 4; i++ {
		m := llm.PlainMessage("user", fmt.Sprintf("u%d", i))
		m.ToolResults = []llm.ToolResultRecord{{ToolUseID: "t", ToolName: "read_file", Content: strings.Repeat("z", 3500)}}
		sess.Messages = append(sess.Messages, m)
	}
	for i := 4; i < 28; i++ {
		sess.Add("user", strings.Repeat(".", 25))
	}

	orch := New(cfg, nil, sess, tools.New(), permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose)
	tokBefore := sessionTokenEstimate(orch.session.Messages)
	limits, ok := cfg.CompactionAutoLimits()
	require.True(t, ok)
	require.GreaterOrEqual(t, tokBefore, limits.LimitMicro)
	require.Less(t, tokBefore, limits.LimitFull)

	orch.maybeCompact(context.Background(), nil)
	require.Len(t, sess.Messages, 28)
	for _, m := range sess.Messages {
		require.NotContains(t, m.Content, "[session compacted]")
	}
	require.Equal(t, compactedToolResult, sess.Messages[0].ToolResults[0].Content)
}

func TestMaybeCompactNoOpWhenAutoDisabled(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Provider = "ollama"
	cfg.AutoCompactThreshold = 0.85
	sess := session.New()
	sess.Add("user", strings.Repeat("x", 50_000))
	orch := New(cfg, nil, sess, tools.New(), permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose)
	orch.autoCompactDisabled = true
	before := len(sess.Messages)
	orch.maybeCompact(context.Background(), nil)
	require.Equal(t, before, len(sess.Messages))
}

func TestMaybeCompactCircuitBreakerAfterIneffectiveRuns(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Provider = "ollama"
	cfg.LLMCompaction = false
	cfg.ModelContextTokens = 2000
	cfg.AutoCompactThreshold = 0.85
	config.NormalizeCompactionThresholds(&cfg)

	sess := session.New()
	for i := 0; i < 4; i++ {
		sess.Add("user", strings.Repeat("a", 300))
	}
	for i := 0; i < 24; i++ {
		sess.Add("user", strings.Repeat("b", 300))
	}

	orch := New(cfg, nil, sess, tools.New(), permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose)
	require.False(t, orch.autoCompactDisabled)

	for range compactionIneffectiveMaxStreak {
		orch.maybeCompact(context.Background(), nil)
	}
	require.True(t, orch.autoCompactDisabled, "expected auto-compact disabled after ineffective phase-2 runs")

	n := len(sess.Messages)
	orch.maybeCompact(context.Background(), nil)
	require.Equal(t, n, len(sess.Messages), "maybeCompact should no-op when auto-compact is disabled")
}
