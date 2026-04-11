package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/okuzpe/goclaw/testutil/mockserver"
	"github.com/stretchr/testify/require"
)

func testToolPolicy() *permissions.Policy {
	p := permissions.NewPolicy()
	for _, n := range []string{"read_file", "glob", "grep", "bash", "web_fetch", "web_search"} {
		p.Set(n, permissions.ModeAllow)
	}
	return p
}

// newOrch builds a test orchestrator wired to the given LLM client.
func newOrch(t *testing.T, client llm.Client) *Orchestrator {
	t.Helper()
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"

	return New(
		cfg,
		client,
		session.New(),
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)
}

func newOrchWithRegistry(t *testing.T, client llm.Client, reg *tools.Registry) *Orchestrator {
	t.Helper()
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"
	return New(
		cfg,
		client,
		session.New(),
		reg,
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)
}

// --- Scenario 1: text-only response ---

func TestOrchestratorTextOnly(t *testing.T) {
	srv := mockserver.New([]mockserver.Scenario{
		{Match: "ping", Response: "pong"},
	})
	defer srv.Close()

	orch := newOrch(t, llm.NewAnthropic("test-key", srv.URL))

	resp, err := orch.Run(context.Background(), "ping")
	require.NoError(t, err)
	require.Equal(t, "pong", resp)
}

// --- Scenario 2: multi-turn conversation (streaming accumulates correctly) ---

func TestOrchestratorMultiTurn(t *testing.T) {
	srv := mockserver.New([]mockserver.Scenario{
		{Match: "first", Response: "this is the first reply"},
		{Match: "second", Response: "this is the second reply"},
		{Match: "", Response: "fallback"},
	})
	defer srv.Close()

	client := llm.NewAnthropic("test-key", srv.URL)
	orch := newOrch(t, client)

	r1, err := orch.Run(context.Background(), "first question")
	require.NoError(t, err)
	require.Equal(t, "this is the first reply", r1)

	r2, err := orch.Run(context.Background(), "second question")
	require.NoError(t, err)
	require.Equal(t, "this is the second reply", r2)
}

// --- Scenario 3: server returns 500 → error propagated, no panic ---

func TestOrchestratorServerError(t *testing.T) {
	srv := mockserver.New([]mockserver.Scenario{
		{Match: "", StatusCode: http.StatusInternalServerError},
	})
	defer srv.Close()

	orch := newOrch(t, llm.NewAnthropic("test-key", srv.URL))

	_, err := orch.Run(context.Background(), "anything")
	require.Error(t, err)
}

// --- Scenario 4: model requests read_file → tool runs → final assistant text ---

func TestOrchestratorReadFileRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "note.txt"), []byte("tool-content"), 0o600))

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))

	srv := mockserver.New([]mockserver.Scenario{
		{
			Match: "phase-a",
			Tool:  &mockserver.ToolReply{Name: "read_file", Input: `{"path":"note.txt"}`},
		},
		{Match: "tool-content", Response: "ack"},
	})
	defer srv.Close()

	orch := newOrchWithRegistry(t, llm.NewAnthropic("test-key", srv.URL), reg)

	out, err := orch.Run(ctx, "phase-a please read")
	require.NoError(t, err)
	require.Equal(t, "ack", out)
}

func TestOrchestratorAskRequiresApprover(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o600))

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))

	srv := mockserver.New([]mockserver.Scenario{
		{
			Match: "phase-a",
			Tool:  &mockserver.ToolReply{Name: "read_file", Input: `{"path":"note.txt"}`},
		},
	})
	defer srv.Close()

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"

	orch := New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		session.New(),
		reg,
		permissions.NewPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)

	_, err := orch.Run(ctx, "phase-a trigger")
	require.Error(t, err)
	require.Contains(t, err.Error(), "approver")
}

func TestOrchestratorUserDeclinesTool(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o600))

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))

	srv := mockserver.New([]mockserver.Scenario{
		{
			Match: "phase-a",
			Tool:  &mockserver.ToolReply{Name: "read_file", Input: `{"path":"note.txt"}`},
		},
		{Match: "declined", Response: "understood, I will skip that read"},
	})
	defer srv.Close()

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"

	decline := func(context.Context, string, string) (bool, error) { return false, nil }
	orch := New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		session.New(),
		reg,
		permissions.NewPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
		WithToolApprover(decline),
	)

	out, err := orch.Run(ctx, "phase-a trigger")
	require.NoError(t, err)
	require.Equal(t, "understood, I will skip that read", out)
}

func TestOrchestratorMultiToolRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("beta"), 0o600))

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))

	srv := mockserver.New([]mockserver.Scenario{
		{
			Match: "multi-read",
			Tools: []*mockserver.ToolReply{
				{ID: "call_a", Name: "read_file", Input: `{"path":"a.txt"}`},
				{ID: "call_b", Name: "read_file", Input: `{"path":"b.txt"}`},
			},
		},
		{Match: "alpha", Response: "done"},
	})
	defer srv.Close()

	orch := newOrchWithRegistry(t, llm.NewAnthropic("test-key", srv.URL), reg)

	out, err := orch.Run(ctx, "multi-read both files")
	require.NoError(t, err)
	require.Equal(t, "done", out)
}

func TestOrchestratorForceCompact(t *testing.T) {
	srv := mockserver.New([]mockserver.Scenario{
		{Match: "", Response: "noop"},
	})
	defer srv.Close()

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"

	sess := session.New()
	for range 30 {
		sess.Add("user", strings.Repeat("y", 200))
	}

	orch := New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		sess,
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)
	require.Equal(t, 30, sess.Len(), "precondition: want 30 messages")
	orch.ForceCompact()
	require.Less(t, sess.Len(), 30)
	var found bool
	for _, m := range sess.Messages {
		if strings.Contains(m.Content, "[session compacted]") {
			found = true
			break
		}
	}
	if !found {
		require.Fail(t, "expected compaction summary message")
	}
}

func TestOrchestratorCompactionTrimsHead(t *testing.T) {
	srv := mockserver.New([]mockserver.Scenario{
		{Match: "", Response: "pong"},
	})
	defer srv.Close()

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"
	cfg.AutoCompactThreshold = 0.5
	// Pin a small budget so 30 × 5 000-char messages (~50 k tokens) exceed the 0.5 limit.
	cfg.ModelContextTokens = 60_000

	sess := session.New()
	for i := range 30 {
		sess.Add("user", fmt.Sprintf("msg-%d-", i)+strings.Repeat("x", 5000))
	}

	orch := New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		sess,
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)

	_, err := orch.Run(context.Background(), "ping")
	require.NoError(t, err)
	var found bool
	for _, m := range sess.Messages {
		if strings.Contains(m.Content, "[session compacted]") {
			found = true
			break
		}
	}
	if !found {
		require.Fail(t, "expected a compaction summary message in session history")
	}
}

func TestCompactToTailNoOpShortHistory(t *testing.T) {
	cfg := config.Default()
	sess := session.New()
	sess.Add("user", "only one")
	orch := New(
		cfg,
		nil,
		sess,
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)
	orch.ForceCompact()
	require.Equal(t, 1, sess.Len())
}

func TestForceCompactSummaryIncludesCounts(t *testing.T) {
	cfg := config.Default()
	sess := session.New()
	for i := range 30 {
		sess.Add("user", fmt.Sprintf("line-%d", i))
	}
	orch := New(
		cfg,
		nil,
		sess,
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)
	orch.ForceCompact()
	require.Equal(t, 25, sess.Len()) // 1 summary + 24 tail
	first := sess.Messages[0].Content
	require.Contains(t, first, "Summarized 6 earlier message")
	require.Contains(t, first, "tail of 24 kept")
}

func TestReplaceSessionClearsTodoStore(t *testing.T) {
	store := todos.NewStore()
	require.NoError(t, store.Apply(`{"merge":false,"todos":[{"id":"a","content":"task","status":"pending"}]}`))
	require.NotEmpty(t, store.FormatForPrompt(), "precondition: todo store should be non-empty")
	srv := mockserver.New([]mockserver.Scenario{{Match: "", Response: "ok"}})
	defer srv.Close()
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"
	orch := New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		session.New(),
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
		WithTodoStore(store),
	)
	orch.ReplaceSession(session.New())
	require.Empty(t, store.FormatForPrompt())
}

// mcpRoundTripTool stands in for an MCP-backed tool (name uses mcp__ prefix).
type mcpRoundTripTool struct{}

func (mcpRoundTripTool) Name() string { return "mcp__rt__echo" }

func (mcpRoundTripTool) Description() string { return "round-trip test" }

func (mcpRoundTripTool) InputSchema() any { return map[string]any{"type": "object"} }

func (mcpRoundTripTool) Execute(_ context.Context, _ string) (tools.Result, error) {
	return tools.Result{Content: "mcp-rt-session-payload"}, nil
}

func TestOrchestratorMCPRoundTripRecordsToolResultInSession(t *testing.T) {
	ctx := context.Background()
	sess := session.New()
	reg := tools.New()
	reg.Register(mcpRoundTripTool{})

	pol := permissions.NewPolicy()
	pol.Set("mcp__rt__echo", permissions.ModeAllow)

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"

	srv := mockserver.New([]mockserver.Scenario{
		{
			Match: "invoke mcp",
			Tool:  &mockserver.ToolReply{Name: "mcp__rt__echo", Input: `{}`},
		},
		{Match: "mcp-rt-session-payload", Response: "saw mcp output"},
	})
	defer srv.Close()

	orch := New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		sess,
		reg,
		pol,
		hooks.New(),
		agents.GeneralPurpose,
	)

	out, err := orch.Run(ctx, "invoke mcp")
	require.NoError(t, err)
	require.Equal(t, "saw mcp output", out)

	var found bool
	for _, m := range sess.Messages {
		if m.Role != "user" || len(m.ToolResults) == 0 {
			continue
		}
		for _, tr := range m.ToolResults {
			if tr.ToolName == "mcp__rt__echo" && strings.Contains(tr.Content, "mcp-rt-session-payload") {
				found = true
			}
		}
	}
	require.True(t, found, "expected session history to include MCP tool_result content")
}
