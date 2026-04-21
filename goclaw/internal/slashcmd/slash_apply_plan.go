package slashcmd

import (
	"fmt"
	"path/filepath"
	"strings"
)

const planPreviewMaxRunes = 4000

// parseApplyPlanRest parses tokens after `/apply-plan`. Supports `--preview` (or `-preview`);
// `--hub` selects coordinator execution; `--steps` queues one model turn per parsed ## Steps line;
// `--yes` / `-y` are accepted and ignored (execution is the default when not previewing).
// Remaining non-flag tokens are joined with a single space as an optional plan path.
func parseApplyPlanRest(rest string) (pathArg string, preview, hub, allSteps bool) {
	parts := strings.Fields(rest)
	var pathParts []string
	for _, p := range parts {
		switch strings.ToLower(p) {
		case "--preview", "-preview":
			preview = true
		case "--hub":
			hub = true
		case "--steps":
			allSteps = true
		case "--yes", "-y":
			// no-op: explicit confirm style; same as default execute path
		default:
			if strings.HasPrefix(p, "-") {
				continue
			}
			pathParts = append(pathParts, p)
		}
	}
	return strings.Join(pathParts, " "), preview, hub, allSteps
}

// formatPlanPreviewOutput builds markdown for /apply-plan --preview and plan review excerpts.
func formatPlanPreviewOutput(planPath, body string) string {
	display := filepath.ToSlash(planPath)
	excerpt := truncateRunesPlanPreview(body, planPreviewMaxRunes)
	var b strings.Builder
	b.WriteString("## Plan preview\n\n")
	b.WriteString("_Not sent to the model yet._\n\n")
	b.WriteString("- **File:** `")
	b.WriteString(display)
	b.WriteString(fmt.Sprintf("`\n- **Size:** %d bytes\n\n", len(body)))
	b.WriteString(MarkdownFencedPlain(excerpt))
	b.WriteString("\n\nRun `/apply-plan` or `/plan run` to execute (general-purpose or coordinator with `--hub` / `plan_apply_use_coordinator`; one model turn by default; add `--steps` when the plan has a `## Steps` section for one turn per step; `/plan run` saves the latest assistant message first).\n")
	b.WriteString("\nOptional: `/plan review`, `/plan approve` (when `plan_require_apply_approval` is true), `/plan steps`.")
	return b.String()
}

// parsePlanRunFields parses tokens after `/plan run` (or `/plan apply`): optional `--hub`, optional `--steps`, then optional plan path.
func parsePlanRunFields(fields []string) (hub bool, allSteps bool, pathTail string) {
	if len(fields) < 3 {
		return false, false, ""
	}
	var pathParts []string
	for _, f := range fields[2:] {
		t := strings.TrimSpace(f)
		if t == "" {
			continue
		}
		if strings.EqualFold(t, "--hub") {
			hub = true
			continue
		}
		if strings.EqualFold(t, "--steps") {
			allSteps = true
			continue
		}
		if strings.HasPrefix(t, "-") {
			continue
		}
		pathParts = append(pathParts, t)
	}
	return hub, allSteps, strings.Join(pathParts, " ")
}

func truncateRunesPlanPreview(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	n := 0
	for i := range s {
		if n >= maxRunes {
			return strings.TrimSpace(s[:i]) + "\n… (truncated for preview)"
		}
		n++
	}
	return s
}
