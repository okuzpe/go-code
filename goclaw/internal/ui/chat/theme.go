package chat

import (
	"fmt"
	"strings"
	"sync"

	lipv2 "charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/ui/terminalstyle"
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

	ErrorStyle    lipgloss.Style
	Separator     lipgloss.Style
	ToolSpinner   lipgloss.Style
	ToolResultOk  lipgloss.Style
	ToolResultErr lipgloss.Style
	InputBorder   lipgloss.Style

	// Tool card styles (claw-code inspired box-drawing panels).
	ToolCardBorder lipgloss.Style
	ToolCardHead   lipgloss.Style
	ToolCardBody   lipgloss.Style

	// Status bar between viewport and input area.
	StatusBar      lipgloss.Style
	StatusBarLabel lipgloss.Style

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

// DefaultTheme returns the standard in-terminal look (ui_appearance auto).
func DefaultTheme() *Theme {
	return newThemeFromPalette(terminalstyle.PaletteForAppearance(""))
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

// SeparatorLine renders a dim horizontal rule between turns (kept shorter than full width
// so the transcript matches Claude Code–style breathing room).
func (t *Theme) SeparatorLine(width int) string {
	if width <= 0 {
		width = 56
	}
	w := width
	const maxRule = 72
	if w > maxRule {
		w = maxRule
	}
	if w < 24 {
		w = 24
	}
	return t.Separator.Render(strings.Repeat("─", w))
}

// RenderToolCard builds a compact card for a completed tool call (claw-code style).
//
//	╭─ read_file ──────────────
//	│  src/main.go
//	╰─ ✓
func (t *Theme) RenderToolCard(toolLabel, summary string, isError bool, width int) string {
	cardW := width - 4
	if cardW < 36 {
		cardW = 36
	}
	if cardW > 88 {
		cardW = 88
	}

	nameRendered := t.ToolCardHead.Render(" " + toolLabel + " ")
	nameW := lipgloss.Width(nameRendered)
	dashCount := cardW - nameW - 2
	if dashCount < 3 {
		dashCount = 3
	}
	header := t.ToolCardBorder.Render("╭─") + nameRendered + t.ToolCardBorder.Render(strings.Repeat("─", dashCount))

	var icon string
	if isError {
		icon = t.ToolResultErr.Render("✗")
	} else {
		icon = t.ToolResultOk.Render("✓")
	}
	footer := t.ToolCardBorder.Render("╰─") + " " + icon

	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(header)
	if s := strings.TrimSpace(summary); s != "" {
		b.WriteString("\n  ")
		b.WriteString(t.ToolCardBorder.Render("│"))
		b.WriteString("  ")
		b.WriteString(t.ToolCardBody.Render(s))
	}
	b.WriteString("\n  ")
	b.WriteString(footer)
	return b.String()
}

// StatusBarRender renders a one-line status bar with separator and content.
func (t *Theme) StatusBarRender(status string, width int) string {
	if width <= 0 {
		width = 60
	}
	w := width
	if w > 100 {
		w = 100
	}
	bar := t.Separator.Render(strings.Repeat("─", w))
	if strings.TrimSpace(status) == "" {
		return bar
	}
	return bar + "\n" + t.StatusBar.Render(status)
}

// UserPrefix renders the left gutter for a user message (Claude Code style: ">").
func (t *Theme) UserPrefix() string {
	if strings.TrimSpace(t.UserLabel) == "" {
		return t.UserEmoji
	}
	return fmt.Sprintf("%s %s", t.UserEmoji, t.UserTag.Render(t.UserLabel))
}

// AssistantPrefix renders the assistant gutter (Claude Code style: "●").
func (t *Theme) AssistantPrefix() string {
	if strings.TrimSpace(t.AssistantName) == "" {
		return t.AssistantEmoji
	}
	return fmt.Sprintf("%s %s", t.AssistantEmoji, t.Assistant.Render(t.AssistantName))
}

// AssistantPlainPrefix is the visible prefix without ANSI (for strip/compare logic).
func (t *Theme) AssistantPlainPrefix() string {
	if strings.TrimSpace(t.AssistantName) == "" {
		return t.AssistantEmoji
	}
	return fmt.Sprintf("%s %s", t.AssistantEmoji, t.AssistantName)
}

// AppearancePreset returns the ui_appearance key for this theme (e.g. "dark", "auto").
func (t *Theme) AppearancePreset() string {
	if t == nil {
		return ""
	}
	return t.appearance
}

// SpinnerAccentStyle is Lip Gloss v2 (required by bubbles/v2 spinner when theme is nil).
func SpinnerAccentStyle() lipv2.Style {
	return terminalstyle.SpinnerAccentLipV2("")
}

// SpinnerAccentV2 returns the Bubbles v2 spinner style for this theme preset.
func (t *Theme) SpinnerAccentV2() lipv2.Style {
	if t == nil {
		return SpinnerAccentStyle()
	}
	return terminalstyle.SpinnerAccentLipV2(t.appearance)
}
