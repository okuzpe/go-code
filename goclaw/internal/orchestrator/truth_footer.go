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

// sanitizeNarratedToolCallText trims model-emitted fake tool-call narration in plain text responses.
// Some models emit "TOOL CALL```" blocks even when there were no native tool calls in the stream.
// We remove that trailing narrated section to keep transcript UX clean and avoid misleading output.
func sanitizeNarratedToolCallText(response string) string {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(response)
	cut := len(response)
	for _, marker := range []string{
		"tool call```",
		"tool call:",
		"[assistant tool_use ",
		"<function_calls>",
	} {
		if idx := strings.Index(lower, marker); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	if cut < len(response) {
		return strings.TrimSpace(response[:cut])
	}
	return strings.TrimSpace(response)
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
