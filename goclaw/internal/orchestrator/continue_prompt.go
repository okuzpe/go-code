package orchestrator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/okuzpe/goclaw/internal/llm"
)

const (
	genericContinueFollowUpPrompt = `Continue working on the prior request: use tools to complete pending edits (write_file, edit_file, patch) or bash/script to verify, or state clearly what blocks you.`
	maxContinueChangedPaths       = 6
)

type continueState struct {
	verifyPending           bool
	requiredVerifyKind      string
	requiredVerifySig       string
	requiredVerifyLabel     string
	editFileRecoveryPending bool
	pathRecoveryPending     bool
	pathRecoveryTool        string
	pathRecoveryTarget      string
	changedPaths            map[string]bool
}

// ContinueFollowUpPrompt reconstructs the latest unfinished runtime state for the most recent
// real user request in this session and returns a follow-up prompt suitable for /continue.
func ContinueFollowUpPrompt(msgs []llm.Message) string {
	state := continueStateFromMessages(msgs)
	if !state.verifyPending && !state.editFileRecoveryPending && !state.pathRecoveryPending {
		return genericContinueFollowUpPrompt
	}

	var b strings.Builder
	b.WriteString("Continue working on the prior request. Use tools to finish the pending work from this same request.")
	if state.editFileRecoveryPending {
		b.WriteString("\n- edit_file recovery is still pending: read_file the target first, copy the exact current text, then retry edit_file with the real old_string.")
	}
	if state.pathRecoveryPending {
		b.WriteString("\n- path recovery is still pending")
		if strings.TrimSpace(state.pathRecoveryTool) != "" {
			b.WriteString(" for `")
			b.WriteString(state.pathRecoveryTool)
			b.WriteString("`")
		}
		if strings.TrimSpace(state.pathRecoveryTarget) != "" {
			b.WriteString(" on `")
			b.WriteString(state.pathRecoveryTarget)
			b.WriteString("`")
		}
		b.WriteString(": use glob or grep to rediscover the real path, then retry with the validated target instead of guessing.")
	}
	if state.verifyPending {
		if strings.TrimSpace(state.requiredVerifyLabel) != "" {
			b.WriteString("\n- Re-run the last failed verification before ending: `")
			b.WriteString(state.requiredVerifyLabel)
			b.WriteString("`.")
		} else {
			b.WriteString("\n- Verification is still pending before you end the turn.")
		}
		if paths := sortedChangedPaths(state.changedPaths, maxContinueChangedPaths); len(paths) > 0 {
			b.WriteString("\n- Files with pending verification: ")
			b.WriteString(strings.Join(paths, ", "))
			if len(state.changedPaths) > len(paths) {
				fmt.Fprintf(&b, " (+%d more)", len(state.changedPaths)-len(paths))
			}
			b.WriteString(".")
		}
	}
	b.WriteString("\n- If something is truly blocked, say the blocker in one sentence after the next tool attempt.")
	return b.String()
}

func continueStateFromMessages(msgs []llm.Message) continueState {
	start := lastNaturalUserMessageIndex(msgs)
	if start < 0 || start >= len(msgs) {
		return continueState{}
	}
	state := continueState{
		changedPaths: make(map[string]bool),
	}
	var lastToolCalls []llm.ToolCallRecord
	for _, msg := range msgs[start+1:] {
		if strings.EqualFold(msg.Role, "assistant") && len(msg.ToolCalls) > 0 {
			lastToolCalls = append([]llm.ToolCallRecord(nil), msg.ToolCalls...)
			continue
		}
		if !strings.EqualFold(msg.Role, "user") || len(msg.ToolResults) == 0 {
			continue
		}
		for idx, result := range msg.ToolResults {
			toolUse := matchedToolUse(lastToolCalls, idx, result)
			if toolResultIsSuccessfulWorkspaceWrite(result) {
				state.verifyPending = true
				state.editFileRecoveryPending = false
				state.pathRecoveryPending = false
				state.pathRecoveryTool = ""
				state.pathRecoveryTarget = ""
				for _, path := range changedPathsFromToolUse(toolUse) {
					if strings.TrimSpace(path) != "" {
						state.changedPaths[path] = true
					}
				}
			}
			if toolResultClearsPathRecovery(result) {
				state.pathRecoveryPending = false
				state.pathRecoveryTool = ""
				state.pathRecoveryTarget = ""
			}
			if result.ToolName == "edit_file" && result.IsError && strings.Contains(result.Content, "old_string not found") {
				state.editFileRecoveryPending = true
			}
			if result.IsError {
				if toolName, target, ok := pathRecoveryFailureDetails(toolUse, result); ok {
					state.pathRecoveryPending = true
					state.pathRecoveryTool = toolName
					state.pathRecoveryTarget = target
				}
			}
			if !toolResultLooksLikeVerify(result) {
				continue
			}
			kind, sig, label, ok := verifyRequirementForToolUse(toolUse, result.ToolName)
			if !ok {
				continue
			}
			if result.IsError {
				state.requiredVerifyKind = kind
				state.requiredVerifySig = sig
				state.requiredVerifyLabel = label
				continue
			}
			if !state.verifyPending {
				continue
			}
			if state.requiredVerifyKind != "" &&
				(state.requiredVerifyKind != kind || state.requiredVerifySig != sig) {
				continue
			}
			state.verifyPending = false
			state.requiredVerifyKind = ""
			state.requiredVerifySig = ""
			state.requiredVerifyLabel = ""
			for path := range state.changedPaths {
				delete(state.changedPaths, path)
			}
		}
		lastToolCalls = nil
	}
	return state
}

func matchedToolUse(toolCalls []llm.ToolCallRecord, idx int, result llm.ToolResultRecord) llm.ToolUse {
	if result.ToolUseID != "" {
		for _, tc := range toolCalls {
			if tc.ID == result.ToolUseID {
				return llm.ToolUse{ID: tc.ID, Name: tc.Name, Input: tc.Input}
			}
		}
	}
	if idx >= 0 && idx < len(toolCalls) {
		tc := toolCalls[idx]
		return llm.ToolUse{ID: tc.ID, Name: tc.Name, Input: tc.Input}
	}
	if strings.TrimSpace(result.ToolName) != "" {
		for _, tc := range toolCalls {
			if tc.Name == result.ToolName {
				return llm.ToolUse{ID: tc.ID, Name: tc.Name, Input: tc.Input}
			}
		}
		return llm.ToolUse{Name: result.ToolName}
	}
	return llm.ToolUse{}
}

func toolResultLooksLikeVerify(result llm.ToolResultRecord) bool {
	return strings.TrimSpace(result.ToolName) == "run_tests" ||
		strings.TrimSpace(result.ToolName) == "bash" ||
		strings.TrimSpace(result.ToolName) == "run_command" ||
		strings.TrimSpace(result.ToolName) == "script"
}

func toolResultClearsPathRecovery(result llm.ToolResultRecord) bool {
	if result.IsError {
		return false
	}
	switch strings.TrimSpace(result.ToolName) {
	case "read_file", "write_file", "write_files", "edit_file", "patch":
		return true
	case "glob", "grep":
		return toolResultHasUsefulPathDiscovery(result.Content)
	default:
		return false
	}
}

func toolResultHasUsefulPathDiscovery(content string) bool {
	trimmed := strings.TrimSpace(content)
	return trimmed != "" && trimmed != "(no matches)"
}

func sortedChangedPaths(paths map[string]bool, max int) []string {
	if len(paths) == 0 || max <= 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for path := range paths {
		if strings.TrimSpace(path) != "" {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	if len(out) > max {
		out = out[:max]
	}
	return out
}

func lastNaturalUserMessageIndex(msgs []llm.Message) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if !strings.EqualFold(msg.Role, "user") {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		if len(msg.ToolResults) > 0 {
			continue
		}
		if isSyntheticRuntimeNudgeContent(msg.Content) {
			continue
		}
		return i
	}
	return -1
}
