package orchestrator

import (
	_ "embed"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/text"
	"github.com/okuzpe/goclaw/internal/tools"
)

//go:embed base_system_prompt.md
var baseSystemPrompt string

// planProfileModeMarker is injected when the active profile is "plan"; tests assert it stays present.
const planProfileModeMarker = "PLAN_PROFILE_MODE"

const (
	memoryMaxBytes          = 4096 // max bytes per memory block to prevent silent context bloat
	memoryRelevanceMinScore = 0.15 // minimum relevance score (0–1) to include a memory entry
)

// qwenFamily reports whether the model string refers to a Qwen-family model.
func qwenFamily(model string) bool {
	return strings.Contains(strings.ToLower(model), "qwen")
}

// qwenSystemSuffix is appended to the system prompt when running a Qwen-family model.
// Qwen 2.5 follows explicit numbered steps more reliably than free-form instructions.
const qwenSystemSuffix = "\n\n[MODEL NOTE: Follow explicit numbered steps. " +
	"For multi-step tasks: (1) group related tool calls logically, " +
	"(2) after tool results either continue with the next tool step or finish briefly, " +
	"(3) keep user-visible prose short and only after the relevant tools have run. " +
	"When the task involves files, code, repo, plans, or shell: do not emit <thinking> blocks or any text before the first native tool call.]"

// planProfileWorkflowOverride is appended last in buildRequest so it wins over the generic
// tool-first and Review-and-Fix instructions in the embedded base prompt when the active profile is plan.
var planProfileWorkflowOverride = "\n\n## " + planProfileModeMarker + " (overrides conflicting global workflow rules)\n\n" +
	"You are in the **plan** agent profile for this session.\n\n" +
	"- **Primary deliverable:** user-facing analysis and a structured markdown plan " +
	"(context, constraints, acceptance criteria, options or tradeoffs when helpful, ordered steps, risks, suggested verification).\n" +
	"- **Clarify only when blocked:** if a useful plan is impossible without one missing detail, ask one focused question. Otherwise continue with explicit assumptions.\n" +
	"- **Tools are optional grounding:** read_file, glob, grep, and web_search (when allowlisted) support claims about this repository or external facts. They do not replace the written plan.\n" +
	"- **Opening text allowed:** a brief framing paragraph before any tool call is permitted when it improves clarity. " +
	"Do not treat the global \"first output must be a tool call\" rule as mandatory when it conflicts with a readable plan.\n" +
	"- **Not implementation execution mode:** do not follow the Review-and-Fix execution loop " +
	"(no mandatory batching of implementation work via todo_write). Omit execution-style todo_write unless the user explicitly wants a visible checklist in session memory.\n" +
	"- **Review gate:** stop after delivering the plan. Treat implementation as a separate phase that starts only after the user approves the plan or explicitly runs `/apply-plan` or `/plan run`. Prefer the review-first path.\n" +
	"- **Honesty:** never claim files were written or commands ran unless a tool actually ran and succeeded.\n" +
	"- **Mandatory close:** after the plan, ask exactly one explicit next-step question that offers these choices in plain language: implement now, adjust the plan, or save/review first.\n" +
	"- **Critical files (repo-grounded plans only):** when the plan depends on this repository's layout or code, include a subsection **Critical files (3–5)** listing three to five repository-relative paths " +
	"that an implementer must touch or read first; each line must be a path you have seen via read_file, glob, or grep — never invent paths.\n\n" +
	"If the request is purely conceptual and needs no repository evidence, answer from reasoning alone without tools."

func buildLitePhaseAllowlist(ut *userTurnState) map[string]struct{} {
	names := []string{
		"read_file",
		"glob",
		"grep",
		"edit_file",
		"write_file",
		"patch",
	}
	if ut != nil && ut.verifyPending {
		names = []string{
			"run_tests",
			"run_command",
			"read_file",
			"edit_file",
			"write_file",
			"patch",
		}
	}
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[n] = struct{}{}
	}
	return out
}

func buildLitePhasePromptBlock(ut *userTurnState) string {
	if ut != nil && ut.verifyPending {
		return "\n\n## Build-lite phase\nYou are in verify/repair mode. Use run_tests or run_command to verify. If verification fails, inspect with read_file, repair with edit_file/write_file/patch, then rerun the same verification."
	}
	return "\n\n## Build-lite phase\nYou are in inspect/edit mode. First inspect with read_file, glob, or grep. Then make the edit with edit_file, write_file, or patch. Do not verify until after a successful workspace write."
}

// effectiveToolSpecs returns registry specs after hidden/revealed filtering, profile allowlist,
// disallowed tools, and read-only stripping.
//
// MCP tools are registered as hidden by default and excluded from the LLM prompt unless:
//   - the model called tool_search this turn (revealed via revealedToolNames), or
//   - the active profile's allowlist explicitly names the tool (or matches a wildcard prefix).
func (o *Orchestrator) effectiveToolSpecs() []tools.ToolSpec {
	allSpecs := o.tools.AllSpecs()

	// Build the allowlist map once so it can serve double duty: hidden-override check and
	// the standard allowlist filter below.
	var allowlistMap map[string]struct{}
	hasAllowlist := o.profile.ToolAllowlist != nil
	if hasAllowlist && len(o.profile.ToolAllowlist) > 0 {
		allowlistMap = make(map[string]struct{}, len(o.profile.ToolAllowlist))
		for _, n := range o.profile.ToolAllowlist {
			allowlistMap[n] = struct{}{}
		}
	}

	// Collect tools revealed via tool_search during the current turn.
	var revealed map[string]bool
	if o.ut != nil {
		revealed = o.ut.revealedToolNames
	}

	// Pass 1: drop hidden tools unless the model revealed them or the allowlist names them.
	specs := make([]tools.ToolSpec, 0, len(allSpecs))
	for _, s := range allSpecs {
		if o.tools.IsHidden(s.Name) {
			if !revealed[s.Name] && !toolMatchesAllowlist(s.Name, allowlistMap) {
				continue
			}
		}
		specs = append(specs, s)
	}

	// Pass 2: apply the profile allowlist (standard filter).
	if hasAllowlist {
		if len(allowlistMap) == 0 {
			specs = nil
		} else {
			filtered := make([]tools.ToolSpec, 0, len(specs))
			for _, s := range specs {
				if toolMatchesAllowlist(s.Name, allowlistMap) {
					filtered = append(filtered, s)
				}
			}
			specs = filtered
		}
	}

	if len(o.profile.DisallowedTools) > 0 {
		denied := make(map[string]struct{}, len(o.profile.DisallowedTools))
		for _, n := range o.profile.DisallowedTools {
			denied[n] = struct{}{}
		}
		filtered := make([]tools.ToolSpec, 0, len(specs))
		for _, s := range specs {
			if _, blocked := denied[s.Name]; !blocked {
				filtered = append(filtered, s)
			}
		}
		specs = filtered
	}
	if o.profile.ReadOnly {
		specs = stripToolName(specs, "bash")
		specs = stripToolName(specs, "run_command")
		specs = stripToolName(specs, "write_file")
		specs = stripToolName(specs, "edit_file")
		specs = stripToolName(specs, "patch")
		specs = stripMCPNames(specs)
	}
	if o.usesBuildLiteRuntime() {
		phaseAllow := buildLitePhaseAllowlist(o.ut)
		filtered := make([]tools.ToolSpec, 0, len(specs))
		for _, s := range specs {
			if _, ok := phaseAllow[s.Name]; ok {
				filtered = append(filtered, s)
			}
		}
		specs = filtered
	}
	return specs
}

func (o *Orchestrator) buildRequest() llm.Request {
	model := o.cfg.Model()
	turnModel := ""
	if o.ut != nil {
		turnModel = strings.TrimSpace(o.ut.turnModel)
	}
	if o.profile.ModelOverride != "" {
		model = o.profile.ModelOverride
	} else if turnModel != "" {
		model = turnModel
	}

	specs := o.effectiveToolSpecs()

	llmTools := make([]llm.ToolSpec, 0, len(specs))
	for _, s := range specs {
		llmTools = append(llmTools, llm.ToolSpec{
			Name:        s.Name,
			Description: s.Description,
			InputSchema: s.InputSchema,
		})
	}

	envKey := o.environmentSystemMemoKey(model)
	if o.cachedEnvSystemKey != envKey || o.cachedEnvSystem == "" {
		o.cachedEnvSystemKey = envKey
		o.cachedEnvSystem = o.buildStaticSystemThroughQwen(model)
	}
	sys := o.cachedEnvSystem

	turnUserMessage := ""
	turnInputLang := ""
	tuiInteractApply := false
	tuiInteractMode := ""
	budgetIter := 0
	budgetLimit := 0
	budgetToolCalls := 0
	turnHadToolRound := false
	turnWorkspaceWriteOK := false
	taskRole := ""
	if o.ut != nil {
		turnUserMessage = o.ut.turnUserMessage
		turnInputLang = o.ut.turnInputLang
		tuiInteractApply = o.ut.tuiInteractApply
		tuiInteractMode = o.ut.tuiInteractMode
		budgetIter = o.ut.budgetIter
		budgetLimit = o.ut.budgetLimit
		budgetToolCalls = o.ut.budgetToolCalls
		turnHadToolRound = o.ut.turnHadToolRound
		turnWorkspaceWriteOK = o.ut.turnWorkspaceWriteOK
		taskRole = o.ut.taskRole
	}

	if o.mem != nil && !o.usesBuildLiteRuntime() {
		if block, err := o.mem.RelevantContext(o.cfg.EffectiveMaxMemorySnippetEntries(), turnUserMessage, memoryRelevanceMinScore); err == nil && block != "" {
			block = truncateMemoryBlock(block, memoryMaxBytes)
			sys = sys + "\n\n## Persistent memory (relevant)\n" + block
		}
	}
	if o.projectMem != nil && !o.usesBuildLiteRuntime() {
		if block, err := o.projectMem.RelevantContext(o.cfg.EffectiveMaxMemorySnippetEntries(), turnUserMessage, memoryRelevanceMinScore); err == nil && block != "" {
			block = truncateMemoryBlock(block, memoryMaxBytes)
			sys = sys + "\n\n## Project memory (.goclaw/memory)\n" + block
		}
	}
	if o.todoStore != nil && !o.usesBuildLiteRuntime() {
		if block := o.todoStore.FormatForPrompt(); block != "" {
			sys = sys + "\n\n## Session task list (todo_write)\n" + block
		}
	}

	if o.usesBuildLiteRuntime() {
		sys = sys + buildLitePhasePromptBlock(o.ut)
	} else if hint := taskExplorationHint(taskRole); hint != "" {
		sys = sys + hint
	}
	if !o.usesBuildLiteRuntime() {
		if hint := o.hiddenMCPToolsPromptHint(specs); hint != "" {
			sys = sys + hint
		}
	}
	if block := verifyChangedPathsBlock(o.ut, o.workdir, o.usesBuildLiteRuntime()); block != "" {
		sys = sys + block
	}

	// When input translation is active (normalize_input_language), the session already holds the
	// English translation so detecting the language from session text would produce "en" and instruct
	// the model to reply in English. turnInputLang holds the original language tag detected before
	// translation; use it directly when available to preserve the user's language.
	if !o.usesBuildLiteRuntime() {
		if langHint := langReplyHint(turnInputLang, o.cfg, o.session.Messages); langHint != "" {
			sys = sys + langHint
		}
	}

	if hint := tuiInteractModePromptBlock(tuiInteractApply, tuiInteractMode); hint != "" {
		sys = sys + hint
	}

	// Budget reminder: inject once we are past the halfway point of available iterations,
	// so the model knows it is approaching its ceiling and should wrap up or report.
	if budgetIter > 0 && budgetLimit > 0 && budgetIter > budgetLimit/2 {
		remaining := budgetLimit - budgetIter
		toolsLeft := o.effectiveMaxToolCalls() - budgetToolCalls
		softenBudget := !o.profile.ReadOnly &&
			toolSpecsAllowWorkspaceWrite(specs) &&
			userMessageWantsWorkspaceWrites(turnUserMessage) &&
			turnHadToolRound &&
			!turnWorkspaceWriteOK
		var body string
		if softenBudget {
			body = "You are running low on iteration budget. If the user asked for concrete code changes, prioritize completing in-flight edits (write_file, edit_file, patch) or clearly stating blockers — avoid stopping with analysis-only while edits are still expected."
		} else {
			body = "If you are close to finishing, wrap up and report rather than starting new work."
		}
		sys = sys + fmt.Sprintf(
			"\n\n<system-reminder>Budget: iteration %d/%d (%d remaining). Tool calls used: %d/%d (%d remaining). %s</system-reminder>",
			budgetIter, budgetLimit, remaining,
			budgetToolCalls, o.effectiveMaxToolCalls(), toolsLeft,
			body,
		)
	}

	if o.profile.Name == agents.Plan.Name {
		sys = sys + planProfileWorkflowOverride
	}

	maxTokens := 4096
	if o.cfg.MaxResponseTokens > 0 {
		maxTokens = o.cfg.MaxResponseTokens
	}
	req := llm.Request{
		Model:     model,
		System:    sys,
		Messages:  o.session.Messages,
		Tools:     llmTools,
		MaxTokens: maxTokens,
	}
	if o.cfg.Provider == "ollama" && o.cfg.OllamaNumCtx > 0 {
		req.NumCtx = o.cfg.OllamaNumCtx
	}
	return req
}

func stripToolName(specs []tools.ToolSpec, name string) []tools.ToolSpec {
	out := specs[:0]
	for _, s := range specs {
		if s.Name != name {
			out = append(out, s)
		}
	}
	return out
}

func stripMCPNames(specs []tools.ToolSpec) []tools.ToolSpec {
	out := specs[:0]
	for _, s := range specs {
		if !strings.HasPrefix(s.Name, "mcp__") {
			out = append(out, s)
		}
	}
	return out
}

func (o *Orchestrator) hiddenMCPToolsPromptHint(specs []tools.ToolSpec) string {
	if o == nil || o.tools == nil || o.profile.ReadOnly {
		return ""
	}
	if !toolSpecNamesContain(specs, "tool_search") {
		return ""
	}
	hiddenCount := 0
	for _, s := range o.tools.AllSpecs() {
		if !strings.HasPrefix(s.Name, "mcp__") || !o.tools.IsHidden(s.Name) {
			continue
		}
		if toolSpecNamesContain(specs, s.Name) {
			continue
		}
		hiddenCount++
	}
	if hiddenCount == 0 {
		return ""
	}
	return fmt.Sprintf(
		"\n\n## Hidden MCP tools\n%d MCP tools are available but hidden by default to save context. Use tool_search to discover the right MCP tool before calling it.",
		hiddenCount,
	)
}

func toolSpecNamesContain(specs []tools.ToolSpec, name string) bool {
	for _, s := range specs {
		if s.Name == name {
			return true
		}
	}
	return false
}

// toolMatchesAllowlist supports exact names and trailing-wildcard prefixes (e.g. mcp__demo__*).
func toolMatchesAllowlist(name string, allow map[string]struct{}) bool {
	if _, ok := allow[name]; ok {
		return true
	}
	for pat := range allow {
		if strings.HasSuffix(pat, "*") && len(pat) > 1 {
			prefix := strings.TrimSuffix(pat, "*")
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
	}
	return false
}

// truncateMemoryBlock caps a memory block at maxRunes runes, appending a marker
// when truncated. Using rune boundaries avoids splitting multibyte UTF-8 characters
// (e.g. emoji or CJK) that byte-slicing would corrupt.
func truncateMemoryBlock(block string, maxRunes int) string {
	if utf8.RuneCountInString(block) <= maxRunes {
		return block
	}
	return text.TruncateRunes(block, maxRunes) + "\n[memory truncated]"
}
