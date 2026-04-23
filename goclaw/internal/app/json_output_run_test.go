package app

import (
	"context"
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

type appStaticTool struct{ name string }

func (a appStaticTool) Name() string        { return a.name }
func (a appStaticTool) Description() string { return a.name }
func (a appStaticTool) InputSchema() any    { return map[string]any{"type": "object"} }
func (a appStaticTool) Execute(context.Context, string) (tools.Result, error) {
	return tools.Result{Content: "ok"}, nil
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
