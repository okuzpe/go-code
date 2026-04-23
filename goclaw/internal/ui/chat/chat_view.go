package chat

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/okuzpe/goclaw/internal/text"
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
		line = line + " · mode:" + mode
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

// headerView renders a single compact context bar for the current session.
func (m *Model) headerView() string {
	profile := strings.TrimSpace(m.activeAgentProfile)
	model := strings.TrimSpace(m.modelLabel)
	if profile == "" && model == "" {
		return ""
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	w := m.width
	if w <= 0 {
		w = defaultTerminalWidthFallback
	}

	const brandText = "GOCLAW"
	partsPlain := []string{brandText}
	partsRendered := []string{th.HeaderBrand.Render(brandText)}

	if profile != "" {
		partsPlain = append(partsPlain, profile)
		partsRendered = append(partsRendered, th.SlashPickerName.Render(profile))
	}
	if model != "" {
		used := lipgloss.Width(strings.Join(partsPlain, " · "))
		remain := w - used - lipgloss.Width(" · ")
		if remain > 12 {
			model = text.TruncateRunes(model, remain)
			partsPlain = append(partsPlain, model)
			partsRendered = append(partsRendered, th.HeaderMeta.Render(model))
		}
	}

	row := strings.Join(partsRendered, " "+th.ShellChrome.Render("·")+" ")
	if lipgloss.Width(strings.Join(partsPlain, " · ")) >= w {
		return row
	}
	return row
}

func (m *Model) overlayFooterLine(line string) string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	if m.width > 4 {
		return th.OverlayHint.Width(m.width).Render(line)
	}
	return th.OverlayHint.Render(line)
}

func (m *Model) footerContextLine(fw int) string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	active := strings.TrimSpace(m.footerPrimaryStatus())
	stats := strings.TrimSpace(m.footerStatsLine)
	if active != "" {
		if stats != "" && fw > 24 {
			plainW := lipgloss.Width(stripANSI(active + " " + stats))
			if plainW <= fw {
				return active + " " + th.OverlayHint.Render("· " + stats)
			}
		}
		return active
	}
	if fh := strings.TrimSpace(m.footerHint); fh != "" {
		return fh
	}
	if guide := strings.TrimSpace(m.footerTranscriptGuideLine(fw)); guide != "" {
		return guide
	}
	if m.transcriptBrowse {
		return transcriptBrowseFooterLine(m.tuiMouseScroll)
	}
	if m.focusLine != nil {
		if fh := strings.TrimSpace(m.focusLine()); fh != "" {
			return fh
		}
	}
	if stats != "" {
		return stats
	}
	return "/ commands · @ files · Ctrl+P profile · Ctrl+T tools"
}

func (m *Model) renderDocOverlay() string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	var b strings.Builder
	title := strings.TrimSpace(m.docOverlayTitle)
	if title != "" {
		b.WriteString(th.OverlayTitle.Render(title))
		b.WriteString("\n\n")
	}
	if src := strings.TrimSpace(m.docOverlaySourceMD); src != "" {
		b.WriteString(th.RenderMarkdown(src, m.width, 0))
	}
	return b.String()
}

// footerPrimaryStatus delegates entirely to the centralized RenderAgentStatus().
// All state-to-text branching lives in agent_state.go; this function is now just a thin adapter.
func (m *Model) footerPrimaryStatus() string {
	// Gather thinking elapsed from the transcript meta slot.
	thinkingElapsed := 0
	if m.thinkingLineIdx >= 0 && m.thinkingLineIdx < len(m.lineMeta) &&
		m.lineMeta[m.thinkingLineIdx].kind == lineKindThinking &&
		!m.lineMeta[m.thinkingLineIdx].startedAt.IsZero() {
		thinkingElapsed = int(time.Since(m.lineMeta[m.thinkingLineIdx].startedAt).Seconds())
	}

	// Gather tool status.
	var toolLabel, toolSummary string
	toolElapsed := 0
	if len(m.toolWaitQueue) > 0 {
		job := m.toolWaitQueue[0]
		toolLabel = job.name
		toolSummary = job.summary
		if !m.toolWaitStartedAt.IsZero() {
			toolElapsed = int(time.Since(m.toolWaitStartedAt).Seconds())
		}
	}

	return RenderAgentStatus(
		m.theme,
		m.agentState,
		m.spinner.View(),
		m.lastThinkingPhase,
		thinkingElapsed,
		toolLabel,
		toolSummary,
		toolElapsed,
		m.curAssistant.Len(),
		m.lastAgentError,
	)
}

func (m *Model) View() tea.View {
	// Keep viewport height in sync with the footer on every frame. Footer line count changes when
	// the spinner/status row appears or disappears; if we only relied on layout() from resize/typing,
	// the transcript viewport could be sized for the wrong footer and clip or overlap the UI.
	m.layout()
	var content string
	header := m.headerRendered
	if strings.TrimSpace(m.profileBarRendered) != "" {
		content = lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.viewport.View(),
			m.profileBarRendered,
			m.footerRendered,
		)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left,
			header,
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

// profileModeBarView returns the single-line profile rail between the transcript and footer (see agent_deck_rail.go).
func (m *Model) profileModeBarView() string {
	return ""
}

func (m *Model) layout() {
	m.syncInputComposeWidth()
	header := m.headerView()
	m.headerRendered = header
	headerH := lipgloss.Height(header)
	m.profileBarRendered = m.profileModeBarView()
	barH := lipgloss.Height(m.profileBarRendered)
	foot := m.footerView()
	m.footerRendered = foot
	footerH := lipgloss.Height(foot)
	h := m.height - headerH - footerH - barH
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
		rendered := m.renderDocOverlay()
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
		return m.overlayFooterLine(line)
	}
	if m.docOverlayOpen {
		return m.overlayFooterLine("Esc · Ctrl+C quit")
	}
	if m.themePickOpen {
		return m.overlayFooterLine("↑↓ · Enter apply · Esc cancel · Ctrl+C quit")
	}
	if m.agentPickOpen {
		return m.overlayFooterLine("↑↓ · Enter apply · Esc cancel · Ctrl+C quit")
	}

	fw := m.width
	var b strings.Builder

	// Horizontal rule separating transcript from footer chrome.
	if fw > 0 {
		ruler := th.Separator.Render(strings.Repeat("─", fw))
		b.WriteString(ruler)
		b.WriteString("\n")
	}

	if line := m.footerContextLine(fw); line != "" {
		if fw > 4 {
			b.WriteString(th.OverlayHint.Width(fw).Render(line))
		} else {
			b.WriteString(th.OverlayHint.Render(line))
		}
		b.WriteString("\n")
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

func (m *Model) compactHelperStrip(head string, rows []string) string {
	if len(rows) == 0 {
		return ""
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	fw := m.width
	if fw <= 0 {
		fw = defaultTerminalWidthFallback
	}
	maxW := fw - 4
	if maxW < 24 {
		maxW = fw
	}
	line := head
	if len(rows) > 0 {
		line += " · " + strings.Join(rows, "  ·  ")
	}
	line = text.TruncateRunes(line, maxW)
	return th.OverlayHint.Render(line)
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
	more := 0
	if len(sugs) > maxSlashSuggestRows {
		more = len(sugs) - maxSlashSuggestRows
		sugs = sugs[:maxSlashSuggestRows]
	}
	rows := make([]string, 0, len(sugs)+1)
	for _, s := range sugs {
		name := "@" + s.RelPath
		if s.IsDir {
			name += "/"
		}
		rows = append(rows, text.AtRefDisplayLabel(name))
	}
	if more > 0 {
		rows = append(rows, fmt.Sprintf("+%d more", more))
	}
	out := m.compactHelperStrip("Paths · Tab completes", rows)
	m.atSuggestLastWalk = now
	m.atSuggestLastOut = out
	return out
}

// slashSuggestStripView renders filtered /commands or :commands above the input (single-line buffer only).
func (m *Model) slashSuggestStripView() string {
	raw := m.input.Value()
	if strings.Contains(raw, "\n") {
		return ""
	}
	// Treat ':cmd' as '/cmd' for the purpose of suggestion rendering.
	trimmedRaw := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmedRaw, ":") && !strings.HasPrefix(trimmedRaw, "::") {
		raw = "/" + trimmedRaw[1:]
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
		head = "Command args"
	} else {
		sugs = slashcmd.TUISlashSuggestions(cur)
		head = "Commands"
	}
	if len(sugs) == 0 {
		return ""
	}
	more := 0
	if len(sugs) > maxSlashSuggestRows {
		more = len(sugs) - maxSlashSuggestRows
		sugs = sugs[:maxSlashSuggestRows]
	}
	rows := make([]string, 0, len(sugs)+1)
	for _, s := range sugs {
		rows = append(rows, slashSuggestDisplayName(s.Name))
	}
	if more > 0 {
		rows = append(rows, fmt.Sprintf("+%d more", more))
	}
	return m.compactHelperStrip(head, rows)
}

// bangAmpHintStripView shows one-line hints for ! and & prefix modes.
func (m *Model) bangAmpHintStripView() string {
	raw := strings.TrimSpace(m.input.Value())
	if strings.Contains(raw, "\n") || raw == "" {
		return ""
	}
	switch {
	case raw == "!":
		return m.compactHelperStrip("Shell", []string{"type command", "Enter runs", "@ inserts paths"})
	case raw == "&":
		return m.compactHelperStrip("Spawn", []string{"one-line task", "general-purpose worker", "best from coordinator"})
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
	msgWord := "messages"
	if n == 1 {
		msgWord = "message"
	}
	preview := queuePreviewOneLine(m.messageQueue[0])
	maxW := m.width - 28
	if maxW < 16 {
		maxW = 16
	}
	parts := []string{fmt.Sprintf("%d %s", n, msgWord), text.TruncateRunes(preview, maxW)}
	if n > 1 {
		parts = append(parts, fmt.Sprintf("+%d more", n-1))
	}
	return m.compactHelperStrip("Queued", parts)
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
	title := th.OverlayTitle.Render(st.ApprovalPromptGlyph() + " Allow")
	sep := th.ShellChrome.Render(" │ ")
	hint := th.OverlayHint.Render("  y/n/esc")

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
