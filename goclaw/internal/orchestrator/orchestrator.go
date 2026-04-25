// Package orchestrator implements the main agent loop:
// user input → LLM → tool execution → feedback → repeat (max 32 iterations).
package orchestrator

import (
	"context"
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

// adaptIterBudget returns an adjusted iteration cap based on the classified task role.
// Lightweight roles (explore, fast) rarely need the full budget; reducing them saves
// context and avoids unnecessary LLM calls on simple read-only turns.
func adaptIterBudget(base int, role string) int {
	switch role {
	case "explore", "fast":
		if half := base / 2; half >= 4 {
			return half
		}
		return 4
	case "research":
		// Research turns need full iterations: multiple web_search rounds + synthesis.
		return base
	default:
		return base
	}
}

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
		o.resetEnvSystemCache()
	}
}

// StreamSink receives streaming and tool lifecycle events during RunStreaming.
// All methods must be fast and non-blocking (or handle their own buffering),
// since they run on the orchestrator's goroutines.
type StreamSink interface {
	// OnThinkingStart is called at the beginning of each LLM stream call (including
	// between tool iterations). phase is a short English line from ThinkingPhaseLine (may be empty).
	OnThinkingStart(phase string)
	OnTextDelta(text string)
	// OnToolUse is emitted when the model requests a tool. toolUseID is the wire id (may be empty for synthetic invocations).
	OnToolUse(toolUseID, name, rawInput string)
	// OnToolResult is called after each tool finishes. content is the full result
	// string (may be large; callers may cap display to a reasonable limit).
	OnToolResult(toolUseID, name string, content string, isError bool)
	// OnToolProgress is called with incremental partial output from a running tool
	// (e.g. bash stdout lines as they arrive). May be called many times between
	// OnToolUse and OnToolResult for the same tool.
	OnToolProgress(name, partial string)
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
		o.resetEnvSystemCache()
	}
}

// WithLaunchDir injects the process working directory at agent start. Relative paths in
// file tools resolve from here unless the model passes an absolute path. Optional; if
// empty, buildRequest only describes the tool path root.
func WithLaunchDir(dir string) Option {
	return func(o *Orchestrator) {
		o.launchDir = strings.TrimSpace(dir)
		o.resetEnvSystemCache()
	}
}

// WithScratchDir sets the absolute session scratch directory for ephemeral model writes
// (auto-approved under ask policy when targets stay under this tree). Empty disables.
func WithScratchDir(dir string) Option {
	return func(o *Orchestrator) {
		o.scratchDir = strings.TrimSpace(dir)
		o.resetEnvSystemCache()
	}
}

// WithProjectContext injects a brief project summary (e.g. from go.mod / README)
// into the system prompt so the agent understands the project without a warm-up tool call.
func WithProjectContext(ctx string) Option {
	return func(o *Orchestrator) {
		o.projectContext = strings.TrimSpace(ctx)
		o.resetEnvSystemCache()
	}
}

// WithProjectContextThin injects a reduced project summary (no CLAUDE.md / standing orders).
func WithProjectContextThin(ctx string) Option {
	return func(o *Orchestrator) {
		o.projectContextThin = strings.TrimSpace(ctx)
		o.resetEnvSystemCache()
	}
}

// WithProjectContextLite injects the minimal build-lite project summary.
func WithProjectContextLite(ctx string) Option {
	return func(o *Orchestrator) {
		o.projectContextLite = strings.TrimSpace(ctx)
		o.resetEnvSystemCache()
	}
}

// Orchestrator wires all subsystems and drives the agent loop.
type Orchestrator struct {
	cfg          config.Config
	llm          llm.Client
	session      *session.Session
	tools        *tools.Registry
	perms        *permissions.Policy
	hooks        *hooks.Registry
	profile      agents.Profile
	approver     ToolApprover
	mem          *memory.Store
	projectMem   *memory.Store // per-project memory store (D14); nil if not present
	afterTool    AfterToolHook
	skillsPrompt string
	todoStore    *todos.Store

	// workdir is the tool path root injected into the system prompt (optional).
	workdir string

	// launchDir is the cwd when goclaw started; injected for path-resolution hints (optional).
	launchDir string

	// projectContext is a brief project summary injected into the system prompt (optional).
	projectContext string
	// projectContextThin omits repo conventions; used by plan/explore-style turns.
	projectContextThin string
	// projectContextLite is the minimal context used by build-lite.
	projectContextLite string

	// ut holds per-user-turn state; non-nil only during runUserTurn.
	ut *userTurnState

	// nextTUIInteract is copied into ut at the start of the next runUserTurn (SetTurnInteractMode).
	nextTUIInteractApply bool
	nextTUIInteractMode  string

	// scratchDir is an absolute session scratch directory (ephemeral notes); empty when disabled.
	scratchDir string

	checkpointMu sync.Mutex
	checkpoints  []checkpointEntry

	// cachedEnvSystem / cachedEnvSystemKey memoize the stable workspace+skills portion of buildRequest.
	cachedEnvSystemKey string
	cachedEnvSystem    string

	// compactionIneffectiveStreak counts consecutive phase-2 compactions that failed to drop
	// estimated context below the auto-compact limit.
	compactionIneffectiveStreak int
	// autoCompactDisabled stops maybeCompact after repeated ineffective compactions (session-only).
	// ForceCompact and /compact are unaffected.
	autoCompactDisabled bool
}

type turnMetrics struct {
	start       time.Time
	translation time.Duration
	stream      time.Duration
	tool        time.Duration
	toolCalls   int
	status      string
}

func (o *Orchestrator) logTurnMetrics(metrics turnMetrics) {
	role := ""
	if o.ut != nil {
		role = o.ut.taskRole
	}
	attrs := []any{
		"role", role,
		"translation_ms", metrics.translation.Milliseconds(),
		"stream_ms", metrics.stream.Milliseconds(),
		"tool_ms", metrics.tool.Milliseconds(),
		"tool_calls", metrics.toolCalls,
		"turn_ms", time.Since(metrics.start).Milliseconds(),
	}
	if strings.TrimSpace(metrics.status) != "" {
		attrs = append(attrs, "status", metrics.status)
	}
	slog.Info("turn metrics", attrs...)
}

func (o *Orchestrator) resetEnvSystemCache() {
	o.cachedEnvSystemKey = ""
	o.cachedEnvSystem = ""
}

func (o *Orchestrator) syncWireToolRequirement() {
	if o == nil {
		return
	}
	oc, ok := o.llm.(*llm.OllamaClient)
	if !ok {
		return
	}
	oc.RequireWireTools = o.cfg.OllamaRequireWireTools || o.usesBuildLiteRuntime()
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
	o.syncWireToolRequirement()
	return o
}

func (o *Orchestrator) effectiveMaxIterations() int {
	n := o.cfg.MaxOrchestratorIterations
	if n <= 0 {
		return maxIterations
	}
	if n > 256 {
		return 256
	}
	return n
}

func (o *Orchestrator) effectiveMaxToolCalls() int {
	n := o.cfg.MaxOrchestratorToolCalls
	if n <= 0 {
		return maxToolCalls
	}
	if n > 512 {
		return 512
	}
	return n
}

func (o *Orchestrator) effectiveParallelToolBatchMax() int {
	if o != nil && o.usesBuildLiteRuntime() {
		return 1
	}
	return o.cfg.EffectiveParallelToolBatchMax()
}

// TaskRole returns the classified task role for the current turn (e.g. "fix", "code", "explore").
// Returns "" between turns.
func (o *Orchestrator) TaskRole() string {
	if o.ut == nil {
		return ""
	}
	return o.ut.taskRole
}

// TurnModel returns the model id resolved for the current turn.
// Returns "" when the default model (cfg.Model()) is used.
func (o *Orchestrator) TurnModel() string {
	if o.ut == nil {
		return ""
	}
	return o.ut.turnModel
}

// SetTurnInteractMode sets the TUI interact mode (chat | code | agent) for the next RunStreaming
// call. The hint is injected into the system prompt for that user turn only. Call with any value
// before each submit from the fullscreen chat; omit for non-TUI callers. Empty mode disables the
// hint until the next SetTurnInteractMode (ClearTurnInteractMode is equivalent).
func (o *Orchestrator) SetTurnInteractMode(mode string) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		o.nextTUIInteractApply = false
		o.nextTUIInteractMode = ""
		return
	}
	o.nextTUIInteractApply = true
	o.nextTUIInteractMode = config.NormalizeTUIInteractMode(mode)
}

// ClearTurnInteractMode removes the TUI interact hint before the next model call.
func (o *Orchestrator) ClearTurnInteractMode() {
	o.nextTUIInteractApply = false
	o.nextTUIInteractMode = ""
}

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

// ReplaceSession switches the in-memory conversation to s (e.g. after /new in the REPL).
func (o *Orchestrator) ReplaceSession(s *session.Session) {
	if s == nil {
		return
	}
	o.session = s
	o.clearCheckpoints()
	if o.todoStore != nil {
		o.todoStore.Clear()
	}
}

// SetProfile switches the active agent profile (tool allowlist, read-only flag, system prompt suffix).
func (o *Orchestrator) SetProfile(p agents.Profile) {
	o.profile = p
	o.resetEnvSystemCache()
	o.syncWireToolRequirement()
}

// SetConfig replaces the orchestrator config snapshot (e.g. after /model in the REPL).
func (o *Orchestrator) SetConfig(cfg config.Config) {
	o.cfg = cfg
	o.resetEnvSystemCache()
	o.syncWireToolRequirement()
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

// executeToolsParallel runs all tools concurrently and returns results in the
// same order as pendingTools. Each goroutine writes to its own index slot so no
// mutex is needed on the result slice. atomicCalls is incremented for each tool
// that is dispatched (used by the caller to update the tool-call budget).
func (o *Orchestrator) executeToolsParallel(ctx context.Context, pendingTools []llm.ToolUse, atomicCalls *atomic.Int64, sink StreamSink) ([]llm.ToolResultRecord, error) {
	results := make([]llm.ToolResultRecord, len(pendingTools))
	var wg sync.WaitGroup
	var fatalErr error
	var fatalErrMu sync.Mutex
	for i, tu := range pendingTools {
		wg.Add(1)
		atomicCalls.Add(1)
		go func(index int, toolUse llm.ToolUse) {
			defer wg.Done()
			toolStart := time.Now()
			out := o.executeTool(ctx, &toolUse, sink)
			slog.Debug("tool done (parallel)", "name", toolUse.Name, "elapsed", time.Since(toolStart), "bytes", len(out.Content), "isError", out.IsError)
			if out.Err != nil {
				fatalErrMu.Lock()
				if fatalErr == nil {
					fatalErr = out.Err
				}
				fatalErrMu.Unlock()
			}
			results[index] = llm.ToolResultRecord{
				ToolUseID: toolUse.ID,
				ToolName:  toolUse.Name,
				Content:   out.Content,
				IsError:   out.IsError,
			}
		}(i, tu)
	}
	wg.Wait()
	if fatalErr != nil {
		return nil, fatalErr
	}
	return results, nil
}
