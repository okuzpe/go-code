package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/okuzpe/goclaw/testutil/mockopenai"
	"github.com/stretchr/testify/require"
)

type appStaticTool struct {
	name    string
	content string
}

func (a appStaticTool) Name() string        { return a.name }
func (a appStaticTool) Description() string { return a.name }
func (a appStaticTool) InputSchema() any    { return map[string]any{"type": "object"} }
func (a appStaticTool) Execute(context.Context, string) (tools.Result, error) {
	if a.content != "" {
		return tools.Result{Content: a.content}, nil
	}
	return tools.Result{Content: "ok"}, nil
}

func TestReadSingleLineStdin(t *testing.T) {
	t.Parallel()
	s, err := readSingleLineStdin(strings.NewReader("hello world\n"))
	require.NoError(t, err)
	require.Equal(t, "hello world", s)
}

func TestReadSingleLineStdinEOFNoNewline(t *testing.T) {
	t.Parallel()
	s, err := readSingleLineStdin(strings.NewReader("only"))
	require.NoError(t, err)
	require.Equal(t, "only", s)
}

func TestAutomationOutputToolApprover(t *testing.T) {
	t.Parallel()
	_, err := automationOutputToolApprover(context.Background(), "bash", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "automation output")
}

func appTestOpenAIClient(srv *mockopenai.Server) llm.Client {
	return llm.NewOpenAICompat("test-key", srv.URL+"/v1")
}

func newActionStallRuntime(client llm.Client) *ChatRuntime {
	cfg := config.Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "mock-model"
	cfg.ActionRepairEscalation = true
	reg := tools.New()
	reg.Register(appStaticTool{name: "write_file"})
	pol := permissions.NewPolicy()
	pol.Set("write_file", permissions.ModeAllow)
	return &ChatRuntime{
		Cfg:     cfg,
		Client:  client,
		Sess:    session.New(),
		Reg:     reg,
		Policy:  pol,
		HookReg: hooks.New(),
		Profile: agents.GeneralPurpose,
	}
}

func newSingleToolRuntime(client llm.Client, toolName, content string) *ChatRuntime {
	cfg := config.Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "mock-model"
	reg := tools.New()
	reg.Register(appStaticTool{name: toolName, content: content})
	pol := permissions.NewPolicy()
	pol.Set(toolName, permissions.ModeAllow)
	return &ChatRuntime{
		Cfg:     cfg,
		Client:  client,
		Sess:    session.New(),
		Reg:     reg,
		Policy:  pol,
		HookReg: hooks.New(),
		Profile: agents.GeneralPurpose,
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	runErr := fn()
	require.NoError(t, w.Close())
	out, readErr := io.ReadAll(r)
	require.NoError(t, readErr)
	require.NoError(t, r.Close())
	return string(out), runErr
}

func TestRunChatJSONOutputFromLineReturnsActionStalledError(t *testing.T) {
	srv := mockopenai.New([]mockopenai.Scenario{
		{Match: "please fix this", Response: "I don't have access to your terminal session."},
		{Match: "[goclaw] The user asked for code or repository changes.", Response: "I don't have access to your terminal session."},
		{Match: "[goclaw] Action nudges were exhausted without native tool calls.", Response: "I don't have access to your terminal session."},
	})
	defer srv.Close()

	rt := newActionStallRuntime(appTestOpenAIClient(srv))
	out, err := captureStdout(t, func() error {
		return RunChatJSONOutputFromLine(context.Background(), rt, "please fix this")
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, orchestrator.ErrActionStalled))
	require.Empty(t, strings.TrimSpace(out), "JSON mode should not emit a fake success payload on stall")
}

func TestRunChatTextOutputFromLineReturnsActionStalledError(t *testing.T) {
	srv := mockopenai.New([]mockopenai.Scenario{
		{Match: "please fix this", Response: "I don't have access to your terminal session."},
		{Match: "[goclaw] The user asked for code or repository changes.", Response: "I don't have access to your terminal session."},
		{Match: "[goclaw] Action nudges were exhausted without native tool calls.", Response: "I don't have access to your terminal session."},
	})
	defer srv.Close()

	rt := newActionStallRuntime(appTestOpenAIClient(srv))
	out, err := captureStdout(t, func() error {
		return RunChatTextOutputFromLine(context.Background(), rt, "please fix this")
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, orchestrator.ErrActionStalled))
	require.Empty(t, strings.TrimSpace(out), "text mode should not emit a fake success reply on stall")
}

func TestRunChatJSONOutputFromLineHandlesLocalPrefixTool(t *testing.T) {
	rt := newSingleToolRuntime(nil, "read_file", "module demo\n")
	out, err := captureStdout(t, func() error {
		return RunChatJSONOutputFromLine(context.Background(), rt, "@go.mod")
	})
	require.NoError(t, err)

	var got orchestrator.JSONTurnResult
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Equal(t, "(read_file)\nmodule demo", got.Response)
	require.Len(t, got.ToolCalls, 1)
	require.Equal(t, localPrefixToolUseID, got.ToolCalls[0].ID)
	require.Equal(t, "read_file", got.ToolCalls[0].Name)
	require.Contains(t, got.ToolCalls[0].Input, `"path":"go.mod"`)
	require.Equal(t, "module demo\n", got.ToolCalls[0].Result)
	require.False(t, got.ToolCalls[0].IsError)
	require.Len(t, rt.Sess.Messages, 2, "local prefix tool should still record user + assistant messages")
}

func TestRunChatTextOutputFromLineExpandsInlineAtRefs(t *testing.T) {
	srv := mockopenai.New([]mockopenai.Scenario{
		{Match: "@go.mod\n---\nmodule demo", Response: "loaded inline context"},
	})
	defer srv.Close()

	rt := newSingleToolRuntime(appTestOpenAIClient(srv), "read_file", "module demo\n")
	out, err := captureStdout(t, func() error {
		return RunChatTextOutputFromLine(context.Background(), rt, "please review @go.mod now")
	})
	require.NoError(t, err)
	require.Equal(t, "loaded inline context\n", out)
	require.Len(t, rt.Sess.Messages, 2)
	require.Contains(t, rt.Sess.Messages[0].Content, "[Files loaded via @ references]")
	require.Contains(t, rt.Sess.Messages[0].Content, "@go.mod\n---\nmodule demo")
}
