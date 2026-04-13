// Package orchestrator implements the main agent loop:
// user input → LLM → tool execution → feedback → repeat (max 32 iterations).
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
)

const (
	maxIterations = 32
	maxToolCalls  = 64
)

// ToolApprover is invoked when permissions.Evaluate returns DecisionAsk.
// It should return true to run the tool, false to deny, or an error to abort.
type ToolApprover func(ctx context.Context, toolName, toolInput string) (bool, error)

// Option configures an Orchestrator.
type Option func(*Orchestrator)

// WithToolApprover registers interactive approval for Ask mode.
func WithToolApprover(a ToolApprover) Option {
	return func(o *Orchestrator) {
		o.approver = a
	}
}

// WithMemoryStore attaches a filesystem memory store; recent entries are injected into the system prompt.
func WithMemoryStore(mem *memory.Store) Option {
	return func(o *Orchestrator) {
		o.mem = mem
	}
}

// WithProjectMemoryStore attaches a per-project memory store (.goclaw/memory/) whose recent entries
// are injected separately from the user memory store (D14). Either store may be nil.
func WithProjectMemoryStore(mem *memory.Store) Option {
	return func(o *Orchestrator) {
		o.projectMem = mem
	}
}

// AfterToolHook is invoked after each tool finishes (before results are added to the session).
// toolInput is the raw JSON arguments the model sent (may be empty).
type AfterToolHook func(toolName string, toolInput string, resultBytes int, isError bool)

// WithAfterTool registers a hook for REPL progress / logging (optional).
func WithAfterTool(h AfterToolHook) Option {
	return func(o *Orchestrator) {
		o.afterTool = h
	}
}

// WithSkillsSnippet appends discovered SKILL.md content to the system prompt (bounded by the skills package).
func WithSkillsSnippet(s string) Option {
	return func(o *Orchestrator) {
		o.skillsPrompt = strings.TrimSpace(s)
	}
}

// StreamSink receives streaming and tool lifecycle events during RunStreaming.
// All methods must be fast and non-blocking (or handle their own buffering),
// since they run on the orchestrator's goroutines.
type StreamSink interface {
	OnTextDelta(text string)
	OnToolUse(name, rawInput string)
	// OnToolResult is called after each tool finishes. content is the full result
	// string (may be large; callers may cap display to a reasonable limit).
	OnToolResult(name string, content string, isError bool)
	OnDone(finalText string)
	// OnCompact is called after automatic context compaction removes messages.
	// removed is the net reduction in message count after compaction.
	OnCompact(removed int)
}

// WithTodoStore attaches a session-scoped task list; when non-nil, its snapshot is appended to the system prompt.
func WithTodoStore(s *todos.Store) Option {
	return func(o *Orchestrator) {
		o.todoStore = s
	}
}

// WithWorkdir injects the tool path root into the system prompt (read_file/glob/write scope).
func WithWorkdir(dir string) Option {
	return func(o *Orchestrator) {
		o.workdir = strings.TrimSpace(dir)
	}
}

// WithLaunchDir injects the process working directory at agent start. Relative paths in
// file tools resolve from here unless the model passes an absolute path. Optional; if
// empty, buildRequest only describes the tool path root.
func WithLaunchDir(dir string) Option {
	return func(o *Orchestrator) {
		o.launchDir = strings.TrimSpace(dir)
	}
}

// WithProjectContext injects a brief project summary (e.g. from go.mod / README)
// into the system prompt so the agent understands the project without a warm-up tool call.
func WithProjectContext(ctx string) Option {
	return func(o *Orchestrator) {
		o.projectContext = strings.TrimSpace(ctx)
	}
}

// Orchestrator wires all subsystems and drives the agent loop.
type Orchestrator struct {
	cfg               config.Config
	llm               llm.Client
	session           *session.Session
	tools             *tools.Registry
	perms             *permissions.Policy
	hooks             *hooks.Registry
	profile           agents.Profile
	approver          ToolApprover
	mem               *memory.Store
	projectMem        *memory.Store // per-project memory store (D14); nil if not present
	afterTool         AfterToolHook
	skillsPrompt      string
	todoStore *todos.Store

	// workdir is the tool path root injected into the system prompt (optional).
	workdir string

	// launchDir is the cwd when goclaw started; injected for path-resolution hints (optional).
	launchDir string

	// projectContext is a brief project summary injected into the system prompt (optional).
	projectContext string

	// turnModel is the resolved model id for the current user turn (tool iterations reuse it).
	// Empty means use cfg.Model() in buildRequest. Cleared after runUserTurn completes.
	turnModel string

	// taskRole is the classified task role for the current user turn (e.g. "code", "explore").
	// Used by buildRequest to inject a per-role system hint. Cleared after runUserTurn completes.
	taskRole string
}

// New creates an Orchestrator with the provided subsystems.
func New(
	cfg config.Config,
	client llm.Client,
	sess *session.Session,
	reg *tools.Registry,
	policy *permissions.Policy,
	hookReg *hooks.Registry,
	profile agents.Profile,
	opts ...Option,
) *Orchestrator {
	o := &Orchestrator{
		cfg:     cfg,
		llm:     client,
		session: sess,
		tools:   reg,
		perms:   policy,
		hooks:   hookReg,
		profile: profile,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// TaskRole returns the classified task role for the current turn (e.g. "fix", "code", "explore").
// Returns "" between turns.
func (o *Orchestrator) TaskRole() string { return o.taskRole }

// TurnModel returns the model id resolved for the current turn.
// Returns "" when the default model (cfg.Model()) is used.
func (o *Orchestrator) TurnModel() string { return o.turnModel }

// Run processes a single user message and returns the final assistant response.
func (o *Orchestrator) Run(ctx context.Context, userMessage string) (string, error) {
	return o.RunStreaming(ctx, userMessage, nil)
}

// RunStreaming processes a single user message. If sink is non-nil, it receives
// incremental deltas and tool lifecycle events.
func (o *Orchestrator) RunStreaming(ctx context.Context, userMessage string, sink StreamSink) (string, error) {
	return o.runUserTurn(ctx, userMessage, sink, nil)
}

// RunStreamingToolTrace runs one user turn like RunStreaming and appends each executed tool to trace (for automation JSON output).
func (o *Orchestrator) RunStreamingToolTrace(ctx context.Context, userMessage string, sink StreamSink, trace *[]JSONToolCall) (string, error) {
	return o.runUserTurn(ctx, userMessage, sink, trace)
}

func (o *Orchestrator) runUserTurn(ctx context.Context, userMessage string, sink StreamSink, toolTrace *[]JSONToolCall) (string, error) {
	if o.session == nil {
		return "", fmt.Errorf("orchestrator: session is required")
	}
	defer o.session.ClearStreamingAssistant()
	o.session.Add("user", userMessage)

	o.prepareTurnModel(ctx, userMessage)
	defer func() { o.turnModel = ""; o.taskRole = "" }()

	toolCalls := 0
	iterLimit := maxIterations
	if o.profile.MaxTurns > 0 && o.profile.MaxTurns < maxIterations {
		iterLimit = o.profile.MaxTurns
	}

	for range iterLimit {
		o.maybeCompact(ctx, sink)

		streamStart := time.Now()
		events, errc := o.llm.Stream(ctx, o.buildRequest())

		var response string
		var pendingTools []llm.ToolUse

		for e := range events {
			switch ev := e.(type) {
			case llm.TextDelta:
				response += ev.Text
				if o.session != nil && ev.Text != "" {
					o.session.AddStreamingAssistantChars(len(ev.Text))
				}
				if sink != nil && ev.Text != "" {
					sink.OnTextDelta(ev.Text)
				}
			case llm.ToolUse:
				pendingTools = append(pendingTools, ev)
				slog.Debug("tool use requested",
					"id", ev.ID,
					"name", ev.Name,
					"preview", FormatToolUsePreview(ev.Name, ev.Input),
				)
				if sink != nil {
					sink.OnToolUse(ev.Name, ev.Input)
				}
			case llm.Done:
				// stream finished
			}
		}

		if err := <-errc; err != nil {
			return "", fmt.Errorf("llm stream: %w", err)
		}
		slog.Debug("llm stream done", "elapsed", time.Since(streamStart), "tools", len(pendingTools))

		if len(pendingTools) == 0 {
			o.session.AddAssistant(response, nil)
			if sink != nil {
				sink.OnDone(response)
			}
			if toolCalls == 0 {
				extractModel := o.cfg.Model()
				if mo := strings.TrimSpace(o.profile.ModelOverride); mo != "" {
					extractModel = o.cfg.NormalizeModelForProvider(mo)
				} else if tm := strings.TrimSpace(o.turnModel); tm != "" {
					extractModel = tm
				}
				memory.ScheduleSilentTurnLLMExtract(o.cfg, o.llm, o.mem, extractModel, userMessage, response)
			}
			return response, nil
		}

		records := make([]llm.ToolCallRecord, len(pendingTools))
		for i, tu := range pendingTools {
			records[i] = llm.ToolCallRecord(tu)
		}
		o.session.AddAssistant(response, records)

		if maxToolCalls-toolCalls < len(pendingTools) {
			return "", fmt.Errorf("tool call limit (%d) reached", maxToolCalls)
		}

		parallel := len(pendingTools) > 1 && o.allToolsAutoApprove(pendingTools) && !pendingToolsIncludeSpawnAgent(pendingTools)
		var results []llm.ToolResultRecord

		if parallel {
			var atomicCalls atomic.Int64
			results = o.executeToolsParallel(ctx, pendingTools, &atomicCalls, sink)
			toolCalls += int(atomicCalls.Load())
			for i, r := range results {
				input := ""
				if i < len(pendingTools) {
					input = pendingTools[i].Input
				}
				if o.afterTool != nil {
					o.afterTool(r.ToolName, input, len(r.Content), r.IsError)
				}
				if sink != nil {
					sink.OnToolResult(r.ToolName, r.Content, r.IsError)
				}
			}
			appendJSONToolTrace(toolTrace, pendingTools, results)
		} else {
			for _, tu := range pendingTools {
				toolCalls++
				toolStart := time.Now()
				out := o.executeTool(ctx, &tu, sink)
				slog.Debug("tool done", "name", tu.Name, "elapsed", time.Since(toolStart), "bytes", len(out.Content), "isError", out.IsError)
				if out.Err != nil {
					return "", out.Err
				}
				if o.afterTool != nil {
					o.afterTool(tu.Name, tu.Input, len(out.Content), out.IsError)
				}
				if sink != nil {
					sink.OnToolResult(tu.Name, out.Content, out.IsError)
				}
				results = append(results, llm.ToolResultRecord{
					ToolUseID: tu.ID,
					ToolName:  tu.Name,
					Content:   out.Content,
					IsError:   out.IsError,
				})
			}
			appendJSONToolTrace(toolTrace, pendingTools, results)
		}
		o.session.AddToolResults(results)
	}

	return "", fmt.Errorf("iteration limit (%d) reached", iterLimit)
}

// ReplaceSession switches the in-memory conversation to s (e.g. after /new in the REPL).
func (o *Orchestrator) ReplaceSession(s *session.Session) {
	if s == nil {
		return
	}
	o.session = s
	if o.todoStore != nil {
		o.todoStore.Clear()
	}
}

// SetProfile switches the active agent profile (tool allowlist, read-only flag, system prompt suffix).
func (o *Orchestrator) SetProfile(p agents.Profile) {
	o.profile = p
}

// SetConfig replaces the orchestrator config snapshot (e.g. after /model in the REPL).
func (o *Orchestrator) SetConfig(cfg config.Config) {
	o.cfg = cfg
}

// SetToolPermission overrides the permission mode for a single tool in the active policy.
// Useful for slash commands that toggle auto-approval within a session (e.g. /allow-writes).
func (o *Orchestrator) SetToolPermission(toolName string, mode permissions.Mode) {
	o.perms.Set(toolName, mode)
}

// YoloThreshold returns the active YOLO auto-approval threshold (-1 = disabled).
func (o *Orchestrator) YoloThreshold() int {
	return o.cfg.YoloThreshold
}

// ProfileName returns the active profile's name.
func (o *Orchestrator) ProfileName() string {
	return o.profile.Name
}

// ActiveProfile returns the active agent profile (tool allowlist, read-only flag, prompts).
func (o *Orchestrator) ActiveProfile() agents.Profile {
	return o.profile
}

// allToolsAutoApprove returns true when every tool in the list would be auto-approved
// without interactive input — either via a DecisionAllow policy or a YOLO bypass.
// Only when this holds for all tools can the turn be executed in parallel.
func (o *Orchestrator) allToolsAutoApprove(pendingTools []llm.ToolUse) bool {
	for _, tu := range pendingTools {
		switch o.perms.Evaluate(tu.Name) {
		case permissions.DecisionAllow:
			// explicitly allowed — fine
		case permissions.DecisionAsk:
			if o.cfg.YoloThreshold < 0 {
				return false
			}
			if permissions.RiskScore(tu.Name, tu.Input) > o.cfg.YoloThreshold {
				return false
			}
		default: // DecisionDeny
			return false
		}
	}
	return true
}

func pendingToolsIncludeSpawnAgent(pendingTools []llm.ToolUse) bool {
	for _, tu := range pendingTools {
		if tu.Name == "spawn_agent" {
			return true
		}
	}
	return false
}

// executeToolsParallel runs all tools concurrently and returns results in the
// same order as pendingTools. Each goroutine writes to its own index slot so no
// mutex is needed on the result slice. atomicCalls is incremented for each tool
// that is dispatched (used by the caller to update the tool-call budget).
func (o *Orchestrator) executeToolsParallel(ctx context.Context, pendingTools []llm.ToolUse, atomicCalls *atomic.Int64, sink StreamSink) []llm.ToolResultRecord {
	results := make([]llm.ToolResultRecord, len(pendingTools))
	var wg sync.WaitGroup
	for i, tu := range pendingTools {
		wg.Add(1)
		atomicCalls.Add(1)
		go func(index int, toolUse llm.ToolUse) {
			defer wg.Done()
			toolStart := time.Now()
			out := o.executeTool(ctx, &toolUse, sink)
			slog.Debug("tool done (parallel)", "name", toolUse.Name, "elapsed", time.Since(toolStart), "bytes", len(out.Content), "isError", out.IsError)
			content := out.Content
			isError := out.IsError
			if out.Err != nil {
				// Fatal errors in parallel mode surface as tool errors rather than
				// aborting the whole run — the sequential path handles fatal errors.
				content = out.Err.Error()
				isError = true
			}
			results[index] = llm.ToolResultRecord{
				ToolUseID: toolUse.ID,
				ToolName:  toolUse.Name,
				Content:   content,
				IsError:   isError,
			}
		}(i, tu)
	}
	wg.Wait()
	return results
}
