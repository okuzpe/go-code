package orchestrator

import (
	"strings"

	"github.com/okuzpe/goclaw/internal/llm"
)

// TruthFooterMarkerEN and TruthFooterMarkerES are stable substrings of noWorkspaceWriteTruthFooter
// (used by the TUI to detect the runtime footer without duplicating copy).
const (
	TruthFooterMarkerEN = "No workspace files were modified this turn"
	TruthFooterMarkerES = "No se modificó ningún archivo del proyecto en este turno"
)

const noWorkspaceWriteTruthFooter = "\n\n---\n[goclaw] " + TruthFooterMarkerEN + " (no successful write_file / edit_file / patch).\n[goclaw] " + TruthFooterMarkerES + " (no hubo write_file / edit_file / patch con éxito)."

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
	if strings.Contains(response, TruthFooterMarkerEN) {
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
