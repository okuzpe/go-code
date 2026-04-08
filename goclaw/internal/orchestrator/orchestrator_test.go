package orchestrator_test

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
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
	"github.com/okuzpe/goclaw/testutil/mockserver"
)

func testToolPolicy() *permissions.Policy {
	p := permissions.NewPolicy()
	for _, n := range []string{"read_file", "glob", "grep", "bash", "web_fetch", "web_search"} {
		p.Set(n, permissions.ModeAllow)
	}
	return p
}

// newOrch builds a test orchestrator wired to the given LLM client.
func newOrch(t *testing.T, client llm.Client) *orchestrator.Orchestrator {
	t.Helper()
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"

	return orchestrator.New(
		cfg,
		client,
		session.New(),
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)
}

func newOrchWithRegistry(t *testing.T, client llm.Client, reg *tools.Registry) *orchestrator.Orchestrator {
	t.Helper()
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"
	return orchestrator.New(
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
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp != "pong" {
		t.Errorf("got %q, want %q", resp, "pong")
	}
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
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if r1 != "this is the first reply" {
		t.Errorf("turn 1: got %q", r1)
	}

	r2, err := orch.Run(context.Background(), "second question")
	if err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	if r2 != "this is the second reply" {
		t.Errorf("turn 2: got %q", r2)
	}
}

// --- Scenario 3: server returns 500 → error propagated, no panic ---

func TestOrchestratorServerError(t *testing.T) {
	srv := mockserver.New([]mockserver.Scenario{
		{Match: "", StatusCode: http.StatusInternalServerError},
	})
	defer srv.Close()

	orch := newOrch(t, llm.NewAnthropic("test-key", srv.URL))

	_, err := orch.Run(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error from 500 response, got nil")
	}
	t.Logf("got expected error: %v", err)
}

// --- Scenario 4: model requests read_file → tool runs → final assistant text ---

func TestOrchestratorReadFileRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("tool-content"), 0o600); err != nil {
		t.Fatal(err)
	}

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
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "ack" {
		t.Fatalf("got %q, want ack", out)
	}
}

func TestOrchestratorAskRequiresApprover(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

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

	orch := orchestrator.New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		session.New(),
		reg,
		permissions.NewPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)

	_, err := orch.Run(ctx, "phase-a trigger")
	if err == nil {
		t.Fatal("expected error when DecisionAsk has no approver")
	}
	if !strings.Contains(err.Error(), "approver") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrchestratorUserDeclinesTool(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

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
	orch := orchestrator.New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		session.New(),
		reg,
		permissions.NewPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
		orchestrator.WithToolApprover(decline),
	)

	out, err := orch.Run(ctx, "phase-a trigger")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "understood, I will skip that read" {
		t.Fatalf("got %q, want assistant follow-up after declined tool", out)
	}
}

func TestOrchestratorMultiToolRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("beta"), 0o600); err != nil {
		t.Fatal(err)
	}

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
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "done" {
		t.Fatalf("got %q, want done", out)
	}
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

	orch := orchestrator.New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		sess,
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)
	if sess.Len() != 30 {
		t.Fatalf("precondition: want 30 messages, got %d", sess.Len())
	}
	orch.ForceCompact()
	if sess.Len() >= 30 {
		t.Fatalf("expected message count to drop after ForceCompact, got %d", sess.Len())
	}
	var found bool
	for _, m := range sess.Messages {
		if strings.Contains(m.Content, "[compaction]") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected compaction summary message")
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

	orch := orchestrator.New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		sess,
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)

	_, err := orch.Run(context.Background(), "ping")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var found bool
	for _, m := range sess.Messages {
		if strings.Contains(m.Content, "[compaction]") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected a compaction summary message in session history")
	}
}

func TestCompactToTailNoOpShortHistory(t *testing.T) {
	cfg := config.Default()
	sess := session.New()
	sess.Add("user", "only one")
	orch := orchestrator.New(
		cfg,
		nil,
		sess,
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)
	orch.ForceCompact()
	if sess.Len() != 1 {
		t.Fatalf("expected no compaction for 1 message, len=%d", sess.Len())
	}
}

func TestForceCompactSummaryIncludesCounts(t *testing.T) {
	cfg := config.Default()
	sess := session.New()
	for i := range 30 {
		sess.Add("user", fmt.Sprintf("line-%d", i))
	}
	orch := orchestrator.New(
		cfg,
		nil,
		sess,
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
	)
	orch.ForceCompact()
	if sess.Len() != 25 { // 1 summary + 24 tail
		t.Fatalf("unexpected len %d", sess.Len())
	}
	first := sess.Messages[0].Content
	if !strings.Contains(first, "Summarized 6 earlier message") {
		t.Fatalf("expected removed count in summary: %q", first)
	}
	if !strings.Contains(first, "tail of 24 kept") {
		t.Fatalf("expected tail count: %q", first)
	}
}

func TestReplaceSessionClearsTodoStore(t *testing.T) {
	store := todos.NewStore()
	if err := store.Apply(`{"merge":false,"todos":[{"id":"a","content":"task","status":"pending"}]}`); err != nil {
		t.Fatal(err)
	}
	if store.FormatForPrompt() == "" {
		t.Fatal("precondition: todo store should be non-empty")
	}
	srv := mockserver.New([]mockserver.Scenario{{Match: "", Response: "ok"}})
	defer srv.Close()
	cfg := config.Default()
	cfg.Provider = "anthropic"
	cfg.APIKey = "test-key"
	orch := orchestrator.New(
		cfg,
		llm.NewAnthropic("test-key", srv.URL),
		session.New(),
		tools.New(),
		testToolPolicy(),
		hooks.New(),
		agents.GeneralPurpose,
		orchestrator.WithTodoStore(store),
	)
	orch.ReplaceSession(session.New())
	if store.FormatForPrompt() != "" {
		t.Fatalf("ReplaceSession should clear todo store, still got: %q", store.FormatForPrompt())
	}
}
