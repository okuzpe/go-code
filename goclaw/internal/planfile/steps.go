package planfile

import (
	"regexp"
	"strings"
)

const maxParsedPlanSteps = 64

var (
	reNumberedStep = regexp.MustCompile(`^\s*(\d+)\.\s+(.+)$`)
	reBulletStep   = regexp.MustCompile(`^\s*[-*]\s+(.+)$`)
)

// ParseImplementationSteps extracts ordered step lines from a Markdown plan.
//
// Convention: after a heading containing "Steps" (e.g. "## Steps"), collect lines until the next
// Markdown heading "## ..." or "### ..." (or end of file). Accepted step lines:
//   - Numbered: "1. Do something"
//   - Bullets: "- Do something" or "* Do something"
//
// If no Steps section is found, returns nil (caller may treat as unstructured plan).
func ParseImplementationSteps(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	inSteps := false
	var out []string
	for _, ln := range lines {
		t := strings.TrimRight(ln, "\r")
		trim := strings.TrimSpace(t)
		if strings.HasPrefix(trim, "##") {
			low := strings.ToLower(trim)
			if strings.Contains(low, "steps") {
				inSteps = true
				continue
			}
			if inSteps {
				break
			}
			continue
		}
		if strings.HasPrefix(trim, "###") && inSteps {
			low := strings.ToLower(trim)
			if !strings.Contains(low, "steps") {
				break
			}
			continue
		}
		if !inSteps {
			continue
		}
		if trim == "" || strings.HasPrefix(trim, "```") {
			continue
		}
		if m := reNumberedStep.FindStringSubmatch(t); len(m) >= 3 {
			s := strings.TrimSpace(m[2])
			if s != "" {
				out = append(out, s)
			}
			continue
		}
		if m := reBulletStep.FindStringSubmatch(t); len(m) >= 2 {
			s := strings.TrimSpace(m[1])
			if s != "" {
				out = append(out, s)
			}
		}
		if len(out) >= maxParsedPlanSteps {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
