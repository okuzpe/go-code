package orchestrator

import "github.com/okuzpe/goclaw/internal/config"

// tuiInteractModePromptBlock returns an extra system section for the fullscreen TUI interact
// mode (chat | code | agent). Empty when the mode is not active for this turn.
func tuiInteractModePromptBlock(active bool, mode string) string {
	if !active {
		return ""
	}
	switch config.NormalizeTUIInteractMode(mode) {
	case config.TUIInteractModeChat:
		return "\n\n## Terminal interact mode: chat\n" +
			"The user selected **chat** mode. Prefer concise answers. Use file or shell tools only when the question clearly needs repository facts or commands; otherwise answer directly without claiming you read files you did not read."
	case config.TUIInteractModeCode:
		return "\n\n## Terminal interact mode: code\n" +
			"The user selected **code** mode. Prefer minimal, scoped edits and short explanations. When changing the repo, use tools (read → edit → verify) rather than dumping large unapplied code blocks."
	case config.TUIInteractModeAgent:
		return "\n\n## Terminal interact mode: agent\n" +
			"The user selected **agent** mode. Drive the task to completion within the iteration budget: discover with read-only tools, apply the smallest viable edits, verify with bash, run_command, run_tests, or script, and finish with a brief factual summary."
	default:
		return ""
	}
}
