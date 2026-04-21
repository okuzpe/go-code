package planfile

import (
	"fmt"
	"path/filepath"
	"strings"
)

// StepExecutionUserMessages builds one user message per parsed ## Steps line for multi-turn execution.
// Each message includes the full plan body as reference (bounded by Read/MaxBytes upstream).
func StepExecutionUserMessages(planPath, planBody string, steps []string, opts HandoffOptions) []string {
	if len(steps) == 0 {
		return nil
	}
	display := filepath.ToSlash(planPath)
	out := make([]string, 0, len(steps))
	for i, step := range steps {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("[goclaw plan execution %d/%d]\n\n", i+1, len(steps)))
		b.WriteString("Saved plan: ")
		b.WriteString(display)
		b.WriteString("\n\n")
		if opts.UseCoordinator {
			b.WriteString("Coordinator (hub) mode: delegate **only this step** via spawn_agent with a self-contained task (paths, acceptance criteria). Do not claim direct edits in the parent session. ")
		} else {
			b.WriteString("Execute **only** this step now using native tools (read → edit → verify). ")
		}
		b.WriteString("Later steps arrive as separate user messages; do not try to complete the whole plan in one turn.\n\n## Step\n\n")
		b.WriteString(strings.TrimSpace(step))
		b.WriteString("\n\n---\n\n## Full plan (reference only)\n\n")
		b.WriteString(planBody)
		out = append(out, b.String())
	}
	return out
}
