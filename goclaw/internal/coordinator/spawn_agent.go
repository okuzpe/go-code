// Package coordinator implements the hub-and-spoke coordinator mode (D16).
// It provides the spawn_agent tool, which creates isolated worker orchestrators
// and returns structured results to the coordinator session.
package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/text"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
)

const (
	defaultWorkerTimeoutSec = 120
	maxWorkerTimeoutSec     = 600

	workerNotificationShortIDRunes  = 8
	workerFirstNonEmptyLineMaxRunes = 200
)

// WorkerNotification is the JSON result returned by spawn_agent.
// The coordinator LLM receives this as a tool_result and synthesizes it
// into its response for the user.
type WorkerNotification struct {
	TaskID  string `json:"task_id"` // worker session id; use with stop_task
	Profile string `json:"profile"`
	Status  string `json:"status"`  // "completed" | "failed" | "running" (interactive)
	Summary string `json:"summary"` // first non-empty line of the result, or capped error text (may end with " [truncated]")
	Result  string `json:"result"`  // full worker response text
}

type spawnInput struct {
	Profile     string `json:"profile"`
	Task        string `json:"task"`
	TimeoutSec  int    `json:"timeout_sec,omitempty"`
	Interactive bool   `json:"interactive,omitempty"`
}

// SpawnAgentTool launches an isolated worker orchestrator for a sub-task.
// Workers run with their own session — they cannot see the coordinator's
// conversation history.
type SpawnAgentTool struct {
	cfg     config.Config
	client  llm.Client
	reg     *tools.Registry // worker registry (no spawn_agent — prevents nesting)
	policy  *permissions.Policy
	hookReg *hooks.Registry
	profs   map[string]agents.Profile // nil = use agents.All()

	// Context injected into every worker orchestrator so workers have the same
	// workspace awareness as the parent agent.
	workdir       string
	launchDir     string // process cwd; prompt + PerAgentMemoryDir project root
	projectCtx    string
	mem           *memory.Store
	skillsSnippet string
}

var _ tools.Tool = (*SpawnAgentTool)(nil)

// New returns a SpawnAgentTool.
// workerReg must contain only built-in tools (no spawn_agent) to prevent infinite nesting.
func New(
	cfg config.Config,
	client llm.Client,
	workerReg *tools.Registry,
	policy *permissions.Policy,
	hookReg *hooks.Registry,
) *SpawnAgentTool {
	return &SpawnAgentTool{
		cfg:     cfg,
		client:  client,
		reg:     workerReg,
		policy:  policy,
		hookReg: hookReg,
	}
}

// WithProfiles injects a merged profile map (built-in + custom) so that custom
// agent profiles defined in .goclaw/agents/*.md can be used as worker profiles.
func (t *SpawnAgentTool) WithProfiles(profs map[string]agents.Profile) *SpawnAgentTool {
	t.profs = profs
	return t
}

// WithWorkdir passes the default project directory (tool path root) to every spawned worker.
func (t *SpawnAgentTool) WithWorkdir(dir string) *SpawnAgentTool {
	t.workdir = strings.TrimSpace(dir)
	return t
}

// WithLaunchDir passes the process working directory (relative-path base) to workers.
func (t *SpawnAgentTool) WithLaunchDir(dir string) *SpawnAgentTool {
	t.launchDir = strings.TrimSpace(dir)
	return t
}

func (t *SpawnAgentTool) workerLaunchDir() string {
	if t.launchDir != "" {
		return t.launchDir
	}
	return t.workdir
}

// WithProjectContext passes the project summary to every spawned worker.
func (t *SpawnAgentTool) WithProjectContext(ctx string) *SpawnAgentTool {
	t.projectCtx = strings.TrimSpace(ctx)
	return t
}

// WithMemoryStore passes the persistent memory store to every spawned worker.
func (t *SpawnAgentTool) WithMemoryStore(mem *memory.Store) *SpawnAgentTool {
	t.mem = mem
	return t
}

// WithSkillsSnippet passes the loaded SKILL.md content to every spawned worker.
func (t *SpawnAgentTool) WithSkillsSnippet(snippet string) *SpawnAgentTool {
	t.skillsSnippet = strings.TrimSpace(snippet)
	return t
}

func (t *SpawnAgentTool) Name() string { return "spawn_agent" }

func (t *SpawnAgentTool) Description() string {
	return "Spawn an isolated sub-agent to complete a self-contained task. " +
		"The worker runs with its own context and tool access; its session is not visible to you. " +
		"Write the task field as 100–300 words: include file paths, function names, relevant code " +
		"snippets, and a clear success criterion. The worker cannot see this conversation — every " +
		"detail it needs must be in the task description. " +
		"Set interactive: true to keep the worker running after this call; the user can send more input via /focus <task_id> in the REPL until /detach or stop_task. " +
		"The profile field must be one of the configured worker profiles (see tool schema enum), typically builder or general-purpose or a full-tool custom profile for coding; " +
		"explore — read-only search; plan — read-only plan output; verification — PASS/FAIL checks; code-review — read-only diff review (no writes)."
}

func (t *SpawnAgentTool) effectiveProfiles() map[string]agents.Profile {
	if t.profs != nil {
		return t.profs
	}
	return agents.All()
}

func (t *SpawnAgentTool) InputSchema() any {
	profs := t.effectiveProfiles()
	enum := spawnWorkerProfileNames(profs)
	if len(enum) == 0 {
		enum = []string{"builder", "general-purpose", "explore", "plan", "verification", "code-review"}
	}
	profileDesc := "Worker profile name. Spawnable profiles for this workspace: " + strings.Join(enum, ", ") +
		". Prefer builder or general-purpose or a full-tool coding profile for edits and shell; explore/plan for read-only work; verification for checks; code-review for read-only diff review."
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"profile": map[string]any{
				"type":        "string",
				"description": profileDesc,
				"enum":        enum,
			},
			"task": map[string]any{
				"type":        "string",
				"description": "Fully self-contained task (100–300 words). Include file paths, function names, relevant code snippets, and a clear success criterion. The worker cannot see this conversation — every detail it needs must be here.",
			},
			"timeout_sec": map[string]any{
				"type":        "integer",
				"description": "Worker timeout in seconds (1–600). Default: 120.",
				"minimum":     1,
				"maximum":     maxWorkerTimeoutSec,
			},
			"interactive": map[string]any{
				"type":        "boolean",
				"description": "If true, worker stays running: tool returns immediately with status running; user uses /focus <task_id> to send more messages until /detach or stop_task.",
			},
		},
		"required": []string{"profile", "task"},
	}
}

func (t *SpawnAgentTool) Execute(ctx context.Context, input string) (tools.Result, error) {
	workerSink := orchestrator.StreamSinkFromContext(ctx)

	var in spawnInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}
	if strings.TrimSpace(in.Task) == "" {
		return tools.Result{Content: "task is required", IsError: true}, nil
	}

	profs := t.effectiveProfiles()
	profileKey := strings.ToLower(strings.TrimSpace(in.Profile))
	profile, ok := profs[profileKey]
	if !ok {
		return tools.Result{
			Content: fmt.Sprintf("unknown profile %q (valid worker profiles: %s)", in.Profile, joinWorkerProfileHint(profs)),
			IsError: true,
		}, nil
	}
	if hubDelegationOnly(profile) {
		return tools.Result{
			Content: fmt.Sprintf("cannot spawn hub-only profile %q as worker", profile.Name),
			IsError: true,
		}, nil
	}

	timeout := in.TimeoutSec
	if timeout <= 0 {
		timeout = defaultWorkerTimeoutSec
	}
	if timeout > maxWorkerTimeoutSec {
		timeout = maxWorkerTimeoutSec
	}

	timeoutDur := time.Duration(timeout) * time.Second
	// Interactive workers must outlive the parent orchestrator tool-call context (reqCtx),
	// which is cancelled when the user's RunStreaming turn finishes. Use a detached root
	// so the worker keeps running until timeout, stop_task, or natural exit.
	var workerCtx context.Context
	var cancel context.CancelFunc
	if in.Interactive {
		workerCtx, cancel = context.WithTimeout(context.Background(), timeoutDur)
	} else {
		workerCtx, cancel = context.WithTimeout(ctx, timeoutDur)
	}

	// Isolated session — worker never sees the coordinator's history.
	workerSess := session.New()
	taskID := workerSess.ID

	// Auto-allow all tools in the worker registry.
	// The coordinator's spawn_agent call already received user approval (if any was required).
	workerPolicy := permissions.NewPolicy()
	for _, spec := range t.reg.Specs() {
		workerPolicy.Set(spec.Name, permissions.ModeAllow)
	}

	// Each worker gets its own todo store so it doesn't share the coordinator's task list.
	workerTodoStore := todos.NewStore()
	workerOpts := []orchestrator.Option{
		orchestrator.WithTodoStore(workerTodoStore),
	}
	// Inject workspace context so workers have the same project awareness as the parent agent.
	if t.workdir != "" {
		workerOpts = append(workerOpts, orchestrator.WithWorkdir(t.workdir))
		workerOpts = append(workerOpts, orchestrator.WithLaunchDir(t.workerLaunchDir()))
	}
	if t.projectCtx != "" {
		workerOpts = append(workerOpts, orchestrator.WithProjectContext(t.projectCtx))
	}
	// Per-agent memory: if the worker profile declares a MemoryScope, give it an isolated store
	// instead of the parent (coordinator) store so each agent's memory stays scoped.
	workerMem := t.mem
	if profile.MemoryScope != "" {
		agentMemDir := memory.PerAgentMemoryDir(profile.MemoryScope, profile.Name, t.cfg.UserConfigDir, t.workerLaunchDir(), t.cfg.ProjectConfigDir)
		if err := os.MkdirAll(agentMemDir, 0o700); err != nil {
			slog.Warn("per-agent memory dir create failed; using parent store", "dir", agentMemDir, "err", err)
		} else {
			workerMem = memory.New(agentMemDir)
		}
	}
	if workerMem != nil {
		workerOpts = append(workerOpts, orchestrator.WithMemoryStore(workerMem))
	}
	if t.skillsSnippet != "" {
		workerOpts = append(workerOpts, orchestrator.WithSkillsSnippet(t.skillsSnippet))
	}

	orch := orchestrator.New(
		t.cfg,
		t.client,
		workerSess,
		t.reg,
		workerPolicy,
		t.hookReg,
		profile,
		workerOpts...,
	)

	if in.Interactive {
		registerWorkerCancel(taskID, cancel)
		inbox := make(chan workerJob, 32)
		iw := &interactiveWorker{
			taskID:  taskID,
			profile: profile.Name,
			inbox:   inbox,
			status:  "running",
			summary: "starting…",
		}
		storeInteractive(iw)
		if workerSink != nil {
			workerSink.OnTextDelta("\n▶ Worker [" + shortID(taskID) + "] · " + profile.Name + " (interactive)\n")
			workerSink.OnTextDelta("  " + firstNonEmptyLine(in.Task) + "\n\n")
		}
		go runInteractiveWorkerLoop(workerCtx, cancel, iw, orch, in.Task, workerSink)
		short := taskID
		if len(short) > 12 {
			short = short[:12] + "…"
		}
		notif := WorkerNotification{
			TaskID:  taskID,
			Profile: profile.Name,
			Status:  "running",
			Summary: fmt.Sprintf("interactive worker started — /focus %s then type; /detach for coordinator", short),
			Result:  "",
		}
		out, marshalErr := json.Marshal(notif)
		if marshalErr != nil {
			return tools.Result{Content: fmt.Sprintf("failed to encode result: %v", marshalErr), IsError: true}, nil
		}
		return tools.Result{Content: string(out)}, nil
	}

	defer cancel()
	registerWorkerCancel(taskID, cancel)
	defer unregisterWorkerCancel(taskID)

	// Emit worker start header to parent TUI.
	if workerSink != nil {
		workerSink.OnTextDelta("\n▶ Worker [" + shortID(taskID) + "] · " + profile.Name + "\n")
		workerSink.OnTextDelta("  " + firstNonEmptyLine(in.Task) + "\n\n")
	}

	// Stream worker output to the parent StreamSink when present (OnDone is not forwarded — see nested_sink.go).
	result, err := orch.RunStreaming(workerCtx, in.Task, wrapNestedWorkerSink(workerSink))

	notif := WorkerNotification{TaskID: taskID, Profile: profile.Name}
	if err != nil {
		notif.Status = "failed"
		notif.Summary = truncateWorkerSummary(fmt.Sprintf("worker failed: %v", err))
		notif.Result = result
	} else {
		notif.Status = "completed"
		notif.Result = result
		// Use the first non-empty line as the summary.
		notif.Summary = firstNonEmptyLine(result)
	}

	// Emit worker end footer to parent TUI.
	if workerSink != nil {
		workerSink.OnTextDelta("\n◀ Worker [" + shortID(taskID) + "] " + notif.Status + " — " + notif.Summary + "\n")
	}

	out, marshalErr := json.Marshal(notif)
	if marshalErr != nil {
		return tools.Result{Content: fmt.Sprintf("failed to encode result: %v", marshalErr), IsError: true}, nil
	}
	return tools.Result{Content: string(out)}, nil
}

func shortID(id string) string {
	return text.TruncateRunesHard(id, workerNotificationShortIDRunes)
}

func truncateWorkerSummary(summary string) string {
	return text.TruncateRunesWithSuffix(summary, workerFirstNonEmptyLineMaxRunes, " [truncated]")
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateWorkerSummary(line)
		}
	}
	return "(no output)"
}
