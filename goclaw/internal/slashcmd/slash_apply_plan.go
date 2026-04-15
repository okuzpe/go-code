package slashcmd

import (
	"fmt"
	"path/filepath"
	"strings"
)

const planPreviewMaxRunes = 4000

// parseApplyPlanRest parses tokens after `/apply-plan`. Supports `--preview` (or `-preview`);
// `--hub` selects coordinator execution; `--yes` / `-y` are accepted and ignored (execution is the default when not previewing).
// Remaining non-flag tokens are joined with a single space as an optional plan path.
func parseApplyPlanRest(rest string) (pathArg string, preview, hub bool) {
	parts := strings.Fields(rest)
	var pathParts []string
	for _, p := range parts {
		switch strings.ToLower(p) {
		case "--preview", "-preview":
			preview = true
		case "--hub":
			hub = true
		case "--yes", "-y":
			// no-op: explicit confirm style; same as default execute path
		default:
			if strings.HasPrefix(p, "-") {
				continue
			}
			pathParts = append(pathParts, p)
		}
	}
	return strings.Join(pathParts, " "), preview, hub
}

// formatPlanPreviewOutput builds human-readable output for /apply-plan --preview.
func formatPlanPreviewOutput(planPath, body string) string {
	display := filepath.ToSlash(planPath)
	excerpt := truncateRunesPlanPreview(body, planPreviewMaxRunes)
	var b strings.Builder
	b.WriteString("Plan preview (not sent to the model yet)\n")
	b.WriteString("File: ")
	b.WriteString(display)
	b.WriteString(fmt.Sprintf("\nSize: %d bytes\n\n", len(body)))
	b.WriteString(excerpt)
	b.WriteString("\n\nRun /apply-plan or /plan run to execute (general-purpose or coordinator with --hub / plan_apply_use_coordinator; one model turn; /plan run saves the latest assistant message first).")
	b.WriteString("\nOptional: /plan review, /plan approve (when plan_require_apply_approval is true), /plan steps.")
	return b.String()
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
