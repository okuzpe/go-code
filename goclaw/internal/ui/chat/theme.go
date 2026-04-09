package chat

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
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

	// New: modern polish styles
	ErrorStyle     lipgloss.Style
	Separator      lipgloss.Style
	ToolSpinner    lipgloss.Style
	ToolResultOk   lipgloss.Style
	ToolResultErr  lipgloss.Style
	InputBorder    lipgloss.Style // border around active input area

	// Markdown renderer (glamour), recreated when terminal width changes.
	mdMu       sync.Mutex
	mdWrap     int
	mdRenderer *glamour.TermRenderer
}

// DefaultTheme returns the standard in-terminal look: goclaw persona, clear 👤/🤖 lanes.
func DefaultTheme() *Theme {
	// Modern palette — inspired by Claude Code / Cursor / Warp
	accentUser := lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}  // emerald
	accentAI := lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C4B5FD"}   // purple (brand)
	muted := lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	dimFG := lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}
	toolFG := lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}     // amber
	modalBody := lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"}
	errorFG := lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}    // red
	sepFG := lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}      // subtle border

	return &Theme{
		AssistantName:  "goclaw",
		UserLabel:      "You",
		UserEmoji:      "❯",
		AssistantEmoji: "✦",
		InputPrompt:    "› ",

		System:    lipgloss.NewStyle().Foreground(muted).Italic(true),
		UserTag:   lipgloss.NewStyle().Bold(true).Foreground(accentUser),
		Assistant: lipgloss.NewStyle().Bold(true).Foreground(accentAI),
		Dim:       lipgloss.NewStyle().Foreground(dimFG),
		Tool:      lipgloss.NewStyle().Foreground(toolFG).Bold(true),
		ToolTag:   lipgloss.NewStyle().Foreground(toolFG),
		FooterDim: lipgloss.NewStyle().Foreground(muted),
		ModalBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C4B5FD"}),
		ModalTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C4B5FD"}),
		ModalBody:  lipgloss.NewStyle().Foreground(modalBody),

		// Modern polish
		ErrorStyle:    lipgloss.NewStyle().Foreground(errorFG).Bold(true),
		Separator:     lipgloss.NewStyle().Foreground(sepFG),
		ToolSpinner:   lipgloss.NewStyle().Foreground(toolFG).Italic(true),
		ToolResultOk:  lipgloss.NewStyle().Foreground(accentUser),
		ToolResultErr: lipgloss.NewStyle().Foreground(errorFG),
		InputBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#4B5563"}),

	}
}

// RenderMarkdown renders markdown text for terminal display using glamour.
// Word wrap follows the TUI width (minus gutter for the assistant prefix).
// Falls back to plain text if glamour fails.
func (t *Theme) RenderMarkdown(md string, width int) string {
	if strings.TrimSpace(md) == "" {
		return md
	}
	wrap := width - 10
	if wrap < 40 {
		wrap = 40
	}
	if wrap > 118 {
		wrap = 118
	}

	t.mdMu.Lock()
	if t.mdRenderer == nil || t.mdWrap != wrap {
		r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(wrap),
		)
		if err != nil {
			t.mdMu.Unlock()
			return md
		}
		t.mdRenderer = r
		t.mdWrap = wrap
	}
	r := t.mdRenderer
	t.mdMu.Unlock()

	rendered, err := r.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimRight(rendered, "\n")
}

// SeparatorLine renders a dim horizontal rule for turn separation.
func (t *Theme) SeparatorLine(width int) string {
	if width <= 0 {
		width = 60
	}
	w := width
	if w > 100 {
		w = 100
	}
	return t.Separator.Render(strings.Repeat("─", w))
}

// UserPrefix renders the left gutter for a user message (❯ You).
func (t *Theme) UserPrefix() string {
	return fmt.Sprintf("%s %s", t.UserEmoji, t.UserTag.Render(t.UserLabel))
}

// AssistantPrefix renders the left gutter for assistant streaming (✦ goclaw).
func (t *Theme) AssistantPrefix() string {
	return fmt.Sprintf("%s %s", t.AssistantEmoji, t.Assistant.Render(t.AssistantName))
}

// AssistantPlainPrefix is the visible prefix without ANSI (for strip/compare logic).
func (t *Theme) AssistantPlainPrefix() string {
	return fmt.Sprintf("%s %s", t.AssistantEmoji, t.AssistantName)
}

// FooterHint is the default status line when idle.
func (t *Theme) FooterHint() string {
	return "Enter: send · Ctrl+J: newline · /help · Esc / Ctrl+C: exit · Ctrl+L: clear"
}

// SpinnerAccentStyle is Lip Gloss v2 (required by bubbles/v2 spinner).
func SpinnerAccentStyle() lipv2.Style {
	return lipv2.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipv2.Color("#7C3AED"),
		Dark:  lipv2.Color("#C4B5FD"),
	})
}
