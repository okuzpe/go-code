package chat

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/slashcmd"
)

func (m *Model) exitTranscriptBrowse() {
	if !m.transcriptBrowse {
		return
	}
	m.transcriptBrowse = false
	m.browseToolCardLine = -1
	m.input.Focus()
	m.syncViewportKeyMapForCompose()
}

func (m *Model) enterTranscriptBrowse() {
	if m.transcriptBrowse {
		return
	}
	m.transcriptBrowse = true
	ids := m.toolCardLineIndices()
	if len(ids) > 0 {
		m.browseToolCardLine = ids[len(ids)-1]
	} else {
		m.browseToolCardLine = -1
	}
	m.input.Blur()
	m.syncViewportKeyMapForCompose()
}

// composeViewportKeyMap avoids stealing Space, arrows, j/k/h/l, u, d, or b from the compose textarea.
// Half-page keys avoid ctrl+u / ctrl+d (textarea delete before/after cursor). alt+pg* helps when PgUp
// is not delivered by the host terminal. Plain PgUp/PgDn are routed to the transcript only (see Update).
// While streaming with an empty compose box, plain ↑/↓ (and j/k) still scroll the transcript via
// tryBusyComposeTranscriptScroll in handleKeyString so users can read history during a reply.
func composeViewportKeyMap() viewport.KeyMap {
	km := viewport.DefaultKeyMap()
	km.PageUp = key.NewBinding(
		key.WithKeys("pgup", "shift+pgup", "alt+pgup"),
		key.WithHelp("pgup", "page up"),
	)
	km.PageDown = key.NewBinding(
		key.WithKeys("pgdown", "shift+pgdown", "alt+pgdown"),
		key.WithHelp("pgdn", "page down"),
	)
	km.HalfPageUp = key.NewBinding(key.WithKeys("ctrl+shift+u"), key.WithHelp("ctrl+shift+u", "½ page up"))
	km.HalfPageDown = key.NewBinding(key.WithKeys("ctrl+shift+d"), key.WithHelp("ctrl+shift+d", "½ page down"))
	km.Up = key.NewBinding(key.WithKeys("alt+up"), key.WithHelp("alt+↑", "scroll up"))
	km.Down = key.NewBinding(key.WithKeys("alt+down"), key.WithHelp("alt+↓", "scroll down"))
	km.Left = key.NewBinding(key.WithDisabled())
	km.Right = key.NewBinding(key.WithDisabled())
	return km
}

// composeTranscriptScrollKey reports keys handled by the transcript viewport in compose mode that
// must not reach the textarea (otherwise PgUp/PgDn move the editor cursor instead of scrolling).
func composeTranscriptScrollKey(k string) bool {
	switch k {
	case "pgup", "pgdown",
		"shift+pgup", "shift+pgdown",
		"alt+pgup", "alt+pgdown",
		"alt+up", "alt+down",
		"ctrl+shift+u", "ctrl+shift+d":
		return true
	default:
		return false
	}
}

// tryBusyComposeTranscriptScroll scrolls the transcript when the model is working (streaming or
// spinner) and the compose box is empty. Plain arrows are otherwise unused by the viewport keymap
// in compose mode and would only move the textarea cursor or repl history.
func (m *Model) tryBusyComposeTranscriptScroll(k string) bool {
	if m.transcriptBrowse {
		return false
	}
	if !(m.streaming || m.spinnerActive) {
		return false
	}
	if strings.TrimSpace(m.input.Value()) != "" {
		return false
	}
	switch k {
	case "up", "k":
		m.viewport.ScrollUp(1)
		return true
	case "down", "j":
		m.viewport.ScrollDown(1)
		return true
	default:
		return false
	}
}

// overlayViewportKeyMap restores pager motion for full-screen overlays (help, pickers, tool detail).
// Space is still not page-down so paste/typing in nested fields never scrolls the transcript.
func overlayViewportKeyMap() viewport.KeyMap {
	km := viewport.DefaultKeyMap()
	km.PageDown = key.NewBinding(key.WithKeys("pgdown", "f"), key.WithHelp("f/pgdn", "page down"))
	return km
}

func (m *Model) syncViewportKeyMapForCompose() {
	if m.transcriptBrowse {
		m.viewport.KeyMap = transcriptBrowseViewportKeyMap()
	} else {
		m.viewport.KeyMap = composeViewportKeyMap()
	}
}

// transcriptBrowseViewportKeyMap restores pager keys (arrows, j/k) while the compose textarea is blurred.
func transcriptBrowseViewportKeyMap() viewport.KeyMap {
	return viewport.DefaultKeyMap()
}

func (m *Model) syncViewportKeyMapForOverlay() {
	m.viewport.KeyMap = overlayViewportKeyMap()
}

func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.approval != nil {
		cmds = append(cmds, waitForApproval(m.approval.Requests))
	}
	// Periodic tick refreshes footer stats when configured and updates thinking / IN-flight tool rows in the transcript.
	cmds = append(cmds, footerStatsTickCmd())
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func tickToolWait() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return toolTickMsg{} })
}

// enterWithNewlineModifiers is true when Enter/Return should insert a newline in the
// compose textarea instead of submitting. Some hosts (notably Windows) report
// Shift+Enter as KeyEnter with ModShift set while msg.String() is still "enter".
func enterWithNewlineModifiers(msg tea.KeyMsg) bool {
	k := msg.Key()
	if k.Code != tea.KeyEnter && k.Code != tea.KeyReturn {
		return false
	}
	return k.Mod&(tea.ModShift|tea.ModAlt) != 0
}

// ctrlCExitArmExpiredMsg clears the exit-confirm window when the scheduled deadline passes.
// expected must match m.exitConfirmDeadline so restarts of the 3s window ignore stale ticks.
type ctrlCExitArmExpiredMsg struct {
	expected time.Time
}

// clearTranscriptLikeCtrlL clears the in-memory transcript (same as Ctrl+L in the TUI).
func (m *Model) clearTranscriptLikeCtrlL() {
	m.exitTranscriptBrowse()
	m.lines = nil
	m.lineMeta = nil
	m.toolWaitQueue = nil
	m.assistantPlaceholder = false
	m.spinnerActive = false
	m.curAssistantLineIdx = -1
	m.idleTranscriptHint = ""
	m.footerHint = ""
	m.setLinesContent(true)
}

func (m *Model) handleKeyString(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	k := msg.String()
	if m.pending != nil {
		switch k {
		case "y", "Y":
			select {
			case m.pending.Resp <- true:
			default:
			}
			m.pending = nil
			return m, waitForApproval(m.approval.Requests), true
		case "enter":
			if enterWithNewlineModifiers(msg) {
				// Shift/Alt+Enter is newline in compose, not approval (Windows reports ModShift on Enter).
				return m, nil, false
			}
			select {
			case m.pending.Resp <- true:
			default:
			}
			m.pending = nil
			return m, waitForApproval(m.approval.Requests), true
		case "n", "N", "esc", "ctrl+c":
			select {
			case m.pending.Resp <- false:
			default:
			}
			m.pending = nil
			return m, waitForApproval(m.approval.Requests), true
		default:
			// Modal approval: swallow most keys, but allow newline chords to reach the compose textarea.
			if k == "shift+enter" || k == "alt+enter" {
				return m, nil, false
			}
			return m, nil, true
		}
	}
	if k == "ctrl+c" {
		return m.handleCtrlCExitConfirm()
	}
	if k == "esc" {
		// Match common agent UIs: Esc stops an in-flight reply; Esc again exits when idle.
		if m.streaming {
			m.submitter.cancel()
			return m, nil, true
		}
		if m.transcriptBrowse {
			m.exitTranscriptBrowse()
			m.layout()
			return m, nil, true
		}
		return m, tea.Quit, true
	}
	if m.transcriptBrowse {
		switch k {
		case "[":
			m.cycleBrowseToolCardFocus(-1)
			m.reflowTranscriptForWidth()
			m.setLinesContent(false)
			m.layout()
			return m, nil, true
		case "]":
			m.cycleBrowseToolCardFocus(1)
			m.reflowTranscriptForWidth()
			m.setLinesContent(false)
			m.layout()
			return m, nil, true
		case "e":
			m.toggleBrowseToolCardExpanded()
			m.reflowTranscriptForWidth()
			m.setLinesContent(false)
			m.layout()
			return m, nil, true
		}
	}
	switch k {
	case "up":
		if m.transcriptBrowse {
			return m, nil, false
		}
		if m.tryBusyComposeTranscriptScroll("up") {
			return m, nil, true
		}
		if m.replHistoryNavUp() {
			m.resizeInput()
			m.layout()
			return m, nil, true
		}
		return m, nil, false
	case "down":
		if m.transcriptBrowse {
			return m, nil, false
		}
		if m.tryBusyComposeTranscriptScroll("down") {
			return m, nil, true
		}
		if m.replHistoryNavDown() {
			m.resizeInput()
			m.layout()
			return m, nil, true
		}
		return m, nil, false
	case "k":
		if m.transcriptBrowse {
			return m, nil, false
		}
		if m.tryBusyComposeTranscriptScroll("k") {
			return m, nil, true
		}
		return m, nil, false
	case "j":
		if m.transcriptBrowse {
			return m, nil, false
		}
		if m.tryBusyComposeTranscriptScroll("j") {
			return m, nil, true
		}
		return m, nil, false
	case "ctrl+b":
		if m.pending != nil {
			return m, nil, true
		}
		if m.transcriptBrowse {
			m.exitTranscriptBrowse()
		} else {
			m.enterTranscriptBrowse()
		}
		m.layout()
		return m, nil, true
	case "ctrl+l":
		m.clearTranscriptLikeCtrlL()
		return m, nil, true
	case "ctrl+shift+right", "ctrl+shift+left":
		if m.cycleAgentProfileFn == nil {
			return m, nil, false
		}
		if m.streaming || m.pending != nil || m.transcriptBrowse ||
			m.toolLogOpen || m.docOverlayOpen || m.themePickOpen || m.agentPickOpen {
			return m, nil, true
		}
		backward := k == "ctrl+shift+left"
		hints, err := m.cycleAgentProfileFn(backward)
		if err != nil {
			m.appendError(fmt.Sprintf("Profile cycle: %v", err))
		} else {
			m.applySlashHints(hints)
		}
		m.layout()
		return m, nil, true
	case "tab":
		if m.transcriptBrowse {
			m.exitTranscriptBrowse()
			m.layout()
		}
		if m.streaming || m.pending != nil {
			return m, nil, false
		}
		// @ completion: find the @token at cursor and replace only that token.
		{
			row := m.input.Line()
			col := m.input.Column()
			allLines := strings.Split(m.input.Value(), "\n")
			var curLine string
			if row < len(allLines) {
				curLine = allLines[row]
			}
			if frag, startCol, fragOK := inputprefix.AtFragmentAtCursor(curLine, col); fragOK {
				if repl, ok := inputprefix.AtTabExpand(strings.TrimSpace(m.workdir), frag); ok {
					replRunes := []rune(repl)
					lineRunes := []rune(curLine)
					newLine := string(lineRunes[:startCol]) + repl + string(lineRunes[col:])
					allLines[row] = newLine
					newValue := strings.Join(allLines, "\n")
					newCursorCol := startCol + len(replRunes)
					lastRow := len(allLines) - 1
					m.input.SetValue(newValue)
					// SetValue leaves cursor at end of last line; navigate back if needed.
					for i := 0; i < lastRow-row; i++ {
						m.input.CursorUp()
					}
					m.input.SetCursorColumn(newCursorCol)
					m.resizeInput()
					m.layout()
					return m, nil, true
				}
			}
		}
		// Slash argument Tab (single-line buffer only; live SlashContext).
		if m.slashContextFn != nil {
			row := m.input.Line()
			col := m.input.Column()
			allLines := strings.Split(m.input.Value(), "\n")
			if row < len(allLines) && !strings.Contains(m.input.Value(), "\n") {
				curLine := allLines[row]
				if newLine, newCol, ok := slashcmd.SlashArgTabExpand(m.ctx, m.slashContextFn(), curLine, col); ok {
					allLines[row] = newLine
					newValue := strings.Join(allLines, "\n")
					lastRow := len(allLines) - 1
					m.input.SetValue(newValue)
					for i := 0; i < lastRow-row; i++ {
						m.input.CursorUp()
					}
					m.input.SetCursorColumn(newCol)
					m.resizeInput()
					m.layout()
					return m, nil, true
				}
			}
		}
		// Slash command Tab expand operates on the full single-line input.
		raw := m.input.Value()
		if repl, ok := slashcmd.SlashTabExpand(raw); ok {
			m.input.SetValue(repl)
			m.input.CursorEnd()
			m.resizeInput()
			m.layout()
			return m, nil, true
		}
		return m, nil, false
	case "ctrl+t", "ctrl+e":
		if m.streaming || m.pending != nil {
			return m, nil, true
		}
		m.openToolLog()
		return m, nil, true
	case "ctrl+m":
		if m.streaming || m.pending != nil || m.transcriptBrowse ||
			m.toolLogOpen || m.docOverlayOpen || m.themePickOpen || m.agentPickOpen {
			return m, nil, true
		}
		m.interactMode = config.CycleTUIInteractMode(m.interactMode)
		m.refreshFooterStatsCache()
		m.layout()
		return m, nil, true
	case "ctrl+p":
		if m.streaming || m.pending != nil {
			return m, nil, true
		}
		m.openAgentPicker()
		return m, nil, true
	case "enter":
		if enterWithNewlineModifiers(msg) {
			return m, nil, false
		}
		if m.transcriptBrowse {
			m.exitTranscriptBrowse()
			m.layout()
		}
		txt := strings.TrimSpace(m.input.Value())
		if txt == "" {
			return m, nil, true
		}
		// Pasting the footer stats line (e.g. when copying the whole TUI) queues garbage; reject if it matches exactly.
		if m.streaming {
			if fs := strings.TrimSpace(m.footerStatsLine); fs != "" && txt == fs {
				m.footerHint = "Not queued — that text is the footer stats line, not your message."
				return m, nil, true
			}
		}
		m.footerHint = ""
		m.idleTranscriptHint = ""
		m.replHistPos = 0
		m.replHistDraft = ""
		m.input.Reset()
		m.resizeInput() // shrink back to 1 line after submit
		if m.streaming {
			// Keep pending sends out of the transcript until this turn finishes; list them above the input.
			m.messageQueue = append(m.messageQueue, txt)
			m.layout()
			return m, nil, true
		}
		m.appendSeparator()
		m.appendUser(txt)
		if cmd := m.runDispatchAfterUserEcho(txt); cmd != nil {
			return m, cmd, true
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m *Model) handleCtrlCExitConfirm() (tea.Model, tea.Cmd, bool) {
	if m.streaming {
		m.submitter.cancel()
	}
	now := time.Now()
	if !m.exitConfirmDeadline.IsZero() && now.Before(m.exitConfirmDeadline) {
		m.exitConfirmDeadline = time.Time{}
		m.ctrlCMsgRendered = ""
		return m, tea.Quit, true
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	expected := now.Add(3 * time.Second)
	m.exitConfirmDeadline = expected
	m.ctrlCMsgRendered = th.System.Render("Press Ctrl+C again within 3 seconds to quit.")
	m.lines = append(m.lines, m.ctrlCMsgRendered)
	m.appendPlainMeta()
	m.setLinesContent(true)
	m.layout()
	return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return ctrlCExitArmExpiredMsg{expected: expected}
	}), true
}
