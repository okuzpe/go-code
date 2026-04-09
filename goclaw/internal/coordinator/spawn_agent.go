// Package coordinator implements the hub-and-spoke coordinator mode (D16).
// It provides the spawn_agent tool, which creates isolated worker orchestrators
// and returns structured results to the coordinator session.
package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
)

const (
	defaultWorkerTimeoutSec = 120
	maxWorkerTimeoutSec     = 600
)

// WorkerNotification is the JSON result returned by spawn_agent.
// The coordinator LLM receives this as a tool_result and synthesizes it
// into its response for the user.
type WorkerNotification struct {
	Profile string `json:"profile"`
	Status  string `json:"status"`  // "completed" | "failed"
	Summary string `json:"summary"` // first non-empty line of the result
	Result  string `json:"result"`  // full worker response text
}

type spawnInput struct {
	Profile    string `json:"profile"`
	Task       string `json:"task"`
	TimeoutSec int    `json:"timeout_sec,omitempty"`
}

// SpawnAgentTool launches an isolated worker orchestrator for a sub-task.
// Workers run with their own session — they cannot see the coordinator's
// conversation history.
type SpawnAgentTool struct {
	cfg     config.Config
	client  llm.Client
	reg     *tools.Registry    // worker registry (no spawn_agent — prevents nesting)
	policy  *permissions.Policy
	hookReg *hooks.Registry
	profs   map[string]agents.Profile // nil = use agents.All()
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

func (t *SpawnAgentTool) Name() string { return "spawn_agent" }

func (t *SpawnAgentTool) Description() string {
	return "Spawn an isolated sub-agent to complete a self-contained task. " +
		"The worker runs with its own context and tool access; its session is not visible to you. " +
		"Provide a fully self-contained task description — include all relevant file paths, " +
		"function names, and context the worker will need. " +
		"Available profiles: explore (read-only search), plan (architecture planning), " +
		"verification (run tests and report PASS/FAIL), general-purpose (full tool access)."
}

func (t *SpawnAgentTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"profile": map[string]any{
				"type":        "string",
				"description": "Worker agent profile: explore | plan | verification | general-purpose",
				"enum":        []string{"explore", "plan", "verification", "general-purpose"},
			},
			"task": map[string]any{
				"type":        "string",
				"description": "Self-contained task description for the worker. Include all context needed; the worker cannot see the current conversation.",
			},
			"timeout_sec": map[string]any{
				"type":        "integer",
				"description": "Worker timeout in seconds (1–600). Default: 120.",
				"minimum":     1,
				"maximum":     maxWorkerTimeoutSec,
			},
		},
		"required": []string{"profile", "task"},
	}
}

func (t *SpawnAgentTool) Execute(ctx context.Context, input string) (tools.Result, error) {
	var in spawnInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}
	if strings.TrimSpace(in.Task) == "" {
		return tools.Result{Content: "task is required", IsError: true}, nil
	}

	profs := t.profs
	if profs == nil {
		profs = agents.All()
	}
	profile, ok := profs[in.Profile]
	if !ok {
		return tools.Result{
			Content: fmt.Sprintf("unknown profile %q (valid: explore, plan, verification, general-purpose)", in.Profile),
			IsError: true,
		}, nil
	}
	// Prevent infinite nesting: coordinator workers cannot themselves be coordinators.
	if profile.Name == "coordinator" {
		return tools.Result{Content: "cannot spawn a coordinator worker", IsError: true}, nil
	}

	timeout := in.TimeoutSec
	if timeout <= 0 {
		timeout = defaultWorkerTimeoutSec
	}
	if timeout > maxWorkerTimeoutSec {
		timeout = maxWorkerTimeoutSec
	}

	workerCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Isolated session — worker never sees the coordinator's history.
	workerSess := session.New()

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

	// Run the worker with no streaming sink — results are captured and returned as JSON.
	result, err := orch.RunStreaming(workerCtx, in.Task, nil)

	notif := WorkerNotification{Profile: profile.Name}
	if err != nil {
		notif.Status = "failed"
		notif.Summary = fmt.Sprintf("worker failed: %v", err)
		notif.Result = result
	} else {
		notif.Status = "completed"
		notif.Result = result
		// Use the first non-empty line as the summary.
		notif.Summary = firstNonEmptyLine(result)
	}

	out, marshalErr := json.Marshal(notif)
	if marshalErr != nil {
		return tools.Result{Content: fmt.Sprintf("failed to encode result: %v", marshalErr), IsError: true}, nil
	}
	return tools.Result{Content: string(out)}, nil
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if len([]rune(line)) > 200 {
				return string([]rune(line)[:200]) + "…"
			}
			return line
		}
	}
	return "(no output)"
}
