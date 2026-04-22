package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// phaseIcon returns a short colored indicator for the current phase.
// spinnerView is the current animated frame from the spinner component.
func phaseIcon(phase, spinnerView string, accent, dim lipgloss.Style) string {
	switch phase {
	case "thinking":
		return accent.Render(spinnerView) + dim.Render(" thinking")
	case "streaming":
		return accent.Render("▸") + dim.Render(" streaming")
	case "executing":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render(spinnerView + " executing")
	case "idle":
		return dim.Render("· idle")
	default:
		return dim.Render(phase)
	}
}

// StatusStrip renders a two-line status bar: branding line + info line.
// spinnerView is the current animated frame from the spinner component.
// When unsafe is true an indicator is shown next to the mode label.
func StatusStrip(width int, accent, dim, panel lipgloss.Style, model, mode, phase, spinnerView string, approxTokens int, unsafe bool) string {
	if width < 20 {
		width = 20
	}

	modeStyle := dim
	if mode == "agent" {
		modeStyle = accent
	}

	// Line 1: branding
	header := panel.Width(width).Render(
		accent.Render(" ◆ agentdemo") + dim.Render("  "+strings.TrimSpace(model)),
	)

	// Line 2: mode + phase + token hint
	right := dim.Render(fmt.Sprintf("~%d tok", approxTokens))
	modeLabel := "[" + mode + "]"
	if unsafe {
		modeLabel += lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("⚠")
	}
	left := modeStyle.Render(modeLabel) + "  " + phaseIcon(phase, spinnerView, accent, dim)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}
	info := panel.Width(width).Render(left + strings.Repeat(" ", gap) + right)

	return header + "\n" + info
}
