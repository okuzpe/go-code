package chat

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/orchestrator"
)

// verticalPickerHandlers maps arrow-list overlay keys for theme and agent pickers.
type verticalPickerHandlers struct {
	moveUp   func()
	moveDown func()
	onEnter  func()
	onClose  func()
}

func (m *Model) dispatchVerticalPickerKey(key string, h verticalPickerHandlers) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		h.moveUp()
		return m, nil
	case "down":
		h.moveDown()
		return m, nil
	case "enter":
		h.onEnter()
		return m, nil
	case "esc":
		h.onClose()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case footerTickMsg:
		m.refreshThinkingTranscriptRow()
		m.refreshToolRunningTranscriptRows()
		m.refreshFooterStatsCache()
		m.layout()
		return m, footerStatsTickCmd()
	case spinner.TickMsg:
		if !m.spinnerActive {
			return m, nil
		}
		var sc tea.Cmd
		m.spinner, sc = m.spinner.Update(msg)
		return m, sc
	case tea.KeyMsg:
		if m.agentPickOpen {
			return m.dispatchVerticalPickerKey(msg.String(), verticalPickerHandlers{
				moveUp:   func() { m.moveAgentPickCursor(-1) },
				moveDown: func() { m.moveAgentPickCursor(1) },
				onEnter:  func() { m.applyAgentPick() },
				onClose:  func() { m.closeAgentPicker() },
			})
		}
		if m.themePickOpen {
			return m.dispatchVerticalPickerKey(msg.String(), verticalPickerHandlers{
				moveUp:   func() { m.moveThemePickCursor(-1) },
				moveDown: func() { m.moveThemePickCursor(1) },
				onEnter: func() {
					out := m.applyThemePick()
					if strings.TrimSpace(out) != "" {
						m.appendSystem(out)
					}
				},
				onClose: func() { m.closeThemePicker() },
			})
		}
		if m.toolLogOpen {
			if m.toolLogDetail {
				switch msg.String() {
				case "esc":
					m.toolLogDetail = false
					m.refreshToolLogOverlay()
					m.layout()
					m.viewport.GotoTop()
					return m, nil
				case "ctrl+c":
					return m, tea.Quit
				default:
					var vcmd tea.Cmd
					m.viewport, vcmd = m.viewport.Update(msg)
					return m, vcmd
				}
			}
			switch msg.String() {
			case "up":
				m.moveToolLogCursor(-1)
				return m, nil
			case "down":
				m.moveToolLogCursor(1)
				return m, nil
			case "enter":
				if m.toolLogCursor >= 0 && m.toolLogCursor < len(m.toolLog) {
					m.toolLogDetail = true
					m.refreshToolLogOverlay()
					m.layout()
					m.viewport.GotoTop()
				}
				return m, nil
			case "esc", "ctrl+t":
				m.closeToolLog()
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				var vcmd tea.Cmd
				m.viewport, vcmd = m.viewport.Update(msg)
				return m, vcmd
			}
		}
		if m.helpOpen {
			switch msg.String() {
			case "esc":
				m.closeHelpOverlay()
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				var vcmd tea.Cmd
				m.viewport, vcmd = m.viewport.Update(msg)
				return m, vcmd
			}
		}
		if mdl, cmd, handled := m.handleKeyString(msg); handled {
			return mdl, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncInputPlaceholder()
		m.rebuildWelcomeForWidth()
		m.reflowTitleSeparator()
		if m.toolLogOpen {
			m.refreshToolLogOverlay()
		}
		if m.themePickOpen {
			m.refreshThemePickOverlay()
		}
		if m.agentPickOpen {
			m.refreshAgentPickOverlay()
		}
		if m.width > 0 && m.width != m.lastReflowWidth {
			m.reflowTranscriptForWidth()
			m.lastReflowWidth = m.width
			m.lastTranscript = ""
		}
		m.layout()
		var vcmd tea.Cmd
		m.viewport, vcmd = m.viewport.Update(msg)
		return m, vcmd
	case approvalMsg:
		r := ApprovalRequest(msg)
		m.pending = &r
		return m, nil
	case assistantPlaceholderMsg:
		m.turnHadWorkspaceWrite = false
		m.footerStatsStreamAt = time.Time{}
		m.refreshFooterStatsCache()
		m.idleTranscriptHint = ""
		m.exitTranscriptBrowse()
		m.streaming = true
		m.assistantPlaceholder = true
		m.statusLine = ""
		m.lastThinkingPhase = ""
		m.curAssistant.Reset()
		m.curAssistantLineIdx = -1
		m.streamPaintAt = time.Time{}
		m.streamPaintSkip = 0
		m.assistantHoldLastPaint = time.Time{}
		m.clearAssistantRevealState()
		m.appendThinkingRow("")
		th := m.theme
		if th == nil {
			th = DefaultTheme()
		}
		m.spinner = spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(th.SpinnerAccentV2()),
		)
		m.spinnerActive = true
		return m, tea.Batch(func() tea.Msg { return m.spinner.Tick() }, footerStatsTickCmd())
	case assistantDeltaMsg:
		if !m.streaming {
			// Drop stray deltas after completion (e.g. race with batching); avoids a second assistant row.
			return m, nil
		}
		if m.assistantPlaceholder {
			m.clearThinkingLine()
			m.stripAssistantPlaceholderLine()
			m.assistantPlaceholder = false
			m.statusLine = ""
		}
		m.curAssistant.WriteString(string(msg))
		m.refreshAssistantStreamDisplay()
		// Throttle O(n) footer stats while tokens stream (footerStats scans session messages).
		if m.footerStats != nil {
			now := time.Now()
			if m.footerStatsStreamAt.IsZero() || now.Sub(m.footerStatsStreamAt) >= 250*time.Millisecond {
				m.footerStatsStreamAt = now
				m.refreshFooterStatsCache()
			}
		}
		return m, nil
	case assistantDoneMsg:
		m.refreshFooterStatsCache()
		m.streaming = false
		m.assistantPlaceholder = false
		m.spinnerActive = false
		m.statusLine = ""
		m.lastThinkingPhase = ""
		m.clearThinkingLine()
		rawAssistant := m.curAssistant.String()
		rawLen := utf8.RuneCountInString(rawAssistant)
		wantReveal := assistantStreamHold && !msg.aborted &&
			rawLen >= assistantRevealMinRunes && strings.TrimSpace(rawAssistant) != ""
		if wantReveal {
			m.assistantRevealRunes = []rune(rawAssistant)
			m.assistantRevealPos = 0
			m.pendingAssistantDone = msg
			m.pendingAssistantDoneSet = true
			m.pendingAssistantRaw = rawAssistant
			if m.curAssistantLineIdx < 0 || m.curAssistantLineIdx >= len(m.lines) {
				m.refreshAssistantHoldLine()
			}
			return m, assistantRevealTickCmd()
		}
		m.clearAssistantRevealState()
		m.finalizeCurrentSegment()
		return m, m.applyAssistantDoneFollowup(msg, rawAssistant, rawLen)
	case assistantRevealTickMsg:
		if len(m.assistantRevealRunes) == 0 || !m.pendingAssistantDoneSet {
			return m, nil
		}
		total := len(m.assistantRevealRunes)
		remain := total - m.assistantRevealPos
		chunk := assistantRevealChunkRunes
		if remain > 6000 {
			chunk = max(assistantRevealChunkRunes, remain/48)
		}
		next := m.assistantRevealPos + chunk
		if next > total {
			next = total
		}
		m.assistantRevealPos = next
		m.paintAssistantRevealPlain()
		if m.assistantRevealPos >= total {
			done := m.pendingAssistantDone
			raw := m.pendingAssistantRaw
			rawLen := utf8.RuneCountInString(raw)
			m.clearAssistantRevealState()
			m.finalizeCurrentSegment()
			return m, m.applyAssistantDoneFollowup(done, raw, rawLen)
		}
		return m, assistantRevealTickCmd()
	case toolUseMsg:
		m.clearThinkingLine()
		// Finalize the pre-tool text with markdown rendering BEFORE resetting.
		m.finalizeCurrentSegment()
		// Reset buffer for the next assistant segment (post-tool).
		m.curAssistant.Reset()
		m.curAssistantLineIdx = -1
		m.toolWaitQueue = append(m.toolWaitQueue, pendingTool{toolUseID: msg.toolUseID, name: msg.name, summary: msg.preview})
		if len(m.toolWaitQueue) == 1 {
			now := time.Now()
			m.toolWaitStartedAt = now
			m.toolLogStart = now
		}
		m.appendToolRunningTranscriptRow(msg.name, msg.preview)
		m.spinnerActive = true
		m.statusLine = m.toolQueueStatusLine()
		return m, tickToolWait()
	case toolTickMsg:
		if len(m.toolWaitQueue) == 0 {
			m.toolWaitStartedAt = time.Time{}
			return m, nil
		}
		m.refreshToolRunningTranscriptRows()
		m.statusLine = m.toolQueueStatusLine()
		return m, tickToolWait()
	case ctrlCExitArmExpiredMsg:
		if m.exitConfirmDeadline != msg.expected || m.exitConfirmDeadline.IsZero() {
			return m, nil
		}
		m.exitConfirmDeadline = time.Time{}
		if m.ctrlCMsgRendered != "" {
			for i := len(m.lines) - 1; i >= 0; i-- {
				if m.lines[i] == m.ctrlCMsgRendered {
					m.lines = append(m.lines[:i], m.lines[i+1:]...)
					if i < len(m.lineMeta) {
						m.lineMeta = append(m.lineMeta[:i], m.lineMeta[i+1:]...)
					}
					break
				}
			}
			m.ctrlCMsgRendered = ""
			m.setLinesContent(false)
		}
		m.layout()
		return m, nil
	case toolResultMsg:
		lineIdx, job := m.dequeueToolResult(msg.toolUseID, msg.name)
		if lineIdx >= 0 && lineIdx < len(m.lineMeta) && m.lineMeta[lineIdx].kind == lineKindToolRunning {
			m.replaceToolRunningWithCard(lineIdx, job.name, job.summary, msg.content, msg.isError)
		} else {
			m.appendToolDoneLine(job.name, job.summary, msg.content, msg.isError)
		}
		if len(m.toolWaitQueue) > 0 {
			m.toolWaitStartedAt = time.Now()
			m.statusLine = m.toolQueueStatusLine()
		} else {
			m.toolWaitStartedAt = time.Time{}
			m.statusLine = ""
		}
		if !msg.isError {
			switch msg.name {
			case "write_file", "edit_file", "patch":
				m.turnHadWorkspaceWrite = true
			}
		}
		if len(m.toolWaitQueue) > 0 {
			return m, tickToolWait()
		}
		return m, nil
	case thinkingPhaseMsg:
		phase := strings.TrimSpace(msg.phase)
		m.lastThinkingPhase = phase
		if m.thinkingLineIdx >= 0 && m.thinkingLineIdx < len(m.lineMeta) && m.lineMeta[m.thinkingLineIdx].kind == lineKindThinking {
			m.lineMeta[m.thinkingLineIdx].thinkingLabel = phase
			m.refreshThinkingTranscriptRow()
		}
		return m, nil
	case thinkingRestartMsg:
		// Re-show thinking row between tool iterations (mid-turn LLM call).
		if m.streaming {
			m.assistantPlaceholder = true
			m.appendThinkingRow(strings.TrimSpace(msg.phase))
		}
		return m, nil
	case toolProgressMsg:
		// Update the live in-progress card for the named tool. Uses msg.name to find the
		// correct card so parallel tool runs each update their own row.
		lineIdx := m.findToolRunLineByName(msg.name)
		if lineIdx >= 0 && lineIdx < len(m.lines) && lineIdx < len(m.lineMeta) &&
			m.lineMeta[lineIdx].kind == lineKindToolRunning {
			meta := m.lineMeta[lineIdx]
			// Append progress chunks (bash: one line per chunk; glob/grep: batched paths split on \n).
			prev := meta.toolSummary
			lines := strings.Split(strings.TrimRight(prev, "\n"), "\n")
			for _, piece := range strings.Split(msg.partial, "\n") {
				if t := strings.TrimSpace(piece); t != "" {
					lines = append(lines, t)
				}
			}
			maxProg := 3
			if meta.toolName == "glob" || meta.toolName == "grep" {
				maxProg = 16
			}
			if len(lines) > maxProg {
				lines = lines[len(lines)-maxProg:]
			}
			newSummary := strings.Join(lines, "\n")
			meta.toolSummary = newSummary
			m.lineMeta[lineIdx] = meta
			th := m.theme
			if th == nil {
				th = DefaultTheme()
			}
			label := orchestrator.ToolWorkingPhrase(meta.toolName)
			elapsed := 0
			if len(m.toolWaitQueue) > 0 && !m.toolWaitStartedAt.IsZero() {
				elapsed = int(time.Since(m.toolWaitStartedAt).Seconds())
			}
			m.lines[lineIdx] = th.RenderToolInProgressRow(label, newSummary, elapsed, m.widthOrDefault())
			m.setLinesContent(false)
		}
		return m, nil
	case compactNoticeMsg:
		m.appendSystem(fmt.Sprintf("⟳ Context compacted — %d message(s) folded into summary", msg.removed))
		m.refreshFooterStatsCache()
		m.layout()
		return m, nil
	case errMsg:
		m.clearAssistantRevealState()
		m.refreshFooterStatsCache()
		m.exitTranscriptBrowse()
		m.streaming = false
		m.assistantPlaceholder = false
		m.spinnerActive = false
		m.statusLine = ""
		m.clearThinkingLine()
		m.turnHadWorkspaceWrite = false
		m.toolRunLineIdx = nil
		m.toolWaitQueue = nil
		if n := len(m.messageQueue); n > 0 {
			m.messageQueue = nil
			m.appendSystem(fmt.Sprintf("(dropped %d queued message(s) after error)", n))
		}
		m.idleTranscriptHint = ""
		m.footerHint = ""
		m.stripAssistantPlaceholderLine()
		eth := m.theme
		if eth == nil {
			eth = DefaultTheme()
		}
		m.appendError(fmt.Sprintf("%s %v", eth.Icons.ToolErr(), msg.err))
		m.curAssistantLineIdx = -1
		return m, nil
	}

	// Windows CRLF: bubbles maps CR and LF to LF separately, so "\r\n" becomes "\n\n".
	if p, ok := msg.(tea.PasteMsg); ok {
		n := inputprefix.NormalizePasteNewlines(p.Content)
		if n != p.Content {
			msg = tea.PasteMsg{Content: n}
		}
	}

	// Drag-and-drop / bracketed paste: file/dir paths from the OS → @relpath tokens in the compose box.
	// Allowed while streaming so mid-reply file drops still work; arbitrary paste while streaming
	// skips this block and is handled below (textarea).
	if paste, isPaste := msg.(tea.PasteMsg); isPaste &&
		m.pending == nil &&
		!m.toolLogOpen && !m.helpOpen && !m.themePickOpen && !m.agentPickOpen {
		if m.transcriptBrowse {
			m.exitTranscriptBrowse()
			m.layout()
		}
		if tokens, ok := inputprefix.TryPasteAsAtPaths(m.workdir, paste.Content); ok {
			// Insert a space before the token if cursor is right after non-space text.
			row := m.input.Line()
			col := m.input.Column()
			allLines := strings.Split(m.input.Value(), "\n")
			prefix := ""
			if row < len(allLines) {
				lineRunes := []rune(allLines[row])
				if col > 0 && col <= len(lineRunes) && lineRunes[col-1] != ' ' && lineRunes[col-1] != '\t' {
					prefix = " "
				}
			}
			m.input.InsertString(prefix + tokens + " ")
			m.resizeInput()
			m.layout()
			return m, nil
		}
	}

	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	// Mouse wheel: with cell mouse mode, scroll the widget under the cursor — compose
	// (last rows) scrolls the textarea; transcript or footer chrome scrolls the transcript.
	if mw, ok := msg.(tea.MouseWheelMsg); ok && m.tuiMouseScroll &&
		!m.toolLogOpen && !m.helpOpen && !m.themePickOpen && !m.agentPickOpen {
		m.layout()
		if m.transcriptBrowse {
			m.viewport, cmd = m.viewport.Update(msg)
			m.resizeInput()
			return m, cmd
		}
		transcriptRows := m.viewport.Height()
		if transcriptRows < 1 {
			transcriptRows = 1
		}
		th := m.theme
		if th == nil {
			th = DefaultTheme()
		}
		composeView := m.composeInputView()
		if m.width > 4 {
			composeView = th.InputBorder.Render(composeView)
		}
		composeLines := lipgloss.Height(composeView)
		if composeLines < 1 {
			composeLines = 1
		}
		if composeLines > m.height {
			composeLines = m.height
		}
		composeTopY := m.height - composeLines
		if mw.Y >= composeTopY {
			m.input, cmd = m.input.Update(msg)
			m.resizeInput()
			return m, cmd
		}
		if mw.Y < transcriptRows {
			m.viewport, cmd = m.viewport.Update(msg)
			m.resizeInput()
			return m, cmd
		}
		m.viewport, cmd = m.viewport.Update(msg)
		m.resizeInput()
		return m, cmd
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	if !m.toolLogOpen && !m.helpOpen && !m.themePickOpen && !m.agentPickOpen {
		if keyMsg, ok := msg.(tea.KeyMsg); ok && composeTranscriptScrollKey(keyMsg.String()) {
			m.resizeInput()
			return m, tea.Batch(cmds...)
		}
		if _, ok := msg.(tea.KeyMsg); ok && m.transcriptBrowse {
			// Browse mode: pager keys go to the viewport; do not forward keys to the blurred textarea.
			m.resizeInput()
			return m, tea.Batch(cmds...)
		}
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
		// Dynamic input height: grow textarea as user types more lines (up to inputMaxHeight).
		m.resizeInput()
	}

	return m, tea.Batch(cmds...)
}
