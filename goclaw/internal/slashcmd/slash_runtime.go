package slashcmd

import (
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/permissions"
)

func handleSlashModel(env SlashEnv, orch *orchestrator.Orchestrator, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if env.SetSessionModel == nil || env.SessionModel == nil {
		return true, "", false, "", slashNextStepError("/model is not available in this mode", "use /doctor to inspect provider mode")
	}
	if len(fields) < 2 {
		return true, fmt.Sprintf("current model: %s\nusage: /model <id>", env.SessionModel()), false, "", nil
	}
	id := strings.TrimSpace(strings.Join(fields[1:], " "))
	if id == "" {
		return true, "", false, "", fmt.Errorf("usage: /model <id>")
	}
	if err := env.SetSessionModel(id); err != nil {
		return true, "", false, "", err
	}
	sub := ""
	if env.ChatSubtitle != nil {
		sub = env.ChatSubtitle()
	}
	if orch != nil {
		setWelcomeHints(hintsOut, orch, sub)
	}
	return true, fmt.Sprintf("model set to %q (this session)", id), false, "", nil
}

func handleSlashTools(env SlashEnv, fields []string) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if env.ToolLog == nil {
		return true, "(tool history not available in this mode - use Ctrl+T in the TUI)", false, "", nil
	}
	n := 0
	if len(fields) >= 2 {
		_, _ = fmt.Sscan(fields[1], &n)
	}
	return true, env.ToolLog(n), false, "", nil
}

func handleSlashAllowWrites(orch *orchestrator.Orchestrator, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireRunningAgent("allow-writes", orch); err != nil {
		return true, "", false, "", err
	}
	for _, toolName := range []string{"write_file", "edit_file", "patch"} {
		orch.SetToolPermission(toolName, permissions.ModeAllow)
	}
	hint := "workspace write tools auto-approved for this session (write_file, edit_file, patch)"
	setFooterHint(hintsOut, hint)
	return true, hint, false, "", nil
}
