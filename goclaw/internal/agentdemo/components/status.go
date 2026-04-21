package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusStrip renders the top status line (model, mode, phase, token hint).
func StatusStrip(width int, accent, dim, panel lipgloss.Style, model, mode, phase string, approxTokens int) string {
	if width < 20 {
		width = 20
	}
	right := dim.Render(fmt.Sprintf("~%d tok est", approxTokens))
	left := fmt.Sprintf("%s · %s · %s", strings.TrimSpace(model), strings.TrimSpace(mode), strings.TrimSpace(phase))
	line := left
	if lipgloss.Width(line)+lipgloss.Width(right)+2 < width {
		pad := width - lipgloss.Width(line) - lipgloss.Width(right) - 2
		if pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		line += right
	}
	return panel.Width(width).Render(accent.Render(" goclaw agentdemo ")) + "\n" + panel.Width(width).Render(line)
}
