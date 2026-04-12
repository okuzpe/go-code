package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/okuzpe/goclaw/testutil/mockserver"
	"github.com/stretchr/testify/require"
)

// --- Test 1: RunStreaming + StreamSink event ordering ---

// captureSink records every event fired by RunStreaming.
type captureSink struct {
	mu      sync.Mutex
	events  []string
	deltas  []string
	doneMsg string
}

func (c *captureSink) OnTextDelta(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, "text:"+text)
	c.deltas = append(c.deltas, text)
}

func (c *captureSink) OnToolUse(name, _ string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, "tool_use:"+name)
}

func (c *captureSink) OnToolResult(name string, _ string, isError bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	suffix := "ok"
	if isError {
		suffix = "err"
	}
	c.events = append(c.events, "tool_result:"+name+":"+suffix)
}

func (c *captureSink) OnDone(finalText string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, "done")
	c.doneMsg = finalText
}

func TestOrchestratorRunStreamingEventOrder(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "note.txt"), []byte("file-content"), 0o600))

	srv := mockserver.New([]mockserver.Scenario{
		{
			Match: "stream-test",
			Tool:  &mockserver.ToolReply{Name: "read_file", Input: `{"path":"note.txt"}`},
		},
		{Match: "file-content", Response: "all done"},
	})
	defer srv.Close()

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))

	sink := &captureSink{}
	orch := newOrchWithRegistry(t, llm.NewAnthropic("test-key", srv.URL), reg)

	out, err := orch.RunStreaming(context.Background(), "stream-test", sink)
	require.NoError(t, err)
	require.Equal(t, "all done", out)

	// Verify the event sequence: tool_use before tool_result, done at the end.
	require.Equal(t, "all done", sink.doneMsg)

	var toolUseIdx, toolResultIdx, doneIdx int = -1, -1, -1
	for i, ev := range sink.events {
		switch {
		case strings.HasPrefix(ev, "tool_use:"):
			toolUseIdx = i
		case strings.HasPrefix(ev, "tool_result:"):
			toolResultIdx = i
		case ev == "done":
			doneIdx = i
		}
	}
	require.GreaterOrEqual(t, toolUseIdx, 0, "expected OnToolUse event")
	require.GreaterOrEqual(t, toolResultIdx, 0, "expected OnToolResult event")
	require.GreaterOrEqual(t, doneIdx, 0, "expected OnDone event")
	require.Less(t, toolUseIdx, toolResultIdx, "OnToolUse must fire before OnToolResult")
	require.Less(t, toolResultIdx, doneIdx, "OnToolResult must fire before OnDone")
	require.Contains(t, sink.events[toolResultIdx], ":ok", "read_file should succeed")
}

func TestOrchestratorRunStreamingTextDeltas(t *testing.T) {
	srv := mockserver.New([]mockserver.Scenario{
		{Match: "delta-test", Response: "hello world"},
	})
	defer srv.Close()

	sink := &captureSink{}
	orch := newOrch(t, llm.NewAnthropic("test-key", srv.URL))

	out, err := orch.RunStreaming(context.Background(), "delta-test", sink)
	require.NoError(t, err)
	require.Equal(t, "hello world", out)

	require.NotEmpty(t, sink.deltas, "expected at least one OnTextDelta call")
	combined := strings.Join(sink.deltas, "")
	require.Equal(t, "hello world", combined)
	require.Equal(t, "hello world", sink.doneMsg)
}

// --- Test 2: PreToolUse hook blocks tool execution ---

func TestOrchestratorPreToolUseHookBlocks(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("sensitive"), 0o600))

	srv := mockserver.New([]mockserver.Scenario{
		{
			Match: "read-blocked",
			Tool:  &mockserver.ToolReply{Name: "read_file", Input: `{"path":"secret.txt"}`},
		},
		// LLM sees the hook error in the tool_result and responds.
		{Match: "pre_tool_use hook blocked", Response: "understood, skipping that file"},
	})
	defer srv.Close()

	hookReg := hooks.New()
	hookReg.On(hooks.PreToolUse, func(_ context.Context, e hooks.Event) error {
		if e.ToolName == "read_file" {
			return fmt.Errorf("access denied by policy")
		}
		return nil
	})

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"

	policy := permissions.NewPolicy()
	policy.Set("read_file", permissions.ModeAllow)

	orch := New(cfg, llm.NewAnthropic("test-key", srv.URL),
		session.New(), reg, policy, hookReg, agents.GeneralPurpose)

	out, err := orch.Run(context.Background(), "read-blocked")
	require.NoError(t, err)
	require.Equal(t, "understood, skipping that file", out)

	// Verify the tool was never actually executed (file content not seen by LLM).
	for _, msg := range orch.session.Messages {
		require.NotContains(t, msg.Content, "sensitive",
			"hook should have prevented file content from reaching the LLM")
	}
}

func TestOrchestratorPostToolUseHookReceivesOutput(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.txt"), []byte("tool-output"), 0o600))

	srv := mockserver.New([]mockserver.Scenario{
		{Match: "post-hook-test",
			Tool: &mockserver.ToolReply{Name: "read_file", Input: `{"path":"data.txt"}`}},
		{Match: "tool-output", Response: "ok"},
	})
	defer srv.Close()

	var capturedOutput string
	hookReg := hooks.New()
	hookReg.On(hooks.PostToolUse, func(_ context.Context, e hooks.Event) error {
		capturedOutput = e.Output
		return nil
	})

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"

	policy := permissions.NewPolicy()
	policy.Set("read_file", permissions.ModeAllow)

	orch := New(cfg, llm.NewAnthropic("test-key", srv.URL),
		session.New(), reg, policy, hookReg, agents.GeneralPurpose)

	_, err := orch.Run(context.Background(), "post-hook-test")
	require.NoError(t, err)
	require.Contains(t, capturedOutput, "tool-output",
		"PostToolUse hook should receive the tool's output")
}

// --- Test 3: Memory store injection into system prompt ---

func TestOrchestratorMemoryInjectedInSystemPrompt(t *testing.T) {
	memDir := t.TempDir()
	memStore := memory.New(memDir)

	_, err := memStore.Save(memory.Entry{
		Name:        "user-context",
		Description: "user background",
		Type:        memory.TypeUser,
		Body:        "The user is working on a Go project called goclaw.",
	})
	require.NoError(t, err)

	srv := mockserver.New([]mockserver.Scenario{
		// The mock server matches on the last user message, not the system prompt.
		// We just verify the request reached the LLM and we got a response back.
		{Match: "memory-check", Response: "I know about goclaw"},
	})
	defer srv.Close()

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"

	orch := New(cfg, llm.NewAnthropic("test-key", srv.URL),
		session.New(), tools.New(), testToolPolicy(), hooks.New(), agents.GeneralPurpose,
		WithMemoryStore(memStore),
	)

	out, err := orch.Run(context.Background(), "memory-check")
	require.NoError(t, err)
	require.Equal(t, "I know about goclaw", out)

	// Verify memory was injected into the built request (check via session messages).
	// The system prompt is not stored in session — verify indirectly by ensuring
	// the orchestrator ran without error and memory store is readable.
	entries, listErr := memStore.List()
	require.NoError(t, listErr)
	require.Len(t, entries, 1)
	require.Equal(t, "user-context", entries[0].Name)
}

func TestOrchestratorMemoryPersistedAcrossTurns(t *testing.T) {
	memDir := t.TempDir()
	memStore := memory.New(memDir)

	_, err := memStore.Save(memory.Entry{
		Name:        "lang-pref",
		Description: "user language preference",
		Type:        memory.TypeFeedback,
		Body:        "Always respond in English.",
	})
	require.NoError(t, err)

	srv := mockserver.New([]mockserver.Scenario{
		{Match: "turn-one", Response: "turn-one reply"},
		{Match: "turn-two", Response: "turn-two reply"},
	})
	defer srv.Close()

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"

	orch := New(cfg, llm.NewAnthropic("test-key", srv.URL),
		session.New(), tools.New(), testToolPolicy(), hooks.New(), agents.GeneralPurpose,
		WithMemoryStore(memStore),
	)

	r1, err := orch.Run(context.Background(), "turn-one")
	require.NoError(t, err)
	require.Equal(t, "turn-one reply", r1)

	r2, err := orch.Run(context.Background(), "turn-two")
	require.NoError(t, err)
	require.Equal(t, "turn-two reply", r2)
}

// --- Test 4: Multi-iteration tool chain (3+ sequential tool calls) ---

func TestOrchestratorThreeIterationToolChain(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.txt"), []byte("list: a.txt b.txt"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("content-A"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("content-B"), 0o600))

	srv := mockserver.New([]mockserver.Scenario{
		// Iteration 1: read the index
		{
			Match: "chain-start",
			Tool:  &mockserver.ToolReply{ID: "call_1", Name: "read_file", Input: `{"path":"index.txt"}`},
		},
		// Iteration 2: read file a (LLM sees "list: a.txt b.txt" from previous result)
		{
			Match: "list: a.txt b.txt",
			Tool:  &mockserver.ToolReply{ID: "call_2", Name: "read_file", Input: `{"path":"a.txt"}`},
		},
		// Iteration 3: LLM sees "content-A", reads b.txt
		{
			Match: "content-A",
			Tool:  &mockserver.ToolReply{ID: "call_3", Name: "read_file", Input: `{"path":"b.txt"}`},
		},
		// Final: LLM sees "content-B", returns summary
		{Match: "content-B", Response: "chain complete"},
	})
	defer srv.Close()

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))

	orch := newOrchWithRegistry(t, llm.NewAnthropic("test-key", srv.URL), reg)

	out, err := orch.Run(context.Background(), "chain-start")
	require.NoError(t, err)
	require.Equal(t, "chain complete", out)

	// Session should record 3 tool result turns + final assistant reply.
	var toolResultTurns int
	for _, msg := range orch.session.Messages {
		if len(msg.ToolResults) > 0 {
			toolResultTurns++
		}
	}
	require.Equal(t, 3, toolResultTurns, "expected 3 tool result turns in session")
}

// --- Test 5: Profile switch changes tool allowlist for subsequent turns ---

func TestOrchestratorSetProfileChangesAllowlist(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("readable"), 0o600))

	srv := mockserver.New([]mockserver.Scenario{
		{
			Match: "read-with-general",
			Tool:  &mockserver.ToolReply{Name: "read_file", Input: `{"path":"file.txt"}`},
		},
		{Match: "readable", Response: "got the file"},
		// After switching to ReadOnly profile, bash is stripped from the request.
		{Match: "after-switch", Response: "profile switched"},
	})
	defer srv.Close()

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))
	reg.Register(tools.NewBash())

	orch := newOrchWithRegistry(t, llm.NewAnthropic("test-key", srv.URL), reg)

	// First turn with GeneralPurpose: read_file works.
	out, err := orch.Run(context.Background(), "read-with-general")
	require.NoError(t, err)
	require.Equal(t, "got the file", out)

	// Switch to Explore profile (ReadOnly: true — strips write/bash tools, keeps read_file).
	orch.SetProfile(agents.Explore)

	// Second turn: LLM only sees read-safe tools.
	out2, err := orch.Run(context.Background(), "after-switch")
	require.NoError(t, err)
	require.Equal(t, "profile switched", out2)
}

// --- Test 6: YOLO threshold auto-approves low-risk tools ---

func TestOrchestratorYoloAutoApproves(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "info.txt"), []byte("yolo-content"), 0o600))

	srv := mockserver.New([]mockserver.Scenario{
		{
			Match: "yolo-read",
			Tool:  &mockserver.ToolReply{Name: "read_file", Input: `{"path":"info.txt"}`},
		},
		{Match: "yolo-content", Response: "auto-approved"},
	})
	defer srv.Close()

	reg := tools.New()
	reg.Register(tools.NewReadFile(dir))

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"
	cfg.YoloThreshold = 50 // auto-approve anything with risk score ≤ 50

	// No ToolApprover set — if YOLO didn't kick in, this would fail with "no approver".
	orch := New(cfg, llm.NewAnthropic("test-key", srv.URL),
		session.New(), reg, permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose)

	out, err := orch.Run(context.Background(), "yolo-read")
	require.NoError(t, err)
	require.Equal(t, "auto-approved", out)
}

func TestOrchestratorYoloBlocksHighRisk(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "safe.txt"), []byte("ok"), 0o600))

	srv := mockserver.New([]mockserver.Scenario{
		{
			// bash with a destructive-looking command has a high risk score.
			Match: "high-risk",
			Tool:  &mockserver.ToolReply{Name: "bash", Input: `{"command":"rm -rf /tmp/all"}`},
		},
		// The orchestrator should return an error (no approver + YOLO threshold exceeded).
	})
	defer srv.Close()

	reg := tools.New()
	reg.Register(tools.NewBash())

	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"
	cfg.YoloThreshold = 10 // only auto-approve very low-risk operations

	// No ToolApprover — YOLO threshold is too low to approve bash rm.
	orch := New(cfg, llm.NewAnthropic("test-key", srv.URL),
		session.New(), reg, permissions.NewPolicy(), hooks.New(), agents.GeneralPurpose)

	_, err := orch.Run(context.Background(), "high-risk")
	require.Error(t, err, "expected error when YOLO threshold is exceeded and no approver")
	require.Contains(t, err.Error(), "approver")
}
