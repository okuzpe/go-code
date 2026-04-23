package planfile

import (
	"fmt"
	"path/filepath"
	"strings"
)

// planBodyReferenceMaxLines is the max lines of plan body included in each step message.
// Keeping it short preserves context budget on small local models; the full file is on disk.
const planBodyReferenceMaxLines = 30

// StepExecutionUserMessages builds one user message per parsed ## Steps line for multi-turn execution.
// When there are multiple steps the plan body reference is capped at planBodyReferenceMaxLines to
// protect context budget on small local models — the model already has the step list inline.
func StepExecutionUserMessages(planPath, planBody string, steps []string, opts HandoffOptions) []string {
	if len(steps) == 0 {
		return nil
	}
	display := filepath.ToSlash(planPath)

	// Cap the reference body when multi-step to avoid re-injecting a large plan on every turn.
	refBody := planBody
	if len(steps) > 1 {
		refBody = truncateLines(planBody, planBodyReferenceMaxLines)
	}

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
		b.WriteString(refBody)
		out = append(out, b.String())
	}
	return out
}

// truncateLines returns at most n lines of s, appending a truncation note when cut.
func truncateLines(s string, n int) string {
	lines := strings.SplitAfter(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "") + fmt.Sprintf("\n…(plan truncated after %d lines — full plan at saved path)", n)
}
