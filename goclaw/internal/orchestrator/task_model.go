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

	if strings.Contains(m, "```") ||
		strings.Contains(lower, "refactor") ||
		strings.Contains(lower, "implement") ||
		strings.Contains(lower, "debug") ||
		strings.Contains(lower, "panic") ||
		strings.Contains(lower, "fix ") ||
		strings.Contains(lower, "stack trace") ||
		strings.Contains(lower, "compil") ||
		strings.Contains(lower, "unit test") ||
		strings.Contains(lower, "typescript") ||
		strings.Contains(lower, "javascript") ||
		strings.Contains(lower, "golang") ||
		strings.Contains(lower, "python") ||
		strings.Contains(lower, "error:") ||
		strings.Contains(lower, "stacktrace") ||
		strings.Contains(lower, "lint ") ||
		strings.Contains(lower, " pull request") ||
		strings.Contains(lower, "pull request") ||
		strings.Contains(lower, "commit ") {
		return "code"
	}

	if strings.Contains(lower, "why ") ||
		strings.Contains(lower, "step by step") ||
		strings.Contains(lower, "prove ") ||
		strings.Contains(lower, "analiz") ||
		strings.Contains(lower, "analyze") ||
		strings.Contains(lower, "trade-off") ||
		strings.Contains(lower, "tradeoff") ||
		strings.Contains(lower, "razon") ||
		strings.Contains(lower, "implic") {
		return "reasoning"
	}

	if strings.Contains(lower, "brainstorm") ||
		strings.Contains(lower, "marketing") ||
		strings.Contains(lower, "slogan") ||
		strings.Contains(lower, "poem") ||
		strings.Contains(lower, "creative") ||
		strings.Contains(lower, "story ") {
		return "creative"
	}

	if strings.Contains(lower, "where is") ||
		strings.Contains(lower, "find ") && (strings.Contains(lower, "file") || strings.Contains(lower, "code") || strings.Contains(lower, "definition")) ||
		strings.Contains(lower, "navigate") ||
		strings.Contains(lower, "codebase") && strings.Contains(lower, "search") {
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
	if strings.TrimSpace(o.profile.ModelOverride) != "" {
		return
	}
	res := o.resolveTaskModel(ctx, userMsg)
	if strings.TrimSpace(res.Model) != "" {
		o.turnModel = strings.TrimSpace(res.Model)
	}
	effective := o.cfg.Model()
	if o.turnModel != "" {
		effective = o.turnModel
	}
	slog.Debug("task model routing", "role", res.Role, "model", effective, "reason", res.Reason)
}
