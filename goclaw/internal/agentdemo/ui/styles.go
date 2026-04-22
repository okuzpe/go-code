package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Styles holds Lip Gloss styles for the demo TUI (dark, minimal accent).
type Styles struct {
	Accent    lipgloss.Style
	Dim       lipgloss.Style
	User      lipgloss.Style
	Assistant lipgloss.Style
	Tool      lipgloss.Style
	ToolDim   lipgloss.Style
	Err       lipgloss.Style
	Panel     lipgloss.Style
	Overlay   lipgloss.Style
	Title     lipgloss.Style
}

func defaultStyles() Styles {
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	panel := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1)
	return Styles{
		Accent:    accent.Bold(true),
		Dim:       dim,
		User:      lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true),
		Assistant: lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Tool:      lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		ToolDim:   dim.Italic(true),
		Err:       lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		Panel:     panel.BorderForeground(lipgloss.Color("236")),
		Overlay: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("99")).
			Background(lipgloss.Color("235")).
			Padding(1, 2),
		Title: accent,
	}
}
