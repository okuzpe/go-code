package orchestrator

import (
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/toolpolicy"
)

const (
	verifyAfterWriteNudgeMessage = `[goclaw] You made workspace changes (write_file, edit_file, or patch). Before finishing with prose only, run verification using native tool calls: bash, run_command, script, or run_tests (pick what fits the project). Paste command output in the tool result. If verification is impossible, state one sentence why, then stop.`

	maxVerifyAfterWriteNudges = 4
)

func toolResultIsSuccessfulWorkspaceWrite(r llm.ToolResultRecord) bool {
	if r.IsError {
		return false
	}
	switch r.ToolName {
	case "write_file", "edit_file", "patch":
		return true
	default:
		return false
	}
}

// verifyGateProcessToolResults updates verify state from one tool batch (writes then verify tools).
func verifyGateProcessToolResults(cfg config.Config, ut *userTurnState, results []llm.ToolResultRecord, intent string) {
	if ut == nil || !cfg.AgentVerifyAfterWrite || !userMessageWantsWorkspaceWrites(intent) {
		return
	}
	for _, r := range results {
		if toolResultIsSuccessfulWorkspaceWrite(r) {
			ut.verifyPending = true
			ut.verifySatisfied = false
		}
	}
	for _, r := range results {
		if toolpolicy.IsVerifyTool(r.ToolName) && !r.IsError {
			ut.verifySatisfied = true
			ut.verifyPending = false
		}
	}
}

// verifyGateShouldInjectNudge reports whether a synthetic user line should force another LLM iteration.
func verifyGateShouldInjectNudge(cfg config.Config, ut *userTurnState, profileReadOnly bool) bool {
	if ut == nil || !cfg.AgentVerifyAfterWrite || profileReadOnly {
		return false
	}
	if !ut.verifyPending || ut.verifySatisfied {
		return false
	}
	if ut.verifyNudges >= maxVerifyAfterWriteNudges {
		return false
	}
	return true
}

func verifyGateApplyNudge(ut *userTurnState) {
	if ut == nil {
		return
	}
	ut.verifyNudges++
}
