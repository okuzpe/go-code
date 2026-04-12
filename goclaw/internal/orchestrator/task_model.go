package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
)

// TaskModelResolution holds the outcome of per-turn model selection.
type TaskModelResolution struct {
	Role   string
	Model  string // empty means use cfg.Model() in buildRequest
	Reason string
}

var validTaskRoles = map[string]struct{}{
	"default":   {},
	"reasoning": {},
	"code":      {},
	"fast":      {},
	"explore":   {},
	"creative":  {},
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func normalizeTaskRole(r string) string {
	r = strings.ToLower(strings.TrimSpace(r))
	if _, ok := validTaskRoles[r]; ok {
		return r
	}
	return "default"
}

// classifyTaskRoleRules scores the user message with lightweight heuristics (no extra LLM calls).
func classifyTaskRoleRules(msg string, profile agents.Profile) string {
	m := strings.TrimSpace(msg)
	if m == "" {
		return profileFallbackRole(profile)
	}
	lower := strings.ToLower(m)

	codeKeywords := []string{
		"refactor", "implement", "debug", "panic", "fix ", "stack trace", "compil",
		"unit test", "typescript", "javascript", "golang", "python", "error:",
		"stacktrace", "lint ", "pull request", "commit ",
	}
	if strings.Contains(m, "```") || containsAny(lower, codeKeywords) {
		return "code"
	}

	reasoningKeywords := []string{
		"why ", "step by step", "prove ", "analiz", "analyze",
		"trade-off", "tradeoff", "implic",
	}
	if containsAny(lower, reasoningKeywords) {
		return "reasoning"
	}

	creativeKeywords := []string{"brainstorm", "marketing", "slogan", "poem", "creative", "story "}
	if containsAny(lower, creativeKeywords) {
		return "creative"
	}

	findWithContext := strings.Contains(lower, "find ") &&
		(strings.Contains(lower, "file") || strings.Contains(lower, "code") || strings.Contains(lower, "definition"))
	codebaseSearch := strings.Contains(lower, "codebase") && strings.Contains(lower, "search")
	if strings.Contains(lower, "where is") || strings.Contains(lower, "navigate") || findWithContext || codebaseSearch {
		return "explore"
	}

	role := profileFallbackRole(profile)
	// Short, single-line prompts: fast only when the profile fallback is still generic default.
	if role == "default" && !strings.Contains(m, "\n") && utf8.RuneCountInString(m) < 90 &&
		!strings.Contains(lower, "architect") &&
		!strings.Contains(lower, "design doc") {
		return "fast"
	}
	return role
}

// taskExplorationHint returns a per-turn system suffix that reinforces the tool-first rule
// for any task type that requires accessing the codebase.
func taskExplorationHint(role string) string {
	switch role {
	case "code":
		return "\n\n[THIS TURN: coding — call glob/grep/read_file FIRST. FIRST output = tool call. No text before it.]"
	case "reasoning", "explore":
		return "\n\n[THIS TURN: analysis/plan — call glob/read_file/grep FIRST to read actual files. FIRST output = tool call. No text before it.]"
	case "fast":
		// Fast/trivial: no tool hint needed, but still suppress verbose output.
		return "\n\n[THIS TURN: answer directly and briefly. No preamble.]"
	default:
		// For any other non-chat role: remind to act first.
		return "\n\n[THIS TURN: if tools are needed, FIRST output = tool call. No preamble, no explanation.]"
	}
}

func profileFallbackRole(profile agents.Profile) string {
	switch profile.Name {
	case "plan":
		return "reasoning"
	case "explore":
		return "explore"
	case "verification":
		return "fast"
	default:
		return "default"
	}
}

func (o *Orchestrator) resolveTaskModel(ctx context.Context, userMsg string) TaskModelResolution {
	if strings.TrimSpace(o.profile.ModelOverride) != "" {
		return TaskModelResolution{Reason: "profile model override"}
	}
	cfg := o.cfg
	if !cfg.TaskModelRoutingActive() {
		return TaskModelResolution{Role: "default", Reason: "router off or empty task_models"}
	}

	mode := config.NormalizeTaskModelRouter(cfg.TaskModelRouter)
	role := classifyTaskRoleRules(userMsg, o.profile)
	reason := fmt.Sprintf("rules:%s", role)

	if mode == "llm" && o.llm != nil {
		if r2, rsn, err := classifyTaskRoleLLM(ctx, o.llm, userMsg, cfg); err == nil && r2 != "" {
			role = normalizeTaskRole(r2)
			reason = "llm:" + rsn
		} else {
			if err != nil {
				slog.Debug("task model llm router failed", "err", err)
			}
			role = normalizeTaskRole(role)
			reason = fmt.Sprintf("rules_fallback:%s", role)
		}
	} else {
		role = normalizeTaskRole(role)
	}

	if m, ok := cfg.TaskModels[role]; ok && strings.TrimSpace(m) != "" {
		return TaskModelResolution{
			Role:   role,
			Model:  cfg.NormalizeModelForProvider(m),
			Reason: fmt.Sprintf("%s task_models[%s]", reason, role),
		}
	}
	if m, ok := cfg.TaskModels["default"]; ok && strings.TrimSpace(m) != "" {
		return TaskModelResolution{
			Role:   role,
			Model:  cfg.NormalizeModelForProvider(m),
			Reason: fmt.Sprintf("%s task_models[default]", reason),
		}
	}
	return TaskModelResolution{Role: role, Reason: fmt.Sprintf("%s (no task_models entry)", reason)}
}

func (o *Orchestrator) prepareTurnModel(ctx context.Context, userMsg string) {
	o.turnModel = ""
	o.taskRole = ""
	if strings.TrimSpace(o.profile.ModelOverride) != "" {
		return
	}
	res := o.resolveTaskModel(ctx, userMsg)
	o.taskRole = res.Role
	if strings.TrimSpace(res.Model) != "" {
		o.turnModel = strings.TrimSpace(res.Model)
	}
	effective := o.cfg.Model()
	if o.turnModel != "" {
		effective = o.turnModel
	}
	slog.Debug("task model routing", "role", res.Role, "model", effective, "reason", res.Reason)
}
