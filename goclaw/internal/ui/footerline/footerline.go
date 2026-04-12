// Package footerline builds compact footer lines (status + session id) for terminal UIs.
package footerline

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SessionLabel returns a compact session marker, or "" if id is empty.
// Uses a git-style # prefix so the id reads as metadata on the right edge of the footer.
func SessionLabel(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	r := []rune(id)
	if len(r) > 10 {
		return "#" + string(r[:8]) + "…"
	}
	return "#" + string(r)
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

// TrimToMaxWidth returns s trimmed to max display cells, with an ellipsis when truncated.
func TrimToMaxWidth(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	r := []rune(s)
	for n := len(r); n >= 1; n-- {
		cand := string(r[:n])
		if n < len(r) {
			cand += "…"
		}
		if lipgloss.Width(cand) <= max {
			return cand
		}
	}
	return ""
}

// AlignedHintsSession builds one idle footer row: brand, optional stats, and keys on the left;
// the session tag is right-aligned when the terminal is wide enough. Falls back to HintsWithSession
// when the row would be too crowded (session moves to the next line).
func AlignedHintsSession(brand, stats, keys, sessionID string, width int) string {
	label := SessionLabel(sessionID)
	var parts []string
	if b := strings.TrimSpace(brand); b != "" {
		parts = append(parts, b)
	}
	if st := strings.TrimSpace(stats); st != "" {
		parts = append(parts, st)
	}
	if k := strings.TrimSpace(keys); k != "" {
		parts = append(parts, k)
	}
	left := strings.Join(parts, " · ")
	if label == "" {
		return TrimToMaxWidth(left, width)
	}
	if width <= 0 {
		return strings.TrimSpace(left + "  " + label)
	}
	rightW := lipgloss.Width(label)
	const gapMin = 2
	maxLeft := width - rightW - gapMin
	if maxLeft < 20 {
		return HintsWithSession(left, sessionID, width)
	}
	leftShown := left
	if lipgloss.Width(left) > maxLeft {
		leftShown = TrimToMaxWidth(left, maxLeft)
	}
	sp := width - lipgloss.Width(leftShown) - rightW
	if sp < gapMin {
		sp = gapMin
	}
	return leftShown + strings.Repeat(" ", sp) + label
}

// IdleFooterTwoLines returns two footer rows for wide terminals: row1 is brand · stats with the
// session tag right-aligned; row2 is keyboard hints only (easier to scan than one long line).
func IdleFooterTwoLines(brand, stats, keys, sessionID string, width int) string {
	row1 := AlignedHintsSession(brand, stats, "", sessionID, width)
	row2 := TrimToMaxWidth(keys, width)
	if strings.TrimSpace(row1) == "" {
		return row2
	}
	if strings.TrimSpace(row2) == "" {
		return row1
	}
	return row1 + "\n" + row2
}
