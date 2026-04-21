package config

import "strings"

// TUI interact mode (footer label; Ctrl+M cycles in the fullscreen TUI). When wired to the
// orchestrator, it adjusts system prompt hints and the chat-mode iteration cap — not agent_profile.
const (
	TUIInteractModeChat  = "chat"
	TUIInteractModeCode  = "code"
	TUIInteractModeAgent = "agent"
)

// NormalizeTUIInteractMode returns chat | code | agent; unknown or empty → chat.
func NormalizeTUIInteractMode(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case TUIInteractModeChat, TUIInteractModeCode, TUIInteractModeAgent:
		return v
	default:
		return TUIInteractModeChat
	}
}

// CycleTUIInteractMode advances chat → code → agent → chat.
func CycleTUIInteractMode(cur string) string {
	switch NormalizeTUIInteractMode(cur) {
	case TUIInteractModeChat:
		return TUIInteractModeCode
	case TUIInteractModeCode:
		return TUIInteractModeAgent
	default:
		return TUIInteractModeChat
	}
}

// TUIFooterDensity controls how much session metadata appears in the fullscreen chat idle footer
// (right side next to workspace; see cmd/goclaw tuiFooterStats). JSON: tui_footer_density.
const (
	TUIFooterDensityMinimal  = "minimal"
	TUIFooterDensityStandard = "standard"
	TUIFooterDensityDebug    = "debug"
)

// NormalizeTUIFooterDensity returns a canonical density; empty or unknown defaults to standard.
func NormalizeTUIFooterDensity(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case TUIFooterDensityMinimal, TUIFooterDensityStandard, TUIFooterDensityDebug:
		return v
	case "":
		return TUIFooterDensityStandard
	default:
		return TUIFooterDensityStandard
	}
}
