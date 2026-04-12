package slashcmd

import (
	"fmt"
	"os"
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
	return formatProfileSwitchSummary(prev, next), nil
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

func formatAgentsList(profs map[string]agents.Profile, active string) string {
	active = strings.TrimSpace(active)
	var b strings.Builder
	b.WriteString("Available agents (built-in + custom *.md under agents dirs):\n\n")
	for _, name := range agents.SortedKeys(profs) {
		p := profs[name]
		prefix := "  "
		if name == active {
			prefix = "* "
		}
		b.WriteString(prefix)
		b.WriteString(name)
		b.WriteString(" — ")
		b.WriteString(p.Summary())
		b.WriteByte('\n')
	}
	b.WriteString("\nUsage: /agents <name>  (same as /profile <name>)\n")
	b.WriteString("Bare /agents in readline uses arrow keys when stdin is a TTY.\n")
	return b.String()
}

func tryInteractiveAgentsPick(env SlashEnv, orch *orchestrator.Orchestrator, hintsOut *UIHints) (out string, used bool, err error) {
	if env.DisableInteractiveAgentPick || orch == nil {
		return "", false, nil
	}
	fd := int(os.Stdin.Fd())
	profs, _ := agents.AllWithCustom(env.UserAgentsDir, env.ProjectAgentsDir)
	names := agents.SortedKeys(profs)
	if len(names) == 0 {
		return "", false, nil
	}
	cur := orch.ProfileName()
	start := 0
	for i, n := range names {
		if n == cur {
			start = i
			break
		}
	}
	choice, res, perr := pickListTTY(fd, os.Stdin, os.Stdout, "Agent profile — ↑↓ move · Enter apply · Esc cancel", names, start)
	if perr != nil {
		return "", false, nil
	}
	switch res {
	case ttyListPickNone:
		return "", false, nil
	case ttyListPickCancelled:
		return "/agents cancelled.", true, nil
	case ttyListPickChosen:
		msg, serr := switchOrchestratorProfile(orch, env, choice)
		if serr != nil {
			return "", true, serr
		}
		sub := ""
		if env.ChatSubtitle != nil {
			sub = env.ChatSubtitle()
		}
		setWelcomeHints(hintsOut, orch, sub)
		return msg, true, nil
	default:
		return "", false, nil
	}
}
