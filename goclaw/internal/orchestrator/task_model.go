package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
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
	"fix":       {},
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

// rulesRouterHighConfidence reports whether the rules classifier result is reliable enough
// to skip the LLM router call for this message. Skipping avoids the round-trip latency
// when the outcome is already obvious:
//   - Very short prompts (< 60 runes): rules "fast" is correct; an LLM call adds no value.
//   - "explore": strong keyword signals (where is, search for, etc.) that LLM rarely overrides.
//   - Profile fallback only ("default"): LLM may genuinely help here — don't skip.
func rulesRouterHighConfidence(msg, rulesRole string) bool {
	runes := utf8.RuneCountInString(strings.TrimSpace(msg))
	switch rulesRole {
	case "fast":
		return runes < 60
	case "explore":
		return true
	default:
		return false
	}
}

// classifyTaskRoleRules scores the user message with lightweight heuristics (no extra LLM calls).
func classifyTaskRoleRules(msg string, profile agents.Profile) string {
	if profile.Name == "code-review" {
		return "reasoning"
	}
	m := strings.TrimSpace(msg)
	if m == "" {
		return profileFallbackRole(profile)
	}
	lower := strings.ToLower(m)

	// Review-and-fix tasks: checked FIRST — "analyze and improve" is a fix task, not pure reasoning.
	// Any message that combines analysis intent with improvement/change intent → fix role.
	fixKeywords := []string{
		"revisa", "arregla", "arreglalo", "arreglarlo", "encuentra gaps", "find gaps",
		"fix ", "review and fix", "audit and", "improve", "mejorar", "mejoras",
		"refactor", "clean up", "find issues", "identify and fix", "diagnose", "detecta",
		"find and fix", "gaps para", "propon", "propón", "proponer", "pulir", "pulirlo",
		"puedes mejorar", "puedes arreglar", "puedes revisar", "y arregla", "y mejora",
		"y propon", "y propón",
		// Additional Spanish imperative / subjunctive forms.
		"pulelo", "puledlo", "encuentres", "analices", "examina",
		"revisalo", "revísalo", "busca gaps", "busca errores",
		"arreglame", "arréglame", "encuentra y", "analiza y",
	}
	reasoningKeywords := []string{
		"why ", "step by step", "prove ", "analiz", "analyze",
		"trade-off", "tradeoff", "implic", "explain",
	}
	// Fix wins even when the message also has reasoning intent (e.g. "analiza y arregla").
	hasFix := containsAny(lower, fixKeywords)
	hasReasoning := containsAny(lower, reasoningKeywords)
	if hasFix || (hasReasoning && containsAny(lower, []string{"arregl", "mejor", "fix", "refactor", "gaps"})) {
		return "fix"
	}

	codeKeywords := []string{
		"implement", "debug", "panic", "stack trace", "compil",
		"unit test", "typescript", "javascript", "golang", "python", "error:",
		"stacktrace", "lint ", "pull request", "git commit", "commit message",
	}
	if strings.Contains(m, "```") || containsAny(lower, codeKeywords) {
		return "code"
	}

	if containsAny(lower, reasoningKeywords) {
		return "reasoning"
	}

	creativeKeywords := []string{"brainstorm", "marketing", "slogan", "poem", "creative", "story "}
	if containsAny(lower, creativeKeywords) {
		return "creative"
	}

	findWithContext := strings.Contains(lower, "find ") &&
		(strings.Contains(lower, "file") || strings.Contains(lower, "code") || strings.Contains(lower, "symbol") || strings.Contains(lower, "definition"))
	codebaseSearch := strings.Contains(lower, "codebase") && strings.Contains(lower, "search")
	searchFor := strings.Contains(lower, "search for") || strings.Contains(lower, "look for")
	if strings.Contains(lower, "where is") || strings.Contains(lower, "navigate") || findWithContext || codebaseSearch || searchFor {
		return "explore"
	}

	// Short social openers (including a few newlines) should not inherit a coding phase label
	// or tool-heavy hints before we have a real task signal. Scoped to direct-coding profiles so
	// hub/explore/plan fallbacks stay unchanged.
	if (profile.Name == "general-purpose" || profile.Name == "builder") && conversationalChatTurn(lower, m) {
		return "fast"
	}

	role := profileFallbackRole(profile)
	// Vague but task-flavoured prompts (e.g. "good code question") should pick a coding role, not the
	// generic short-chat fast path.
	if role == "default" && conversationalTurnHasTaskSignals(lower) {
		switch profile.Name {
		case "general-purpose", "builder":
			return "code"
		}
	}
	// Short, single-line prompts: fast only when the profile fallback is still generic default.
	if role == "default" && !strings.Contains(m, "\n") && utf8.RuneCountInString(m) < 120 &&
		!strings.Contains(lower, "architect") &&
		!strings.Contains(lower, "design doc") &&
		!conversationalTurnHasTaskSignals(lower) {
		return "fast"
	}
	return role
}

// conversationalChatTurn is true for brief greetings / thanks / goodbyes in common languages.
// It runs after code/fix/reasoning/explore heuristics so task-like phrases still win.
func conversationalChatTurn(lower, m string) bool {
	m = strings.TrimSpace(m)
	if m == "" || strings.Contains(m, "```") {
		return false
	}
	if utf8.RuneCountInString(m) > 140 {
		return false
	}
	fields := strings.Fields(m)
	if len(fields) == 0 || len(fields) > 10 {
		return false
	}
	first := strings.Trim(strings.ToLower(fields[0]), ",.:;!?¡¿")
	switch first {
	case "hola", "hi", "hello", "hey", "buenas", "buenos", "gracias", "thanks", "thank",
		"bye", "adiós", "adios", "chao", "vale", "ok", "okay", "yes", "no", "sí", "si",
		"good", "morning", "evening", "afternoon", "what's", "whats", "sup":
	default:
		return false
	}
	return !conversationalTurnHasTaskSignals(lower)
}

// conversationalTurnHasTaskSignals catches coding/repo vocabulary missed by earlier
// keyword lists (e.g. "good code" as two words).
func conversationalTurnHasTaskSignals(lower string) bool {
	phraseHits := []string{
		"implement", "debug", "panic", "compil", "refactor", "repository", "codebase",
		"pull request", "git commit", "lint ", "stack trace", "stacktrace", "unit test",
		"typescript", "javascript", "golang", "python", "error:",
	}
	if containsAny(lower, phraseHits) {
		return true
	}
	for _, tok := range strings.Fields(lower) {
		t := strings.Trim(tok, ",.:;!?¡¿()[]{}\"'`")
		switch t {
		case "code", "file", "repo", "branch", "commit", "bug", "test", "tests":
			return true
		}
	}
	return false
}

// taskExplorationHint returns a per-turn system suffix that reinforces the tool-first rule
// for any task type that requires accessing the codebase.
func taskExplorationHint(role string) string {
	const noNarration = " BANNED before any real tool_use: the literal substring TOOL CALL; phrases I'll/I will invoke/use/run; Let me / Vamos a; fenced ```tool / ```json tool stubs; <function_calls> or XML/JSON that only *describes* tools. " +
		"Those lines do not execute — zero bytes read from disk. Your first emitted content in this turn must be a native API tool call (or, after tools finish, one short line only). Narrating tools = failure."
	switch role {
	case "code":
		return "\n\n[THIS TURN: coding. IMMEDIATE first output = native tool_use (glob / grep / read_file / bash as needed)." + noNarration + "]"
	case "fix":
		return "\n\n[THIS TURN: review-and-fix. FIRST output must be a tool call (glob). " +
			"After reading enough files (see base prompt analysis rules): apply fixes with edit_file or write_file, then verify with bash or script. " +
			"If bash/script returns non-zero, diagnose the error, apply the targeted fix, and re-verify (max 2 retries). After 2 failed retries, stop and report the failure with full evidence. " +
			"End with one short paragraph — not a suggestion list." +
			noNarration + "]"
	case "reasoning", "explore":
		return "\n\n[THIS TURN: analysis. IMMEDIATE first output = native tool_use (glob / read_file / grep)." + noNarration + "]"
	case "fast":
		return "\n\n[THIS TURN: answer directly and briefly in the user's language (same language they wrote). No preamble. If they still need repo facts, use tools first — same narration ban applies. Never say \"no tool call\" or similar meta to the user.]"
	default:
		return "\n\n[THIS TURN: if the answer needs the repo or shell, IMMEDIATE first output = native tool_use." + noNarration + "]"
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
	case "code-review":
		return "reasoning"
	case "general-purpose", "builder":
		// Default to "default" so short/vague turns route to the fast path (direct reply) and
		// the TUI shows neutral "Planning"/"Thinking" instead of implying repo analysis before
		// we have any coding signal. Explicit code/fix/explore keywords still select those roles.
		return "default"
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

	if mode == "llm" && o.llm != nil && !rulesRouterHighConfidence(userMsg, role) {
		start := time.Now()
		if r2, rsn, err := classifyTaskRoleLLM(ctx, o.llm, userMsg, cfg); err == nil && r2 != "" {
			role = normalizeTaskRole(r2)
			reason = fmt.Sprintf("llm:%s (%.0fms)", rsn, float64(time.Since(start).Milliseconds()))
		} else {
			elapsed := time.Since(start)
			if err != nil {
				slog.Warn("task model llm router failed, using rules classification",
					"err", err, "elapsed", elapsed.Round(time.Millisecond), "rules_role", role)
			}
			role = normalizeTaskRole(role)
			reason = fmt.Sprintf("rules_fallback:%s (%.0fms)", role, float64(elapsed.Milliseconds()))
		}
	} else {
		role = normalizeTaskRole(role)
		if mode == "llm" && rulesRouterHighConfidence(userMsg, role) {
			reason = fmt.Sprintf("rules_confident:%s", role)
		}
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
