package orchestrator

import (
	"context"
	"errors"
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
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/okuzpe/goclaw/testutil/mockopenai"
	"github.com/stretchr/testify/require"
)

type staticTool struct {
	name    string
	content string
}

func (s staticTool) Name() string        { return s.name }
func (s staticTool) Description() string { return s.name }
func (s staticTool) InputSchema() any    { return map[string]any{"type": "object"} }
func (s staticTool) Execute(context.Context, string) (tools.Result, error) {
	return tools.Result{Content: s.content}, nil
}

func newActionStallOrch(t *testing.T, client llm.Client, reg *tools.Registry, cfg config.Config) *Orchestrator {
	t.Helper()
	pol := permissions.NewPolicy()
	for _, spec := range reg.AllSpecs() {
		pol.Set(spec.Name, permissions.ModeAllow)
	}
	return New(
		cfg,
		client,
		session.New(),
		reg,
		pol,
		hooks.New(),
		agents.GeneralPurpose,
	)
}

func TestOrchestratorActionStalledFirstCompletionNoTools(t *testing.T) {
	cfg := testOrchestratorConfig()
	cfg.ActionRepairEscalation = true

	srv := mockopenai.New([]mockopenai.Scenario{
		{Match: "please fix the bug", Response: "I don't have access to your terminal session."},
		{Match: "[goclaw] The user asked for code or repository changes.", Response: "I don't have access to your terminal session."},
		{Match: "[goclaw] Action nudges were exhausted without native tool calls.", Response: "I don't have access to your terminal session."},
	})
	defer srv.Close()

	reg := tools.New()
	reg.Register(staticTool{name: "write_file", content: "wrote"})

	orch := newActionStallOrch(t, testOpenAIClient(srv), reg, cfg)
	_, err := orch.Run(context.Background(), "please fix the bug")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrActionStalled)
	require.Contains(t, err.Error(), "tool access is unavailable")
}

func TestOrchestratorActionStalledAfterRepeatedReadOnlyRounds(t *testing.T) {
	cfg := testOrchestratorConfig()
	cfg.ActionRepairEscalation = true

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("two"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("three"), 0o600))

	srv := mockopenai.New([]mockopenai.Scenario{
		{Match: "improve the repo", Tool: &mockopenai.ToolReply{Name: "read_file", Input: `{"path":"a.txt"}`}},
		{Match: "one", Tool: &mockopenai.ToolReply{Name: "read_file", Input: `{"path":"b.txt"}`}},
		{Match: "two", Tool: &mockopenai.ToolReply{Name: "read_file", Input: `{"path":"c.txt"}`}},
		{Match: "three", Response: "I will continue reading key files before editing."},
		{Match: "[goclaw] Reflection checkpoint:", Response: "I will continue reading key files before editing."},
		{Match: "[goclaw] The user asked for concrete code improvements", Response: "I will continue reading key files before editing."},
		{Match: "[goclaw] Action nudges were exhausted without native tool calls.", Response: "I will continue reading key files before editing."},
	})
	defer srv.Close()

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))
	reg.Register(staticTool{name: "edit_file", content: "edited"})

	orch := newActionStallOrch(t, testOpenAIClient(srv), reg, cfg)
	_, err := orch.Run(context.Background(), "improve the repo")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrActionStalled)
	require.Contains(t, err.Error(), "future-tense narration")

	var sawReflection bool
	for _, msg := range orch.session.Messages {
		if msg.Role == "user" && strings.Contains(msg.Content, "[goclaw] Reflection checkpoint:") {
			sawReflection = true
			break
		}
	}
	require.True(t, sawReflection, "expected reflection nudge before hard fail")
}

func TestOrchestratorActionStalledFenceOnlyResponse(t *testing.T) {
	cfg := testOrchestratorConfig()
	cfg.ActionRepairEscalation = true

	srv := mockopenai.New([]mockopenai.Scenario{
		{Match: "please refactor this", Response: "```json\n```"},
		{Match: "[goclaw] The user asked for code or repository changes.", Response: "```json\n```"},
		{Match: "[goclaw] Action nudges were exhausted without native tool calls.", Response: "```json\n```"},
	})
	defer srv.Close()

	reg := tools.New()
	reg.Register(staticTool{name: "write_file", content: "wrote"})

	orch := newActionStallOrch(t, testOpenAIClient(srv), reg, cfg)
	_, err := orch.Run(context.Background(), "please refactor this")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrActionStalled)
	require.Contains(t, err.Error(), "fence-only")
}

func TestOrchestratorWriteThenVerifyStillSucceeds(t *testing.T) {
	cfg := testOrchestratorConfig()
	cfg.ActionRepairEscalation = true

	srv := mockopenai.New([]mockopenai.Scenario{
		{Match: "apply the fix", Tool: &mockopenai.ToolReply{Name: "write_file", Input: `{}`}},
		{Match: "wrote file", Tool: &mockopenai.ToolReply{Name: "run_tests", Input: `{}`}},
		{Match: "", Response: "done"},
	})
	defer srv.Close()

	reg := tools.New()
	reg.Register(staticTool{name: "write_file", content: "wrote file"})
	reg.Register(staticTool{name: "run_tests", content: "tests ok"})

	orch := newActionStallOrch(t, testOpenAIClient(srv), reg, cfg)
	out, err := orch.Run(context.Background(), "apply the fix")
	require.NoError(t, err)
	require.Equal(t, "done", out)
}

func TestNonActionCompletionReason(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     string
	}{
		{name: "empty", response: "   ", want: "empty"},
		{name: "fence only", response: "```json\n```", want: "fence-only"},
		{name: "fake tool", response: "[assistant tool_use read_file]\n{\"path\":\"README.md\"}", want: "fake tool narration"},
		{name: "meta", response: "As an AI language model, I don't have access to your terminal.", want: "tool access"},
		{name: "future narration", response: "Continuaré leyendo archivos clave para obtener más contexto antes de editar.", want: "future-tense narration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nonActionCompletionReason(tt.response)
			require.True(t, ok)
			require.Contains(t, got, tt.want)
		})
	}
}

func TestActionStalledErrorWrapsSentinel(t *testing.T) {
	err := newActionStalledError("no native tool calls were made after recovery", "I don't have access to your terminal.")
	require.True(t, errors.Is(err, ErrActionStalled))
	require.Contains(t, err.Error(), "action stalled")
}
