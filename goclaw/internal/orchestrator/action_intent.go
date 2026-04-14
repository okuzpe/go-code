package orchestrator

import (
	"strings"

	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/tools"
)

const (
	maxActionNudgesPerUserTurn = 2

	actionContinueNudgeMessage = `[goclaw] The user asked for concrete code improvements (not analysis-only). You already ran read-only tools in this turn. Continue with native tool calls: use edit_file, write_file, or patch for real edits, then bash or script to verify. Do not answer with prose-only until edits are done or truly blocked.`
)

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
	for _, kw := range workspaceWriteIntentKeywords {
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
	return false
}

func toolSpecsAllowWorkspaceWrite(specs []tools.ToolSpec) bool {
	for _, s := range specs {
		switch s.Name {
		case "write_file", "edit_file", "patch":
			return true
		}
	}
	return false
}

func toolUsesIncludeWorkspaceWrite(uses []llm.ToolUse) bool {
	for _, u := range uses {
		switch u.Name {
		case "write_file", "edit_file", "patch":
			return true
		}
	}
	return false
}

func (o *Orchestrator) shouldInjectActionNudge(
	userMessage string,
	toolCalls int,
	lastBatchReadOnly bool,
	hadToolRound bool,
	actionNudges int,
) bool {
	if !o.cfg.AutoContinueActionRequests {
		return false
	}
	if actionNudges >= maxActionNudgesPerUserTurn {
		return false
	}
	if o.profile.ReadOnly || !hadToolRound || !lastBatchReadOnly || toolCalls == 0 {
		return false
	}
	if !toolSpecsAllowWorkspaceWrite(o.effectiveToolSpecs()) {
		return false
	}
	return userMessageWantsWorkspaceWrites(userMessage)
}
