package orchestrator

import (
	"strings"

	"github.com/okuzpe/goclaw/internal/llm"
)

const noWorkspaceWriteTruthFooter = "\n\n---\n[goclaw] No workspace files were modified this turn (no successful write_file / edit_file / patch).\n[goclaw] No se modificó ningún archivo del proyecto en este turno (no hubo write_file / edit_file / patch con éxito)."

// maybeAppendNoWorkspaceWriteFooter appends a bilingual runtime footer when the user message
// signals code changes, tools ran, writes were available, but no successful workspace write completed.
func maybeAppendNoWorkspaceWriteFooter(
	o *Orchestrator,
	response, userMessage string,
	hadToolRound, workspaceWriteOK bool,
) string {
	if o == nil || !o.cfg.TruthFooterNoWorkspaceWrites {
		return response
	}
	if o.profile.ReadOnly || !hadToolRound || workspaceWriteOK {
		return response
	}
	if !userMessageWantsWorkspaceWrites(userMessage) {
		return response
	}
	if !toolSpecsAllowWorkspaceWrite(o.effectiveToolSpecs()) {
		return response
	}
	if strings.Contains(response, "No workspace files were modified this turn") {
		return response
	}
	return response + noWorkspaceWriteTruthFooter
}

func recordWorkspaceWriteFromResults(workspaceWriteOK *bool, results []llm.ToolResultRecord) {
	for _, r := range results {
		if r.IsError {
			continue
		}
		switch r.ToolName {
		case "write_file", "edit_file", "patch":
			*workspaceWriteOK = true
			return
		}
	}
}
