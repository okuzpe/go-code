package slashcmd

import (
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/orchestrator"
)

// switchOrchestratorProfile sets the orchestrator profile after resolving name against built-in + custom agents.
func switchOrchestratorProfile(orch *orchestrator.Orchestrator, env SlashEnv, rawName string) (string, error) {
	if orch == nil {
		return "", fmt.Errorf("requires a running agent")
	}
	profs, _ := agents.AllWithCustom(env.UserAgentsDir, env.ProjectAgentsDir)
	key := strings.ToLower(strings.TrimSpace(rawName))
	if key == "" {
		return "", fmt.Errorf("empty agent name")
	}
	next, ok := profs[key]
	if !ok {
		return "", fmt.Errorf("unknown agent %q; valid: %s", rawName, sortedProfileNames(profs))
	}
	prev := profs[strings.ToLower(orch.ProfileName())]
	if prev.Name == "" {
		prev = agents.Profile{Name: orch.ProfileName()}
	}
	orch.SetProfile(next)
	if env.OnProfileChange != nil {
		env.OnProfileChange(next)
	}
	summary := formatProfileSwitchSummary(prev, next)
	if tail := profileSwitchFollowUp(next); tail != "" {
		return summary + tail, nil
	}
	return summary, nil
}

// profileSwitchFollowUp appends a short operator hint after /profile and /agents switches
// so read-only or hub profiles are not mistaken for full coding sessions.
func profileSwitchFollowUp(p agents.Profile) string {
	switch strings.ToLower(strings.TrimSpace(p.Name)) {
	case "plan":
		return "\nPlan profile cannot modify the workspace. After the plan in chat: /plan review, /plan approve if your settings require it, then /plan run or /plan save + /apply-plan (--preview, --hub). Execution is one model turn (general-purpose or coordinator)."
	case "explore":
		return "\nRead-only search profile — no write_file, edit_file, or patch. For direct edits: /profile general-purpose or /profile builder."
	case "guide", "statusline":
		return "\nThis profile has no file or shell tools — chat only. For repo edits: /profile general-purpose or /profile builder."
	case "coordinator":
		return "\nCoordinator hub — the parent session has no direct read/write tools; delegate with spawn_agent or use /profile general-purpose for single-agent edits."
	case "code-review":
		return "\ncode-review has no workspace write tools — output is review prose only. To implement fixes: /profile general-purpose or /profile builder."
	case "verification":
		return "\nVerification profile runs checks (read_file, bash, script) — no workspace write tools. To apply code changes: /profile general-purpose or /profile builder."
	default:
		if p.ReadOnly {
			return "\nRead-only profile — use /profile general-purpose or /profile builder for file edits in this session."
		}
		return ""
	}
}

func formatProfileSwitchSummary(prev, next agents.Profile) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("active profile: %s (was %s)\n", next.Name, prev.Name))
	b.WriteString(fmt.Sprintf("  read_only: %v → %v\n", prev.ReadOnly, next.ReadOnly))
	b.WriteString(fmt.Sprintf("  tool_allowlist: %s → %s\n", toolAllowlistSummary(prev.ToolAllowlist), toolAllowlistSummary(next.ToolAllowlist)))
	if strings.TrimSpace(next.Description) != "" {
		if sp := strings.TrimSpace(firstLine(next.SystemPrompt)); sp != "" {
			b.WriteString("  system_prompt (first line): ")
			b.WriteString(sp)
			b.WriteByte('\n')
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func toolAllowlistSummary(list []string) string {
	if len(list) == 0 {
		return "all tools"
	}
	return fmt.Sprintf("%d tools", len(list))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

func visibleProfileMap(env SlashEnv, profs map[string]agents.Profile) map[string]agents.Profile {
	pg := planGateFrom(env)
	if len(pg.AgentPickerHide) == 0 {
		return profs
	}
	hide := make(map[string]struct{}, len(pg.AgentPickerHide))
	for _, h := range pg.AgentPickerHide {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			hide[h] = struct{}{}
		}
	}
	out := make(map[string]agents.Profile)
	for k, v := range profs {
		if _, skip := hide[strings.ToLower(k)]; skip {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return profs
	}
	return out
}

func formatAgentsList(profs map[string]agents.Profile, active string, env SlashEnv) string {
	profs = visibleProfileMap(env, profs)
	active = strings.TrimSpace(active)
	var b strings.Builder
	b.WriteString("## Available agents\n\n")
	b.WriteString("Built-in + custom `*.md` under agents dirs.\n\n")
	for _, name := range agents.SortedKeys(profs) {
		p := profs[name]
		if name == active {
			b.WriteString("- **")
			b.WriteString(name)
			b.WriteString("** (active) — ")
		} else {
			b.WriteString("- `")
			b.WriteString(name)
			b.WriteString("` — ")
		}
		b.WriteString(p.Summary())
		b.WriteByte('\n')
	}
	b.WriteString("\nPrimary flow: `/profile <name>`.\n")
	b.WriteString("Compatibility alias: `/agents <name>`.\n")
	b.WriteString("In the fullscreen TUI, use **Ctrl+P** or bare `/profile` to open the picker.\n")
	if pg := planGateFrom(env); len(pg.AgentPickerHide) > 0 {
		b.WriteString("\nSome profiles are hidden from the picker (`agent_picker_hidden_profiles`); `/profile <name>` still works.\n")
	}
	return strings.TrimSpace(b.String())
}

func tryInteractiveAgentsPick(env SlashEnv, orch *orchestrator.Orchestrator, hintsOut *UIHints) (out string, used bool, err error) {
	if env.DisableInteractiveAgentPick || orch == nil {
		return "", false, nil
	}
	_ = hintsOut
	// Profile picking is handled in the fullscreen TUI (Ctrl+P overlay); bare /agents lists names.
	return "", false, nil
}

func handleSlashProfile(env SlashEnv, orch *orchestrator.Orchestrator, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireRunningAgent("profile", orch); err != nil {
		return true, "", false, "", err
	}
	if len(fields) < 2 {
		profs, _ := agents.AllWithCustom(env.UserAgentsDir, env.ProjectAgentsDir)
		return true, "", false, "", fmt.Errorf("usage: /profile <name>\nnames: %s", agents.JoinSortedProfileKeys(profs))
	}
	msg, err := switchOrchestratorProfile(orch, env, fields[1])
	if err != nil {
		return true, "", false, "", err
	}
	sub := ""
	if env.ChatSubtitle != nil {
		sub = env.ChatSubtitle()
	}
	setWelcomeHints(hintsOut, orch, sub)
	return true, msg, false, "", nil
}

func handleSlashAgents(env SlashEnv, orch *orchestrator.Orchestrator, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireRunningAgent("agents", orch); err != nil {
		return true, "", false, "", err
	}
	profs, _ := agents.AllWithCustom(env.UserAgentsDir, env.ProjectAgentsDir)
	if len(fields) < 2 {
		if out, used, ierr := tryInteractiveAgentsPick(env, orch, hintsOut); ierr != nil {
			return true, "", false, "", ierr
		} else if used {
			return true, out, false, "", nil
		}
		setTUIDocOverlay(hintsOut, "Agents")
		return true, formatAgentsList(profs, orch.ProfileName(), env), false, "", nil
	}
	msg, err := switchOrchestratorProfile(orch, env, fields[1])
	if err != nil {
		return true, "", false, "", err
	}
	sub := ""
	if env.ChatSubtitle != nil {
		sub = env.ChatSubtitle()
	}
	setWelcomeHints(hintsOut, orch, sub)
	return true, msg, false, "", nil
}
