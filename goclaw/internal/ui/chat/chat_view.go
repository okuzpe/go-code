package chat

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/okuzpe/goclaw/internal/text"
	"github.com/okuzpe/goclaw/internal/ui/footerline"
)

func (m *Model) syncInputComposeWidth() {
	if m.width <= 4 {
		m.input.SetWidth(0)
		return
	}
	maxW := m.width - 4
	if maxW < minComposeWidth {
		maxW = minComposeWidth
	}
	m.input.SetWidth(maxW)
}

// transcriptScrollNavHint is a short footer line for reading long assistant output in the TUI.
func (m *Model) transcriptScrollNavHint() string {
	return transcriptScrollNavFooterLine(m.tuiMouseScroll)
}

// footerTranscriptGuideLine returns at most one dim line for scroll help: busy-stream hint while
// working, otherwise the post-reply idle hint. Suppressed during tool approval or when the send
// queue is visible so the footer stays shorter.
func (m *Model) footerTranscriptGuideLine(termWidth int) string {
	if m.pending != nil || len(m.messageQueue) > 0 {
		return ""
	}
	if m.streaming || m.spinnerActive {
		return strings.TrimSpace(m.streamBusyFooterScrollHint(termWidth))
	}
	return strings.TrimSpace(m.idleTranscriptHint)
}

// streamBusyFooterScrollHint is a dim footer line while the model is working so users can read
// earlier transcript lines (see tryBusyComposeTranscriptScroll). Empty when overlays or approval
// modal are active. Copy lives in transcript_scroll_hints.go.
func (m *Model) streamBusyFooterScrollHint(termWidth int) string {
	if m.transcriptBrowse || m.pending != nil {
		return ""
	}
	if !(m.streaming || m.spinnerActive) {
		return ""
	}
	narrow := termWidth > 0 && termWidth < composePlaceholderNarrowMaxW
	empty := strings.TrimSpace(m.input.Value()) == ""
	return streamBusyTranscriptScrollFooterLine(m.tuiMouseScroll, empty, narrow)
}

// refreshFooterStatsCache recomputes the idle footer stats line (token / compact hints). Call after
// session changes or on the periodic footer tick — not on every keystroke (see tuiFooterStats).
func (m *Model) refreshFooterStatsCache() {
	if m.footerStats == nil {
		m.footerStatsLine = ""
		return
	}
	line := strings.TrimSpace(m.footerStats())
	if line != "" {
		mode := config.NormalizeTUIInteractMode(m.interactMode)
		line = line + " · ui:" + mode
	}
	m.footerStatsLine = line
}

// resizeInput dynamically adjusts the textarea height based on content.
func (m *Model) resizeInput() {
	lineCount := m.input.LineCount()
	if lineCount < 1 {
		lineCount = 1
	}
	if lineCount > inputMaxHeight {
		lineCount = inputMaxHeight
	}
	if m.inputResizeLineCount == lineCount {
		return
	}
	m.inputResizeLineCount = lineCount
	m.input.SetHeight(lineCount)
	m.layout()
}

// viewportSetJoinedContent updates transcript content in the viewport, preserving scroll offset
// when the user was not pinned to the bottom (so periodic refreshes do not yank the view).
func (m *Model) viewportSetJoinedContent(joined string, stickToBottom bool) {
	// Empty viewport reports AtBottom() == true; do not jump to end on first paint (tall welcome
	// in a short CMD window should start scrolled to the top).
	stick := stickToBottom || (m.viewport.TotalLineCount() > 0 && m.viewport.AtBottom())
	preserveY := m.viewport.YOffset()
	m.viewport.SetContent(joined)
	m.lastTranscript = joined
	if stick {
		m.viewport.GotoBottom()
	} else {
		m.viewport.SetYOffset(preserveY)
	}
}

// setLinesContent refreshes the transcript in the viewport. If stickToBottom is true or the user
// was already at the bottom, scroll stays pinned to the latest output.
func (m *Model) setLinesContent(stickToBottom bool) {
	m.viewportSetJoinedContent(strings.Join(m.lines, "\n"), stickToBottom)
}

func (m *Model) syncInputPlaceholder() {
	m.input.Placeholder = placeholderForWidth(m.width)
}

// footerWorkspaceBrand is the idle-footer left label: workspace basename (no folder glyph), or Context.
func (m *Model) footerWorkspaceBrand() string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	w := strings.TrimSpace(m.workdir)
	if w == "" {
		return th.FooterDim.Render("Context")
	}
	base := filepath.Base(w)
	if base == "." || base == "/" || base == string(filepath.Separator) {
		return th.FooterDim.Render("Context")
	}
	return th.FooterWorkspaceChip.Render(base)
}

func (m *Model) footerPrimaryStatus() string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	status := strings.TrimSpace(m.statusLine)
	if m.spinnerActive {
		spin := strings.TrimSpace(m.spinner.View())
		base := strings.TrimSpace(status)
		if base == "" {
			if m.assistantPlaceholder {
				lab := strings.TrimSpace(m.lastThinkingPhase)
				if lab == "" {
					lab = "Thinking"
				}
				base = th.StatusBarLabel.Render(lab) + th.FooterDim.Render("…")
			} else {
				base = th.StatusBarLabel.Render("Responding") + th.FooterDim.Render("…")
			}
		}
		return strings.TrimSpace(spin + "  " + base)
	}
	return status
}

func (m *Model) View() tea.View {
	// Keep viewport height in sync with the footer on every frame. Footer line count changes when
	// the spinner/status row appears or disappears; if we only relied on layout() from resize/typing,
	// the transcript viewport could be sized for the wrong footer and clip or overlap the UI.
	m.layout()
	var content string
	if strings.TrimSpace(m.profileBarRendered) != "" {
		content = lipgloss.JoinVertical(lipgloss.Left,
			m.profileBarRendered,
			m.viewport.View(),
			m.footerRendered,
		)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			m.footerRendered,
		)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	if m.tuiMouseScroll {
		// Enables wheel delivery to the bubbles viewport (selection behavior may change per terminal).
		v.MouseMode = tea.MouseModeCellMotion
	} else {
		// MouseModeNone keeps the host terminal’s normal mouse selection and copy.
		v.MouseMode = tea.MouseModeNone
	}
	return v
}

func (m *Model) applySlashHints(hints slashcmd.UIHints) {
	if hints.RefreshWelcome {
		if hints.WelcomeProfile != "" {
			m.welcomeOpts.Profile = hints.WelcomeProfile
			m.activeAgentProfile = hints.WelcomeProfile
		}
		if hints.WelcomeSubtitle != "" {
			m.welcomeOpts.Subtitle = hints.WelcomeSubtitle
		}
		if hints.WelcomeFileWriteToolsHidden != nil {
			m.welcomeOpts.FileWriteToolsHidden = *hints.WelcomeFileWriteToolsHidden
		}
		if hints.WelcomeHubDelegatesCoding != nil {
			m.welcomeOpts.HubDelegatesCoding = *hints.WelcomeHubDelegatesCoding
		}
		if hints.WelcomeWriteApprovalRequired != nil {
			m.welcomeOpts.WriteApprovalRequired = *hints.WelcomeWriteApprovalRequired
		}
		if m.welcomeBlockEnd > 0 {
			m.rebuildWelcomeForWidth()
			m.preambleEnd = m.welcomeBlockEnd
		}
	}
	if hints.ReloadTranscript != nil {
		m.rebuildTranscriptForSession(hints.ReloadTranscript)
	}
	if hints.FooterHint != nil {
		m.footerHint = strings.TrimSpace(*hints.FooterHint)
	}
}

// profileModeBarView returns the single-line profile rail above the transcript (see agent_deck_rail.go).
func (m *Model) profileModeBarView() string {
	if m.cycleAgentProfileFn == nil {
		return ""
	}
	if m.toolLogOpen || m.docOverlayOpen || m.themePickOpen || m.agentPickOpen {
		return ""
	}
	if m.width <= 0 {
		return ""
	}
	if strings.TrimSpace(m.activeAgentProfile) == "" {
		return ""
	}
	return m.agentDeckRailView()
}

func (m *Model) layout() {
	m.syncInputComposeWidth()
	m.profileBarRendered = m.profileModeBarView()
	barH := lipgloss.Height(m.profileBarRendered)
	foot := m.footerView()
	m.footerRendered = foot
	footerH := lipgloss.Height(foot)
	h := m.height - footerH - barH
	if h < 1 {
		h = 1
	}
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(h)
	if m.toolLogOpen {
		m.viewport.SetContent(m.toolLogText)
		m.lastTranscript = m.toolLogText
		return
	}
	if m.docOverlayOpen {
		th := m.theme
		if th == nil {
			th = DefaultTheme()
		}
		rendered := ""
		if strings.TrimSpace(m.docOverlaySourceMD) != "" {
			rendered = th.RenderMarkdown(m.docOverlaySourceMD, m.width, 0)
		}
		m.viewport.SetContent(rendered)
		m.lastTranscript = rendered
		return
	}
	if m.agentPickOpen {
		m.viewport.SetContent(m.agentPickFullText)
		m.lastTranscript = m.agentPickFullText
		return
	}
	if m.themePickOpen {
		m.viewport.SetContent(m.themePickFullText)
		m.lastTranscript = m.themePickFullText
		return
	}
	joined := strings.Join(m.lines, "\n")
	if joined != m.lastTranscript {
		m.viewportSetJoinedContent(joined, false)
	}
}

func (m *Model) openDocOverlay(title, sourceMD string) {
	m.exitTranscriptBrowse()
	m.exitConfirmDeadline = time.Time{}
	m.toolLogOpen = false
	m.toolLogDetail = false
	m.toolLogText = ""
	m.themePickOpen = false
	m.themePickFullText = ""
	m.agentPickOpen = false
	m.agentPickFullText = ""
	m.docOverlayTitle = strings.TrimSpace(title)
	m.docOverlaySourceMD = strings.TrimSpace(sourceMD)
	m.docOverlayOpen = true
	m.syncViewportKeyMapForOverlay()
	m.layout()
	m.viewport.GotoTop()
}

func (m *Model) openHelpDocOverlay(replBodyMD string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	var b strings.Builder
	if m.appVersion != "" {
		b.WriteString("# goclaw · v")
		b.WriteString(m.appVersion)
	} else {
		b.WriteString("# goclaw")
	}
	b.WriteString("\n\n")
	b.WriteString(slashcmd.TUIHelpShortcutsMarkdown(th.Icons))
	b.WriteString("\n\n")
	b.WriteString(strings.TrimSpace(replBodyMD))
	m.openDocOverlay("Help", b.String())
}

func (m *Model) closeDocOverlay() {
	m.docOverlayOpen = false
	m.docOverlayTitle = ""
	m.docOverlaySourceMD = ""
	m.syncViewportKeyMapForCompose()
	m.layout()
	m.viewport.GotoBottom()
}

func (m *Model) footerView() string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	if m.toolLogOpen {
		var line string
		if m.toolLogDetail {
			line = "Esc back · Ctrl+C quit"
		} else {
			line = "↑↓ move · Enter view · Esc close · Ctrl+C quit"
		}
		if m.width > 4 {
			return th.FooterDim.Width(m.width).Render(line)
		}
		return th.FooterDim.Render(line)
	}
	if m.docOverlayOpen {
		line := "Esc · Ctrl+C quit"
		if m.width > 4 {
			return th.FooterDim.Width(m.width).Render(line)
		}
		return th.FooterDim.Render(line)
	}
	if m.themePickOpen {
		line := "↑↓ · Enter apply · Esc cancel · Ctrl+C quit"
		if m.width > 4 {
			return th.FooterDim.Width(m.width).Render(line)
		}
		return th.FooterDim.Render(line)
	}
	if m.agentPickOpen {
		line := "↑↓ · Enter apply · Esc cancel · Ctrl+C quit"
		if m.width > 4 {
			return th.FooterDim.Width(m.width).Render(line)
		}
		return th.FooterDim.Render(line)
	}

	fw := m.width
	primary := strings.TrimSpace(m.footerPrimaryStatus())
	stats := strings.TrimSpace(m.footerStatsLine)

	var b strings.Builder

	// Status row: spinner/thinking indicator with accent label, right-aligned stats.
	// Shown only while active so idle UI does not grow an extra blank line.
	if primary != "" {
		var statusRow string
		if stats != "" && fw > 40 {
			// Right-align stats next to the primary status when there is room.
			statW := lipgloss.Width(stats)
			primW := lipgloss.Width(primary)
			gap := fw - primW - statW
			if gap < 2 {
				gap = 2
			}
			statusRow = primary + strings.Repeat(" ", gap) + th.FooterDim.Render(stats)
		} else {
			statusRow = primary
		}
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(statusRow))
		} else {
			b.WriteString(th.FooterDim.Render(statusRow))
		}
		b.WriteString("\n")
	} else if stats != "" {
		// Idle: show stats (message/token budget for the LLM context window — not assistant output).
		// Leading workspace chip (basename) plus optional #session id on the right.
		session := footerline.AlignedHintsSession(m.footerWorkspaceBrand(), stats, "", m.sessionID, fw)
		if strings.TrimSpace(session) != "" {
			if fw > 4 {
				b.WriteString(th.FooterDim.Width(fw).Render(session))
			} else {
				b.WriteString(th.FooterDim.Render(session))
			}
			b.WriteString("\n")
		}
	}

	if fh := strings.TrimSpace(m.footerHint); fh != "" {
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(fh))
		} else {
			b.WriteString(th.FooterDim.Render(fh))
		}
		b.WriteString("\n")
	}
	if guide := m.footerTranscriptGuideLine(fw); guide != "" {
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(guide))
		} else {
			b.WriteString(th.FooterDim.Render(guide))
		}
		b.WriteString("\n")
	}
	if m.transcriptBrowse {
		line := transcriptBrowseFooterLine(m.tuiMouseScroll)
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(line))
		} else {
			b.WriteString(th.FooterDim.Render(line))
		}
		b.WriteString("\n")
	}

	if m.focusLine != nil {
		if fh := strings.TrimSpace(m.focusLine()); fh != "" {
			if fw > 4 {
				b.WriteString(th.FooterDim.Width(fw).Render(fh))
			} else {
				b.WriteString(th.FooterDim.Render(fh))
			}
			b.WriteString("\n")
		}
	}

	if strip := m.prefixSuggestStripView(); strip != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(strip)
	}
	if m.pending != nil {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m.approvalStripView())
	}
	if qs := m.messageQueueStripView(); qs != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(qs)
	}

	// Spacer before compose: only when the last footer segment did not already end with a newline
	// (avoids a double blank between idle stats and the input on a quiet footer).
	if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
		b.WriteString("\n")
	}

	inputView := m.composeInputView()
	if m.width > 4 {
		// Do not use Width() here: it pads the line with spaces to the terminal edge.
		inputView = th.InputBorder.Render(inputView)
	}
	b.WriteString(inputView)
	return b.String()
}

// prefixSuggestStripView shows @ path picks, / slash picks, or short ! / & hints above the input.
func (m *Model) prefixSuggestStripView() string {
	if m.toolLogOpen || m.docOverlayOpen || m.themePickOpen || m.agentPickOpen || m.streaming || m.pending != nil || m.transcriptBrowse {
		return ""
	}
	if s := m.atSuggestStripView(); s != "" {
		return s
	}
	if s := m.slashSuggestStripView(); s != "" {
		return s
	}
	return m.bangAmpHintStripView()
}

// atSuggestStripView lists workspace paths matching the @token at the current cursor position.
// Works regardless of where in the input the @ appears.
func (m *Model) atSuggestStripView() string {
	if strings.TrimSpace(m.workdir) == "" {
		return ""
	}
	row := m.input.Line()
	col := m.input.Column()
	lines := strings.Split(m.input.Value(), "\n")
	var currentLine string
	if row < len(lines) {
		currentLine = lines[row]
	}
	frag, _, ok := inputprefix.AtFragmentAtCursor(currentLine, col)
	if !ok {
		m.atSuggestLastWalk = time.Time{}
		return ""
	}
	now := time.Now()
	if !m.atSuggestLastWalk.IsZero() && now.Sub(m.atSuggestLastWalk) < atSuggestWalkMinInterval {
		return m.atSuggestLastOut
	}
	sugs := inputprefix.TUIAtPathSuggestions(m.workdir, frag)
	if len(sugs) == 0 {
		m.atSuggestLastWalk = now
		m.atSuggestLastOut = ""
		return ""
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	ruleW := m.width
	const maxPickRule = 52
	if ruleW > maxPickRule {
		ruleW = maxPickRule
	}
	maxW := m.width - 4
	if maxW < 40 {
		maxW = m.width
	}
	if maxW < 24 {
		maxW = 72
	}
	more := 0
	if len(sugs) > maxSlashSuggestRows {
		more = len(sugs) - maxSlashSuggestRows
		sugs = sugs[:maxSlashSuggestRows]
	}
	var b strings.Builder
	b.WriteString(th.SeparatorLine(ruleW))
	b.WriteString("\n")
	b.WriteString(th.SlashPickerDesc.Render(fmt.Sprintf("@ paths · max %d · Tab completes · workspace", maxSlashSuggestRows)))
	for _, s := range sugs {
		name := "@" + s.RelPath
		if s.IsDir {
			name += "/"
		}
		displayName := text.AtRefDisplayLabel(name)
		snippet := "dir"
		if !s.IsDir {
			snippet = "file"
		}
		nameW := lipgloss.Width(th.SlashPickerName.Render(displayName))
		budget := maxW - nameW - 2
		if budget < 8 {
			budget = 8
		}
		snippet = text.TruncateRunes(snippet, budget)
		line := lipgloss.JoinHorizontal(lipgloss.Top, th.SlashPickerName.Render(displayName), th.SlashPickerDesc.Render("  "+snippet))
		if lipgloss.Width(line) > maxW {
			line = th.SlashPickerName.Render(displayName)
		}
		b.WriteString("\n")
		b.WriteString(line)
	}
	if more > 0 {
		b.WriteString("\n")
		b.WriteString(th.SlashPickerDesc.Render(fmt.Sprintf("… +%d more — keep typing", more)))
	}
	out := b.String()
	m.atSuggestLastWalk = now
	m.atSuggestLastOut = out
	return out
}

// slashSuggestStripView renders filtered /commands or argument picks above the input (single-line buffer only).
func (m *Model) slashSuggestStripView() string {
	raw := m.input.Value()
	if strings.Contains(raw, "\n") {
		return ""
	}
	row := m.input.Line()
	col := m.input.Column()
	lines := strings.Split(raw, "\n")
	if row >= len(lines) {
		return ""
	}
	cur := lines[row]
	var sugs []slashcmd.SlashCommandSuggest
	var head string
	if m.slashContextFn != nil {
		sugs = slashcmd.SlashInlineSuggestions(m.ctx, m.slashContextFn(), cur, col)
		head = fmt.Sprintf("/ args · max %d · Tab · type to narrow", maxSlashSuggestRows)
	} else {
		sugs = slashcmd.TUISlashSuggestions(cur)
		head = fmt.Sprintf("/ commands · max %d shown · Tab · type to narrow", maxSlashSuggestRows)
	}
	if len(sugs) == 0 {
		return ""
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	ruleW := m.width
	const maxPickRule = 52
	if ruleW > maxPickRule {
		ruleW = maxPickRule
	}
	maxW := m.width - 4
	if maxW < 40 {
		maxW = m.width
	}
	if maxW < 24 {
		maxW = 72
	}
	more := 0
	if len(sugs) > maxSlashSuggestRows {
		more = len(sugs) - maxSlashSuggestRows
		sugs = sugs[:maxSlashSuggestRows]
	}
	var b strings.Builder
	b.WriteString(th.SeparatorLine(ruleW))
	b.WriteString("\n")
	b.WriteString(th.SlashPickerDesc.Render(head))
	for _, s := range sugs {
		displayName := slashSuggestDisplayName(s.Name)
		nameW := lipgloss.Width(th.SlashPickerName.Render(displayName))
		budget := maxW - nameW - 2
		if budget < 8 {
			budget = 8
		}
		snippet := text.TruncateRunes(s.Summary, budget)
		line := lipgloss.JoinHorizontal(lipgloss.Top, th.SlashPickerName.Render(displayName), th.SlashPickerDesc.Render("  "+snippet))
		if lipgloss.Width(line) > maxW {
			line = th.SlashPickerName.Render(displayName)
		}
		b.WriteString("\n")
		b.WriteString(line)
	}
	if more > 0 {
		b.WriteString("\n")
		b.WriteString(th.SlashPickerDesc.Render(fmt.Sprintf("… +%d more — keep typing", more)))
	}
	return b.String()
}

// bangAmpHintStripView shows one-line hints for ! and & prefix modes.
func (m *Model) bangAmpHintStripView() string {
	raw := strings.TrimSpace(m.input.Value())
	if strings.Contains(raw, "\n") || raw == "" {
		return ""
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	ruleW := m.width
	const maxPickRule = 52
	if ruleW > maxPickRule {
		ruleW = maxPickRule
	}
	var b strings.Builder
	switch {
	case raw == "!":
		b.WriteString(th.SeparatorLine(ruleW))
		b.WriteString("\n")
		b.WriteString(th.SlashPickerDesc.Render("! — bash tool (allowlisted) · type command · Enter runs · @ shows path picks"))
		return b.String()
	case raw == "&":
		b.WriteString(th.SeparatorLine(ruleW))
		b.WriteString("\n")
		b.WriteString(th.SlashPickerDesc.Render("& — spawn_agent (general-purpose) · one line · coordinator profile"))
		return b.String()
	default:
		return ""
	}
}

// slashSuggestDisplayName shortens workspace-like paths in the / picker without touching
// single-segment slash commands (/export, /theme, …).
func slashSuggestDisplayName(name string) string {
	rest := strings.TrimPrefix(name, "/")
	if !strings.Contains(rest, "/") && !strings.Contains(rest, `\`) {
		return name
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return text.AtRefDisplayLabel(name)
	}
	return name
}

const maxMessageQueueStripLines = 5

func queuePreviewOneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

// messageQueueStripView lists pending sends above the compose box (FIFO) while the model is busy.
func (m *Model) messageQueueStripView() string {
	n := len(m.messageQueue)
	if n == 0 {
		return ""
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	ruleW := m.width
	const maxPickRule = 52
	if ruleW > maxPickRule {
		ruleW = maxPickRule
	}
	fw := m.width
	if fw <= 0 {
		fw = defaultTerminalWidthFallback
	}
	maxW := fw - 4
	if maxW < 16 {
		maxW = fw
	}

	var b strings.Builder
	b.WriteString(th.SeparatorLine(ruleW))
	b.WriteString("\n")
	msgWord := "messages"
	if n == 1 {
		msgWord = "message"
	}
	b.WriteString(th.SlashPickerDesc.Render(fmt.Sprintf("Queued · %d %s · sent in order when idle", n, msgWord)))
	b.WriteString("\n")

	show := n
	extra := 0
	if show > maxMessageQueueStripLines {
		extra = show - maxMessageQueueStripLines
		show = maxMessageQueueStripLines
	}
	for i := 0; i < show; i++ {
		preview := queuePreviewOneLine(m.messageQueue[i])
		num := fmt.Sprintf("%d.", i+1)
		budget := maxW - utf8.RuneCountInString(num) - 1
		if budget < 8 {
			budget = 8
		}
		line := num + " " + text.TruncateRunes(preview, budget)
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(line))
		} else {
			b.WriteString(th.FooterDim.Render(line))
		}
		b.WriteString("\n")
	}
	if extra > 0 {
		more := fmt.Sprintf("… and %d more", extra)
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(more))
		} else {
			b.WriteString(th.FooterDim.Render(more))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// approvalStripView renders the tool approval request above the input with a card-style border.
func (m *Model) approvalStripView() string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	maxW := m.width - 6
	if maxW < 24 {
		maxW = m.width
	}
	if maxW < 24 {
		maxW = 72
	}
	toolPlain := m.pending.ToolName
	previewPlain := m.pending.Preview

	st := th.Icons
	title := th.ModalTitle.Render(st.ApprovalPromptGlyph() + " Allow")
	sep := th.ToolCardBorder.Render(" │ ")
	hint := th.Dim.Render("  y/n/esc")

	try := func(toolShow, prevShow string) (string, bool) {
		toolSt := th.Tool.Render(toolShow)
		body := th.ModalBody.Render(prevShow)
		s := title + " " + toolSt + sep + body + hint
		if lipgloss.Width(s) <= maxW {
			return s, true
		}
		return s, false
	}

	toolShow := toolPlain
	if lipgloss.Width(toolShow) > 24 {
		toolShow = text.TruncateRunes(toolShow, 20)
	}

	previewRunes := utf8.RuneCountInString(previewPlain)
	if previewRunes > approvalOverlayPreviewMaxRunes {
		previewRunes = approvalOverlayPreviewMaxRunes
	}
	for previewRunes >= approvalOverlayPreviewMinRunes {
		prevShow := text.TruncateRunes(previewPlain, previewRunes)
		if s, ok := try(toolShow, prevShow); ok {
			return s
		}
		previewRunes -= approvalOverlayPreviewStep
	}
	if s, ok := try(toolShow, "…"); ok {
		return s
	}
	toolSt := th.Tool.Render(text.TruncateRunes(toolShow, 12))
	return title + " " + toolSt + hint
}
