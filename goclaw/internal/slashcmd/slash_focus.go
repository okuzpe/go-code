package slashcmd

import (
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/coordinator"
)

func handleSlashWorkers() (handled bool, out string, quit bool, modelSubmit string, err error) {
	list := coordinator.ListInteractiveWorkers()
	if len(list) == 0 {
		return true, "(no interactive workers — spawn with spawn_agent interactive: true)", false, "", nil
	}
	var b strings.Builder
	for _, w := range list {
		b.WriteString(w.TaskID)
		b.WriteString("  ")
		b.WriteString(w.Profile)
		b.WriteString("  ")
		b.WriteString(w.Status)
		if strings.TrimSpace(w.Summary) != "" {
			b.WriteString("  — ")
			b.WriteString(previewRunes(w.Summary, 72))
		}
		b.WriteByte('\n')
	}
	return true, strings.TrimSuffix(b.String(), "\n"), false, "", nil
}

func handleSlashDetach(env SlashEnv) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireFocusRouter("detach", env); err != nil {
		return true, "", false, "", err
	}
	env.Focus.Detach()
	return true, "focus: coordinator (parent session)", false, "", nil
}

func handleSlashFocus(env SlashEnv, fields []string) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if err := requireFocusRouter("focus", env); err != nil {
		return true, "", false, "", err
	}
	if len(fields) < 2 {
		return true, "", false, "", fmt.Errorf(`usage: /focus <task_id_prefix> | /focus parent   (alias: /in)
parent — return to coordinator (same as /back or /detach)
use /workers to list interactive worker ids`)
	}
	arg := strings.TrimSpace(strings.ToLower(fields[1]))
	if arg == "parent" || arg == ".." || arg == "coordinator" {
		env.Focus.Detach()
		return true, "focus: coordinator (parent session)", false, "", nil
	}
	prefix := strings.TrimSpace(fields[1])
	full, ok := coordinator.ResolveInteractiveTaskID(prefix)
	if !ok {
		return true, "", false, "", slashNextStepError(fmt.Sprintf("no unique interactive worker matches prefix %q", prefix), "run /workers and choose a longer prefix")
	}
	env.Focus.FocusTaskID(full)
	out = fmt.Sprintf("focus: worker %s (input goes here until /back or /detach)", full)
	if snap, ok := coordinator.SnapshotInteractiveWorker(full); ok && strings.TrimSpace(snap) != "" {
		out = "[worker history]\n" + snap + "\n" + out
	}
	return true, out, false, "", nil
}
