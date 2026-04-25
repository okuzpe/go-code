package orchestrator

import (
	"context"
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
)

type fakeTool struct {
	name string
}

func (f fakeTool) Name() string        { return f.name }
func (f fakeTool) Description() string { return "fake" }
func (f fakeTool) InputSchema() any    { return map[string]any{"type": "object"} }
func (f fakeTool) Execute(context.Context, string) (tools.Result, error) {
	return tools.Result{}, nil
}

func TestBuildRequestExploreProfileOmitsBash(t *testing.T) {
	reg := tools.New()
	reg.Register(tools.NewBash())
	reg.Register(tools.NewRunCommandWithTimeout(0))
	reg.Register(tools.NewReadFile(t.TempDir()))

	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.Explore,
	}

	req := o.buildRequest()
	for _, s := range req.Tools {
		if s.Name == "bash" || s.Name == "run_command" {
			t.Fatalf("explore profile is read-only: %s must not appear in tool specs", s.Name)
		}
	}
}

func TestBuildRequestAllowlistWildcardMCP(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	reg.RegisterHidden(fakeTool{name: "mcp__demo__echo"})
	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.Profile{
			Name:          "t",
			ToolAllowlist: []string{"read_file", "mcp__demo__*"},
		},
	}
	req := o.buildRequest()
	var saw bool
	for _, s := range req.Tools {
		if s.Name == "mcp__demo__echo" {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("tools: %#v", req.Tools)
	}
}

func TestBuildRequestHintsAboutHiddenMCPTools(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	reg.Register(fakeTool{name: "tool_search"})
	reg.RegisterHidden(fakeTool{name: "mcp__demo__echo"})
	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.Builder,
	}
	req := o.buildRequest()
	if toolSpecNamesContain(convertLLMTools(req.Tools), "mcp__demo__echo") {
		t.Fatalf("hidden MCP tool should not appear in default prompt: %#v", req.Tools)
	}
	if !strings.Contains(req.System, "Hidden MCP tools") {
		t.Fatalf("expected hidden MCP hint in system prompt: %q", req.System)
	}
}

func TestBuildRequestRevealedHiddenMCPToolAppearsNextIteration(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "tool_search"})
	reg.RegisterHidden(fakeTool{name: "mcp__demo__echo"})
	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.Builder,
		ut: &userTurnState{
			revealedToolNames: map[string]bool{"mcp__demo__echo": true},
		},
	}
	req := o.buildRequest()
	if !toolSpecNamesContain(convertLLMTools(req.Tools), "mcp__demo__echo") {
		t.Fatalf("revealed hidden MCP tool should appear in prompt: %#v", req.Tools)
	}
}

func TestBuildRequestInjectsTodoBlock(t *testing.T) {
	store := todos.NewStore()
	if err := store.Apply(`{"merge":false,"todos":[{"id":"t1","content":"do thing","status":"pending"}]}`); err != nil {
		t.Fatal(err)
	}

	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	o := &Orchestrator{
		cfg:       config.Default(),
		session:   session.New(),
		tools:     reg,
		perms:     permissions.NewPolicy(),
		hooks:     hooks.New(),
		profile:   agents.Explore,
		todoStore: store,
	}
	req := o.buildRequest()
	if !strings.Contains(req.System, "## Session task list (todo_write)") {
		t.Fatalf("system prompt missing todo header: %q", req.System)
	}
	if !strings.Contains(req.System, "do thing") {
		t.Fatalf("system prompt missing todo content: %q", req.System)
	}
}

func TestBuildRequestInjectsSkillsBlock(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	o := &Orchestrator{
		cfg:          config.Default(),
		session:      session.New(),
		tools:        reg,
		perms:        permissions.NewPolicy(),
		hooks:        hooks.New(),
		profile:      agents.Explore,
		skillsPrompt: "Use the frobnicate pattern.",
	}
	req := o.buildRequest()
	if !strings.Contains(req.System, "## Loaded skills (SKILL.md)") {
		t.Fatalf("missing skills header: %q", req.System)
	}
	if !strings.Contains(req.System, "frobnicate") {
		t.Fatalf("missing skills body: %q", req.System)
	}
}

func TestBuildRequestDoesNotEncourageThinkingBlocks(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.GeneralPurpose,
	}
	req := o.buildRequest()
	if strings.Contains(req.System, "Use <thinking>...</thinking>") {
		t.Fatalf("system prompt should not encourage thinking blocks: %q", req.System)
	}
	if !strings.Contains(req.System, "do not emit <thinking> blocks") {
		t.Fatalf("system prompt should explicitly ban thinking blocks before first tool call: %q", req.System)
	}
}

func TestBuildRequestQwenSuffixDoesNotAskForPreToolNarration(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	cfg := config.Default()
	cfg.Provider = "ollama"
	cfg.OllamaModel = "qwen2.5-coder:14b"
	o := &Orchestrator{
		cfg:     cfg,
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.GeneralPurpose,
	}
	req := o.buildRequest()
	if !strings.Contains(req.System, "group related tool calls logically") {
		t.Fatalf("expected qwen-specific suffix in system prompt: %q", req.System)
	}
	if strings.Contains(req.System, "state what you will do") {
		t.Fatalf("qwen suffix should not ask for pre-tool narration: %q", req.System)
	}
}

func TestBuildRequestGeneralPurposePromptStaysLean(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.GeneralPurpose,
	}
	req := o.buildRequest()
	if len(req.System) > 7000 {
		t.Fatalf("general-purpose system prompt regressed in size: got %d bytes", len(req.System))
	}
}

func TestBuildRequestBuildLiteOmitsHiddenMCPHintAndSkills(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	reg.Register(fakeTool{name: "tool_search"})
	reg.RegisterHidden(fakeTool{name: "mcp__demo__echo"})
	o := &Orchestrator{
		cfg:          config.Default(),
		session:      session.New(),
		tools:        reg,
		perms:        permissions.NewPolicy(),
		hooks:        hooks.New(),
		profile:      agents.GeneralPurpose,
		skillsPrompt: "Use the frobnicate pattern.",
	}
	req := o.buildRequest()
	if strings.Contains(req.System, "Hidden MCP tools") {
		t.Fatalf("build-lite should omit hidden MCP hint: %q", req.System)
	}
	if strings.Contains(req.System, "## Loaded skills (SKILL.md)") {
		t.Fatalf("build-lite should omit skills block: %q", req.System)
	}
}

func TestBuildRequestBuildLitePhaseAllowlist(t *testing.T) {
	reg := tools.New()
	for _, name := range []string{"read_file", "glob", "grep", "edit_file", "write_file", "patch", "run_tests", "run_command", "tool_search"} {
		reg.Register(fakeTool{name: name})
	}
	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.GeneralPurpose,
	}

	req := o.buildRequest()
	names := map[string]bool{}
	for _, tool := range req.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"read_file", "glob", "grep", "edit_file", "write_file", "patch"} {
		if !names[want] {
			t.Fatalf("inspect/edit phase missing %q: %#v", want, req.Tools)
		}
	}
	for _, banned := range []string{"run_tests", "run_command", "tool_search"} {
		if names[banned] {
			t.Fatalf("inspect/edit phase should omit %q: %#v", banned, req.Tools)
		}
	}

	o.ut = &userTurnState{verifyPending: true}
	req = o.buildRequest()
	names = map[string]bool{}
	for _, tool := range req.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"run_tests", "run_command", "read_file", "edit_file", "write_file", "patch"} {
		if !names[want] {
			t.Fatalf("verify/repair phase missing %q: %#v", want, req.Tools)
		}
	}
	for _, banned := range []string{"glob", "grep", "tool_search"} {
		if names[banned] {
			t.Fatalf("verify/repair phase should omit %q: %#v", banned, req.Tools)
		}
	}
}

func TestBuildRequestInjectsVerifyChangedFilesBlock(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.GeneralPurpose,
		workdir: "C:/repo",
		ut: &userTurnState{
			verifyPending: true,
			changedPaths: map[string]bool{
				"internal/a.go": true,
				"README.md":     true,
			},
		},
	}
	req := o.buildRequest()
	if !strings.Contains(req.System, "## Verify changed files") {
		t.Fatalf("missing verify changed files header: %q", req.System)
	}
	if !strings.Contains(req.System, "internal/a.go") || !strings.Contains(req.System, "README.md") {
		t.Fatalf("missing changed paths in system prompt: %q", req.System)
	}
	if strings.Contains(req.System, "git status --short") {
		t.Fatalf("build-lite verify guidance should avoid git-first shell hints: %q", req.System)
	}
}

func TestBuildRequestReadOnlyStripsMCP(t *testing.T) {
	reg := tools.New()
	reg.RegisterHidden(fakeTool{name: "mcp__srv__t"})
	reg.Register(fakeTool{name: "read_file"})
	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.Profile{Name: "ro", ReadOnly: true},
	}
	req := o.buildRequest()
	for _, s := range req.Tools {
		if strings.HasPrefix(s.Name, "mcp__") {
			t.Fatalf("mcp tool leaked into read-only specs: %s", s.Name)
		}
	}
}

func convertLLMTools(specs []llm.ToolSpec) []tools.ToolSpec {
	out := make([]tools.ToolSpec, 0, len(specs))
	for _, s := range specs {
		out = append(out, tools.ToolSpec{Name: s.Name, Description: s.Description, InputSchema: s.InputSchema})
	}
	return out
}
