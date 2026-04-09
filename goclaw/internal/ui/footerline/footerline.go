// Package footerline builds compact footer lines (status + session id) for terminal UIs.
package footerline

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SessionLabel returns a compact session marker, or "" if id is empty.
func SessionLabel(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	r := []rune(id)
	if len(r) > 12 {
		id = string(r[:12]) + "…"
	}
	return "sess·" + id
}

// Join appends the session label when there is room (width = terminal cells, or 0 = no trim).
func Join(primary, sessionID string, width int) string {
	label := SessionLabel(sessionID)
	line := strings.TrimSpace(primary)
	if label == "" {
		return line
	}
	if line == "" {
		return label
	}
	full := line + "  " + label
	if width <= 0 || lipgloss.Width(full) <= width {
		return full
	}
	gapW := lipgloss.Width("  " + label)
	maxPrimary := width - gapW
	if maxPrimary < 8 {
		return label
	}
	runes := []rune(line)
	for n := len(runes); n >= 1; n-- {
		cand := string(runes[:n])
		if n < len(runes) {
			cand += "…"
		}
		if lipgloss.Width(cand) <= maxPrimary {
			return cand + "  " + label
		}
	}
	return label
}

// HintsWithSession appends an optional session label to hints. When the combined string is wider
// than width (terminal cells, 0 = no wrap), the session label moves to the next line.
func HintsWithSession(hints, sessionID string, width int) string {
	hints = strings.TrimSpace(hints)
	label := SessionLabel(sessionID)
	if label == "" {
		return hints
	}
	if hints == "" {
		return label
	}
	combined := hints + "  " + label
	if width <= 0 || lipgloss.Width(combined) <= width {
		return combined
	}
	return hints + "\n" + label
}
