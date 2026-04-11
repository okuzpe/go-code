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
	p, ok := profs[key]
	if !ok {
		return "", fmt.Errorf("unknown agent %q; valid: %s", rawName, sortedProfileNames(profs))
	}
	orch.SetProfile(p)
	return fmt.Sprintf("active profile: %s", p.Name), nil
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

func tryInteractiveAgentsPick(env SlashEnv, orch *orchestrator.Orchestrator) (out string, used bool, err error) {
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
		return msg, true, nil
	default:
		return "", false, nil
	}
}
