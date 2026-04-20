package orchestrator

import (
	"strings"

	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/tools"
)

const (
	defaultActionNudgesPerUserTurn = 2
	maxActionNudgesCap             = 5

	// actionContinueNudgeMessage follows a batch of read-only tools when the model stopped in prose.
	actionContinueNudgeMessage = `[goclaw] The user asked for concrete code improvements (not analysis-only). You already ran read-only tools in this turn. Continue with native tool calls: use edit_file, write_file, or patch for real edits, then bash or script to verify. Do not answer with prose-only until edits are done or truly blocked.`

	// actionFirstTurnNoToolsNudgeMessage applies when the first model completion in this user turn had zero tool calls
	// but the user message signals repo/code work (common with local models that narrate instead of calling tools).
	actionFirstTurnNoToolsNudgeMessage = `[goclaw] The user asked for code or repository changes. Your last completion had no native tool calls — prose alone does not read or edit files. Reply with tool calls only on the next turn: use read_file, glob, or grep first as needed, then edit_file, write_file, or patch to apply changes, then bash or script to verify. Do not simulate tools in markdown.`

	// actionRepairModelEscalationMessage follows exhausted action-continue nudges when action_repair_escalation is on:
	// the runtime switches to the configured coding-tier model for one more attempt.
	actionRepairModelEscalationMessage = `[goclaw] Action nudges were exhausted without native tool calls. This turn now uses the configured coding model — reply with tool calls only: read_file/glob/grep as needed, then edit_file/write_file/patch, then bash or script to verify. Do not simulate tools in markdown.`

	// editFileNotFoundNudgeMessage is injected when the most recent tool batch included
	// an edit_file call that failed with "old_string not found", signalling the model
	// hallucinated the string instead of reading the file first.
	editFileNotFoundNudgeMessage = `[goclaw] edit_file failed: old_string not found. ` +
		`You must read the file first (read_file), find the exact text in the output, ` +
		`then call edit_file with that exact string. Do not guess or re-invent the string.`
)

func (o *Orchestrator) effectiveMaxActionNudges() int {
	if o == nil {
		return defaultActionNudgesPerUserTurn
	}
	n := o.cfg.AutoContinueActionMaxNudges
	if n <= 0 {
		return defaultActionNudgesPerUserTurn
	}
	if n > maxActionNudgesCap {
		return maxActionNudgesCap
	}
	return n
}

// workspaceWriteIntentKeywords are lowercase substrings: if any appears in the user
// message (after explain-only veto), we treat the turn as requesting workspace writes.
var workspaceWriteIntentKeywords = []string{
	"apply changes",
	"arregla",
	"arreglar",
	"audit and",
	"audit ",
	"auditoría",
	"auditoria",
	"clean up",
	"cleanup",
	"code smell",
	"deuda técnica",
	"deuda tecnica",
	"diagnos",
	"find and fix",
	"find gaps",
	"find issues",
	"fix ",
	"fixes",
	"fixed",
	"gaps ",
	"implement",
	"improve",
	"improving",
	"inconsisten",
	"iteración",
	"iteracion",
	"mejorar",
	"patch ",
	"refactor",
	"refactoring",
	"refactorización",
	"refactorizacion",
	"review and fix",
	"sin romper",
	"smell",
	"write_file",
	"edit_file",

	// Common imperative phrasing (space-prefixed where it reduces false positives).
	" add ",
	" create ",
	" update ",
	" remove ",
	" delete ",
	"apply the",
	"complete the",
	"finish the",
	"finish implementing",

	"añade ",
	"crea ",
	"crear ",   // infinitive form: "crear un proyecto" — "crea " alone misses this
	"crear un", // "crear un proyecto de X"
	"actualiza ",
	"elimina ",
	"borra ",
	"aplica los",
	"aplica el",
	"completa la implementación",
	"termina de implementar",
	"construir",
	"construye",
	"hazme un",
	"hazme el",
	"hazme la",

	// Spanish (and short phrases) common in operator chat.
	" no funciona",
	" corrige ",
	" corregir ",
	" escribe el código",
	" escribe el codigo",
	" hazlo",
	" reparar",
	" soluciona",
}

// userMessageWantsWorkspaceWrites is a conservative heuristic for when the original user
// message asked for code changes, not analysis-only. Used to decide auto-continue nudges.
func userMessageWantsWorkspaceWrites(userMessage string) bool {
	low := strings.ToLower(strings.TrimSpace(userMessage))
	if low == "" {
		return false
	}
	if looksPureExplainOnly(low) {
		return false
	}
	pad := " " + low + " "
	for _, kw := range workspaceWriteIntentKeywords {
		if len(kw) >= 2 && kw[0] == ' ' && kw[len(kw)-1] == ' ' {
			if strings.Contains(pad, kw) {
				return true
			}
			continue
		}
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}

func looksPureExplainOnly(low string) bool {
	if strings.Contains(low, "explain how") || strings.Contains(low, "what does ") ||
		strings.Contains(low, "how does ") || strings.Contains(low, "describe the structure") {
		if strings.Contains(low, "fix") || strings.Contains(low, "refactor") || strings.Contains(low, "mejorar") {
			return false
		}
		return true
	}
	// "explain … delete/remove" is usually conceptual, not a request to modify the repo.
	if strings.Contains(low, "explain") && (strings.Contains(low, "delete") || strings.Contains(low, "remove")) {
		if strings.Contains(low, "fix") {
			return false
		}
		pad := " " + low + " "
		if strings.Contains(pad, " implement ") || strings.Contains(pad, " implementar ") || strings.Contains(pad, " apply ") {
			return false
		}
		return true
	}
	return false
}

func toolSpecsAllowWorkspaceWrite(specs []tools.ToolSpec) bool {
	for _, s := range specs {
		switch s.Name {
		case "write_file", "write_files", "edit_file", "patch":
			return true
		}
	}
	return false
}

func toolUsesIncludeWorkspaceWrite(uses []llm.ToolUse) bool {
	for _, u := range uses {
		switch u.Name {
		case "write_file", "write_files", "edit_file", "patch":
			return true
		}
	}
	return false
}

// toolResultsHaveEditNotFound reports whether any tool result in the batch
// is an edit_file failure due to old_string not found.
func toolResultsHaveEditNotFound(results []llm.ToolResultRecord) bool {
	for _, r := range results {
		if r.ToolName == "edit_file" && r.IsError &&
			strings.Contains(r.Content, "old_string not found") {
			return true
		}
	}
	return false
}

// responseAppearsComplete reports whether the assistant's response text looks like a finished
// answer rather than an incomplete stub. Used to avoid spurious action-continue nudges after
// genuinely complete replies (e.g. "no changes needed", "done.").
func responseAppearsComplete(response string) bool {
	if len(response) < 40 {
		return false
	}
	lower := strings.ToLower(response)
	completionPhrases := []string{
		"in summary", "to summarize", "in conclusion",
		"the changes are complete", "changes are done",
		"done.", "completed.", "all done", "finished.",
		"no issues found", "nothing to fix", "no changes needed",
		"no changes required", "no problems found", "looks good",
		"everything is ", "task complete", "task is complete",
	}
	for _, phrase := range completionPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// pickActionContinueNudge returns a synthetic user line to re-prompt the model when it answered
// with prose but the user turn clearly asked for code/repo changes. Covers (1) first completion
// with zero tool calls — typical with small local models — and (2) after read-only tools only.
// response is the assistant's last text output; when it appears complete the nudge is suppressed.
func (o *Orchestrator) pickActionContinueNudge(
	userMessage string,
	toolCalls int,
	lastBatchReadOnly bool,
	hadToolRound bool,
	actionNudges int,
	response string,
) (message string, ok bool) {
	if o == nil || !o.cfg.AutoContinueActionRequests {
		return "", false
	}
	if actionNudges >= o.effectiveMaxActionNudges() {
		return "", false
	}
	if o.profile.ReadOnly {
		return "", false
	}
	if !toolSpecsAllowWorkspaceWrite(o.effectiveToolSpecs()) {
		return "", false
	}
	if !userMessageWantsWorkspaceWrites(userMessage) {
		return "", false
	}
	// If the response looks genuinely complete, don't nudge — the model is done.
	if responseAppearsComplete(response) {
		return "", false
	}
	if hadToolRound && lastBatchReadOnly && toolCalls > 0 {
		return actionContinueNudgeMessage, true
	}
	if !hadToolRound && toolCalls == 0 {
		return actionFirstTurnNoToolsNudgeMessage, true
	}
	return "", false
}
