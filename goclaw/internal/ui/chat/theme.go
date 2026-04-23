package chat

import (
	"fmt"
	"strings"
	"sync"

	lipv2 "charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/text"
	"github.com/okuzpe/goclaw/internal/ui/icons"
	"github.com/okuzpe/goclaw/internal/ui/terminalstyle"
)

const (
	mdRenderRightMarginCells = 3
	mdMinWrapWidth           = 36

	separatorDefaultWidth = 56
	statusBarDefaultWidth = 60

	toolCardWidthInset            = 4
	toolCardMinInnerWidth         = 36
	toolCardDashMin               = 3
	toolInProgressSummaryMaxRunes = 120
	toolCardOutcomeMaxRunes       = 100
	toolCardSummaryLineMaxRunes   = 100 // per line when summary contains embedded newlines (glob/grep path sample)
)

func toolCardInnerWidth(termWidth int) int {
	cardW := termWidth - toolCardWidthInset
	if cardW < toolCardMinInnerWidth {
		cardW = toolCardMinInnerWidth
	}
	return cardW
}

func toolCardTopRule(t *Theme, headRendered string, cardW int) string {
	st := icons.Unicode
	if t != nil {
		st = t.Icons
	}
	nameW := lipgloss.Width(headRendered)
	dashCount := cardW - nameW - 2
	if dashCount < toolCardDashMin {
		dashCount = toolCardDashMin
	}
	return t.ToolCardBorder.Render(st.ToolCardTopLeft()) + headRendered + t.ToolCardBorder.Render(strings.Repeat(st.ToolCardH(), dashCount))
}

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
	ShellChrome   lipgloss.Style
	HeaderBrand   lipgloss.Style
	HeaderMeta    lipgloss.Style
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
	OverlayTitle   lipgloss.Style
	OverlayHint    lipgloss.Style

	// FooterWorkspaceChip styles the workspace basename in the idle footer (text only, no chip background).
	FooterWorkspaceChip lipgloss.Style

	// SlashPickerName / SlashPickerDesc style the / command rows above the input (TUI).
	SlashPickerName lipgloss.Style
	SlashPickerDesc lipgloss.Style

	// AtRefChip styles @path tokens inside user messages in the transcript.
	AtRefChip lipgloss.Style

	// WelcomeFrame colors the startup welcome panel borders (wide ASCII frame + narrow rounded box).
	WelcomeFrame lipgloss.Style

	// AgentDeck* styles the single-line profile rail above the fullscreen transcript (no block borders:
	// rounded Lip Gloss borders are multi-line and break layout next to the viewport).
	AgentDeckMicro   lipgloss.Style // faint secondary (lane hint, keys)
	AgentDeckRail    lipgloss.Style // left lane glyph
	AgentDeckBracket lipgloss.Style // ‹ › / < > around profile name
	AgentDeckTrace   lipgloss.Style // middle rhythm glyphs
	AgentDeckChord   lipgloss.Style // keybinding hint (kept subtle — not a second shouty accent)

	// Icons selects transcript/tool/footer glyphs (see config TUIIcons). Zero means Unicode preset.
	Icons icons.Set

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
	wrap := termWidth - prefixDisplayWidth - mdRenderRightMarginCells
	if wrap < mdMinWrapWidth {
		wrap = mdMinWrapWidth
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

// SeparatorLine renders a dim horizontal rule between turns.
// It uses a short centered section (40 % of terminal width, min 8, max 40 cells)
// flanked by spaces so the eye reads it as a divider rather than a page break.
// This lighter treatment matches modern CLI design (e.g. GitHub CLI, lazygit)
// where turn breaks are visible but do not compete with message content.
func (t *Theme) SeparatorLine(width int) string {
	if width <= 0 {
		width = separatorDefaultWidth
	}
	// Rule spans roughly 40 % of the terminal, capped so it stays compact.
	ruleW := width * 2 / 5
	if ruleW < 8 {
		ruleW = 8
	}
	if ruleW > 40 {
		ruleW = 40
	}
	st := icons.Unicode
	if t != nil {
		st = t.Icons
	}
	rule := t.Separator.Render(strings.Repeat(st.ToolCardH(), ruleW))
	return "  " + rule
}

// RenderToolCard builds a compact card for a completed tool call (claw-code style).
//
//	╭─ read_file ──────────────
//	│  src/main.go
//	│  42 lines · 1200 chars
//	╰─ ✓
//
// The │ bar uses the tool accent color to give each card a left-accent visual
// similar to VS Code diagnostic panels. outcome is optional (tool result hint).
func (t *Theme) RenderToolCard(toolLabel, summary, outcome string, isError bool, width int) string {
	cardW := toolCardInnerWidth(width)

	nameRendered := t.ToolCardHead.Render(" " + toolLabel + " ")
	header := toolCardTopRule(t, nameRendered, cardW)

	st := icons.Unicode
	if t != nil {
		st = t.Icons
	}
	var icon string
	if isError {
		icon = t.ToolResultErr.Render(st.ToolErr())
	} else {
		icon = t.ToolResultOk.Render(st.ToolOK())
	}
	footer := t.ToolCardBorder.Render(st.ToolCardBottomLeft()) + t.ToolCardBorder.Render(strings.Repeat(st.ToolCardH(), max(2, min(8, cardW/6)))) + " " + icon

	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(header)
	if s := strings.TrimSpace(summary); s != "" {
		for _, rawLine := range strings.Split(s, "\n") {
			line := strings.TrimSpace(rawLine)
			if line == "" {
				continue
			}
			line = text.TruncateRunes(line, toolCardSummaryLineMaxRunes)
			b.WriteString("\n  ")
			// Tool-accent color on the vertical bar creates a clear left-accent for the card.
			b.WriteString(t.ToolTag.Render(st.ToolCardV()))
			b.WriteString("  ")
			b.WriteString(t.ToolCardBody.Render(line))
		}
	}
	if o := strings.TrimSpace(outcome); o != "" {
		o = text.TruncateRunes(o, toolCardOutcomeMaxRunes)
		b.WriteString("\n  ")
		b.WriteString(t.ToolTag.Render(st.ToolCardV()))
		b.WriteString("  ")
		b.WriteString(t.FooterDim.Render(o))
	}
	b.WriteString("\n  ")
	b.WriteString(footer)
	return b.String()
}

// RenderThinkingRow returns an empty string — thinking state is shown exclusively in the
// status line (footerPrimaryStatus), not in the transcript. The lineMeta entry is still
// created so elapsed-time tracking works; this keeps the slot without visual noise.
func (t *Theme) RenderThinkingRow(_ string, _ int, _ int) string {
	return ""
}

// RenderToolRunning renders a single in-flight tool line using the compact »/< pair style.
// Format: "» toolLabel: summary (Ns)"
// Used for the transcript row while a tool is executing and for progress updates.
func (t *Theme) RenderToolRunning(toolLabel, summary string, elapsedSec int, _ int) string {
	callGlyph := t.ShellChrome.Render("»")
	name := t.Tool.Render(toolLabel)
	sum := strings.TrimSpace(summary)
	if elapsedSec >= 2 {
		if sum != "" {
			sum = fmt.Sprintf("%s (%ds)", sum, elapsedSec)
		} else {
			sum = fmt.Sprintf("(%ds)", elapsedSec)
		}
	}
	line := callGlyph + " " + name
	if sum != "" {
		line += ": " + t.FooterDim.Render(text.TruncateRunes(sum, toolInProgressSummaryMaxRunes))
	}
	return "  " + line
}

// RenderToolPair renders a completed tool call as a compact two-line »/< pair.
//
//	» read_file: src/main.go
//	< 142 lines loaded  [+]
//
// The first result line is always visible; truncated output appends a dim "[+]" marker.
// Error results use "!" instead of "<" and are rendered in the error color.
func (t *Theme) RenderToolPair(toolLabel, summary, content string, isError bool, width int) string {
	callGlyph := t.ShellChrome.Render("»")
	name := t.Tool.Render(toolLabel)
	callLine := "  " + callGlyph + " " + name
	if sum := strings.TrimSpace(summary); sum != "" {
		callLine += ": " + t.FooterDim.Render(text.TruncateRunes(sum, toolInProgressSummaryMaxRunes))
	}

	var resultGlyph string
	if isError {
		resultGlyph = t.ErrorStyle.Render("!")
	} else {
		resultGlyph = t.ToolCardBody.Render("<")
	}

	result := strings.TrimSpace(content)
	firstLine := result
	truncated := false
	if nl := strings.IndexByte(result, '\n'); nl >= 0 {
		firstLine = result[:nl]
		truncated = true
	}
	maxResultW := width - 6
	if maxResultW < 20 {
		maxResultW = 60
	}
	firstLine = strings.TrimSpace(firstLine)
	if len([]rune(firstLine)) > maxResultW {
		firstLine = text.TruncateRunes(firstLine, maxResultW)
		truncated = true
	}

	var resultLine string
	if firstLine == "" && isError {
		resultLine = "  " + resultGlyph + " " + t.ErrorStyle.Render("tool error")
	} else {
		body := t.ToolCardBody.Render(firstLine)
		if truncated {
			body += " " + t.FooterDim.Render("[+]")
		}
		resultLine = "  " + resultGlyph + " " + body
	}

	return callLine + "\n" + resultLine
}

// RenderToolInProgressRow delegates to RenderToolRunning for backward compatibility
// (called from reflowTranscriptForWidth for lineKindToolRunning rows).
func (t *Theme) RenderToolInProgressRow(toolLabel, summary string, elapsedSec int, width int) string {
	return t.RenderToolRunning(toolLabel, summary, elapsedSec, width)
}

// StatusBarRender renders a one-line status bar with separator and content.
func (t *Theme) StatusBarRender(status string, width int) string {
	if width <= 0 {
		width = statusBarDefaultWidth
	}
	st := icons.Unicode
	if t != nil {
		st = t.Icons
	}
	bar := t.Separator.Render(strings.Repeat(st.ToolCardH(), width))
	if strings.TrimSpace(status) == "" {
		return bar
	}
	return bar + "\n" + t.StatusBar.Render(status)
}

// UserPrefix renders the left gutter for a user message (Claude Code style: ">").
// The glyph is always rendered in the user accent color so it stands out from plain text.
func (t *Theme) UserPrefix() string {
	if strings.TrimSpace(t.UserLabel) == "" {
		return t.UserTag.Render(t.UserEmoji)
	}
	return fmt.Sprintf("%s %s", t.UserTag.Render(t.UserEmoji), t.UserTag.Render(t.UserLabel))
}

// AssistantPrefix renders the assistant gutter (Claude Code style bullet; see Theme.Icons).
// The glyph is always rendered in the AI accent color for consistent visual hierarchy.
func (t *Theme) AssistantPrefix() string {
	bullet := icons.Unicode.AssistantBullet()
	if t != nil {
		bullet = t.Icons.AssistantBullet()
	}
	if strings.TrimSpace(t.AssistantName) == "" {
		return t.Assistant.Render(bullet)
	}
	return fmt.Sprintf("%s %s", t.Assistant.Render(bullet), t.Assistant.Render(t.AssistantName))
}

// AssistantPlainPrefix is the visible prefix without ANSI (for strip/compare logic).
func (t *Theme) AssistantPlainPrefix() string {
	bullet := icons.Unicode.AssistantBullet()
	if t != nil {
		bullet = t.Icons.AssistantBullet()
	}
	if strings.TrimSpace(t.AssistantName) == "" {
		return bullet
	}
	return fmt.Sprintf("%s %s", bullet, t.AssistantName)
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
