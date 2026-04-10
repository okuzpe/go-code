package chat

import (
	"fmt"
	"strings"
	"sync"

	lipv2 "charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Theme holds Lip Gloss styles and copy for the chat TUI. Centralize here so the
// assistant persona, prompt, and palette stay consistent (Claude Code–style polish).
type Theme struct {
	AssistantName string
	UserLabel     string

	UserEmoji      string
	AssistantEmoji string
	InputPrompt    string

	System      lipgloss.Style
	UserTag     lipgloss.Style
	Assistant   lipgloss.Style
	Dim         lipgloss.Style
	Tool        lipgloss.Style
	ToolTag     lipgloss.Style
	FooterDim   lipgloss.Style
	ModalBorder lipgloss.Style
	ModalTitle  lipgloss.Style
	ModalBody   lipgloss.Style

	// New: modern polish styles
	ErrorStyle    lipgloss.Style
	Separator     lipgloss.Style
	ToolSpinner   lipgloss.Style
	ToolResultOk  lipgloss.Style
	ToolResultErr lipgloss.Style
	InputBorder   lipgloss.Style // border around active input area

	// SlashPickerName / SlashPickerDesc style the / command rows above the input (TUI).
	SlashPickerName lipgloss.Style
	SlashPickerDesc lipgloss.Style

	// Markdown renderer (glamour), recreated when terminal width or mdGlamourStyle changes.
	mdMu           sync.Mutex
	mdWrap         int
	mdGlamourStyle string // "auto", "dark", "light", "ascii" — passed to glamour
	mdBuiltStyle   string // style key last used to construct mdRenderer
	mdRenderer     *glamour.TermRenderer

	// appearance is the canonical preset name (see config.NormalizeUIAppearance); used for spinner accents.
	appearance string
}

// DefaultTheme returns the standard in-terminal look: goclaw persona, clear 👤/🤖 lanes.
func DefaultTheme() *Theme {
	// Modern palette — inspired by Claude Code / Cursor / Warp
	accentUser := lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"} // emerald
	accentAI := lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C4B5FD"}   // purple (brand)
	muted := lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	dimFG := lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}
	toolFG := lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"} // amber
	modalBody := lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"}
	errorFG := lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"} // red
	sepFG := lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}   // subtle border
	slashPick := lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"} // blue command names
	slashDesc := lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"}

	return &Theme{
		AssistantName:  "goclaw",
		UserLabel:      "You",
		UserEmoji:      "❯",
		AssistantEmoji: "✦",
		InputPrompt:    "› ",
		mdGlamourStyle: "auto",
		appearance:     "auto",

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
		SlashPickerName: lipgloss.NewStyle().Bold(true).Foreground(slashPick),
		SlashPickerDesc: lipgloss.NewStyle().Foreground(slashDesc),
	}
}

// RenderMarkdown renders markdown text for terminal display using glamour.
// prefixDisplayWidth is lipgloss.Width(AssistantPrefix()) so wrapped lines fit the column under the gutter.
// Falls back to plain text if glamour fails.
func (t *Theme) RenderMarkdown(md string, termWidth int, prefixDisplayWidth int) string {
	if strings.TrimSpace(md) == "" {
		return md
	}
	// Reserve: prefix column + one space after prefix + small right margin.
	margin := 3
	wrap := termWidth - prefixDisplayWidth - margin
	if wrap < 36 {
		wrap = 36
	}
	if wrap > 120 {
		wrap = 120
	}

	glamStyle := t.mdGlamourStyle
	if glamStyle == "" {
		glamStyle = "auto"
	}

	t.mdMu.Lock()
	if t.mdRenderer == nil || t.mdWrap != wrap || t.mdBuiltStyle != glamStyle {
		var opts []glamour.TermRendererOption
		opts = append(opts, glamour.WithWordWrap(wrap))
		switch glamStyle {
		case "auto":
			opts = append(opts, glamour.WithAutoStyle())
		default:
			opts = append(opts, glamour.WithStandardStyle(glamStyle))
		}
		r, err := glamour.NewTermRenderer(opts...)
		if err != nil {
			t.mdMu.Unlock()
			return md
		}
		t.mdRenderer = r
		t.mdWrap = wrap
		t.mdBuiltStyle = glamStyle
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

// FooterHint is the default hints row when idle (session id is shown separately in the TUI footer).
func (t *Theme) FooterHint() string {
	return t.FooterHintForWidth(0)
}

// FooterHintForWidth returns footer hints; when width is small, uses a shorter line so it does not
// hard-wrap mid-token in narrow terminals.
func (t *Theme) FooterHintForWidth(termWidth int) string {
	const full = "Enter send · Ctrl+J newline · ? /help · / Tab · Esc · stop reply while streaming, exit when idle · Ctrl+C · quit · Ctrl+L clear"
	const mid = "Enter · Ctrl+J newline · ? /help · Tab · Esc / Ctrl+C · Ctrl+L clear"
	const short = "Enter · /help · Esc · Ctrl+C quit · Ctrl+L clear"
	if termWidth <= 0 {
		return full
	}
	if termWidth < 58 {
		return short
	}
	if termWidth < 96 {
		return mid
	}
	return full
}

// SpinnerAccentStyle is Lip Gloss v2 (required by bubbles/v2 spinner).
func SpinnerAccentStyle() lipv2.Style {
	return lipv2.NewStyle().Foreground(compat.AdaptiveColor{
		Light: lipv2.Color("#7C3AED"),
		Dark:  lipv2.Color("#C4B5FD"),
	})
}
