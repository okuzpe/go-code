package app

import (
	"log/slog"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/orchestrator"
)

// RunStreaming entry points that run coordinator auto-profile (same behavior as the TUI):
//   - cmd/goclaw: chat PreSubmit + submitter → RunStreaming (see internal/ui/chat.RunApp)
//   - internal/app/json_output_run.go (RunStreaming / RunStreamingToolTrace)
//   - internal/app/telegram_bridge.go (RunStreaming)
// Nested workers use internal/coordinator/spawn_agent.go and internal/coordinator/interactive.go — do not
// apply parent-session auto-profile there.
// Per-agent MemoryScope is chosen at PrepareChatRuntime; auto-switch only promotes to built-in
// general-purpose or builder (empty MemoryScope). Custom-agent targets with non-empty memory scope are skipped.
// After auto-elevation, the session stays on the direct profile until the user runs /profile coordinator
// (no automatic revert at end of turn).

const autoProfileSwitchNoticePrefix = "[goclaw] "

// MaybeCoordinatorToDirectProfile switches coordinator to general-purpose or builder when settings
// and heuristics indicate single-session coding. Returns transcript lines (English) when a switch happened.
// Profile stays elevated until the user runs /profile coordinator (no auto-revert after each turn).
func MaybeCoordinatorToDirectProfile(rt *ChatRuntime, orch *orchestrator.Orchestrator, userMessage string, workerFocusActive bool) []string {
	if rt == nil || orch == nil {
		return nil
	}
	if rt.DisableTools || rt.Mock {
		return nil
	}
	if workerFocusActive {
		return nil
	}
	if rt.ExplicitAgentProfileFromCLI {
		return nil
	}
	targetKey := config.NormalizeAutoDirectCodingProfile(rt.Cfg.AutoDirectCodingProfile)
	if targetKey == "off" {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(orch.ProfileName()), "coordinator") {
		return nil
	}
	if !shouldAutoElevateFromCoordinator(rt.Cfg, userMessage) {
		return nil
	}
	next, ok := rt.Profs[targetKey]
	if !ok {
		next, ok = rt.Profs[strings.ToLower(targetKey)]
	}
	if !ok || next.MemoryScope != "" {
		// Built-in general-purpose and builder use the global memory store (empty MemoryScope).
		return nil
	}
	prevName := orch.ProfileName()
	if strings.EqualFold(prevName, next.Name) {
		return nil
	}
	orch.SetProfile(next)
	rt.Profile = next
	slog.Info("auto profile switch", "from", prevName, "to", next.Name)
	line := autoProfileSwitchNoticePrefix + "Switched session profile from " + prevName + " to " + next.Name + " for direct coding tools (use /profile coordinator to return to hub mode)."
	return []string{line}
}

func shouldAutoElevateFromCoordinator(cfg config.Config, userMessage string) bool {
	switch config.NormalizeAutoProfileIntent(cfg.AutoProfileIntent) {
	case "rules":
		res := orchestrator.ClassifyProfileIntentRules(userMessage)
		return res.Intent == orchestrator.ProfileIntentDirectCode
	default:
		return orchestrator.FusedDirectCodeIntent(userMessage)
	}
}
