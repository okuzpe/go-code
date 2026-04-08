package chat

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	lipv2 "charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// Theme holds Lip Gloss styles and copy for the chat TUI. Centralize here so the
// assistant persona, prompt, and palette stay consistent (Claude Code–style polish).
type Theme struct {
	AssistantName string
	UserLabel     string

	UserEmoji      string
	AssistantEmoji string
	InputPrompt    string

	System    lipgloss.Style
	UserTag   lipgloss.Style
	Assistant lipgloss.Style
	Dim       lipgloss.Style
	Tool      lipgloss.Style
	ToolTag   lipgloss.Style
	FooterDim lipgloss.Style
	ModalBorder lipgloss.Style
	ModalTitle  lipgloss.Style
	ModalBody   lipgloss.Style
}

// DefaultTheme returns the standard in-terminal look: goclaw persona, clear 👤/🤖 lanes.
func DefaultTheme() *Theme {
	accentUser := lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
	accentAI := lipgloss.AdaptiveColor{Light: "#4F46E5", Dark: "#818CF8"}
	muted := lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	dimFG := lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}
	toolFG := lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	modalBody := lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"}

	return &Theme{
		AssistantName:  "goclaw",
		UserLabel:      "You",
		UserEmoji:      "👤",
		AssistantEmoji: "🤖",
		InputPrompt:    "goclaw > ",

		System:    lipgloss.NewStyle().Foreground(muted),
		UserTag:   lipgloss.NewStyle().Bold(true).Foreground(accentUser),
		Assistant: lipgloss.NewStyle().Bold(true).Foreground(accentAI),
		Dim:       lipgloss.NewStyle().Foreground(dimFG),
		Tool:      lipgloss.NewStyle().Foreground(toolFG).Bold(true),
		ToolTag:   lipgloss.NewStyle().Foreground(toolFG),
		FooterDim: lipgloss.NewStyle().Foreground(muted),
		ModalBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F59E0B")),
		ModalTitle: lipgloss.NewStyle().Bold(true),
		ModalBody:  lipgloss.NewStyle().Foreground(modalBody),
	}
}

// UserPrefix renders the left gutter for a user message (👤 You).
func (t *Theme) UserPrefix() string {
	return fmt.Sprintf("%s %s", t.UserEmoji, t.UserTag.Render(t.UserLabel))
}

// AssistantPrefix renders the left gutter for assistant streaming (🤖 goclaw).
func (t *Theme) AssistantPrefix() string {
	return fmt.Sprintf("%s %s", t.AssistantEmoji, t.Assistant.Render(t.AssistantName))
}

// AssistantPlainPrefix is the visible prefix without ANSI (for strip/compare logic).
func (t *Theme) AssistantPlainPrefix() string {
	return fmt.Sprintf("%s %s", t.AssistantEmoji, t.AssistantName)
}

// FooterHint is the default status line when idle.
func (t *Theme) FooterHint() string {
	return "Enter: send · /edit: multiline · /help · Esc / Ctrl+C: exit · Ctrl+L: clear"
}

// SpinnerAccentStyle is Lip Gloss v2 (required by bubbles/v2 spinner).
func SpinnerAccentStyle() lipv2.Style {
	return lipv2.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipv2.Color("#7C3AED"),
		Dark:  lipv2.Color("#C4B5FD"),
	})
}
