package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/replhistory"
	"github.com/okuzpe/goclaw/internal/slashcmd"
)

type submitRunner struct {
	fn         func(userText, interactMode string)
	mu         sync.Mutex
	cancelLast context.CancelFunc
}

func (r *submitRunner) setCancel(fn context.CancelFunc) {
	r.mu.Lock()
	r.cancelLast = fn
	r.mu.Unlock()
}

func (r *submitRunner) cancel() {
	r.mu.Lock()
	fn := r.cancelLast
	r.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// runDispatchAfterUserEcho runs theme/agents/slash/model for a user line already appended to the transcript.
// Returns tea.Quit when a slash handler requests exit (same as the Enter path).
func (m *Model) runDispatchAfterUserEcho(txt string) tea.Cmd {
	if bareThemeSlashInput(txt) && strings.TrimSpace(m.userConfigDir) != "" {
		m.openThemePicker()
		return nil
	}
	if (bareAgentsSlashInput(txt) || bareProfileSlashInput(txt)) && m.slashHandle != nil {
		m.openAgentPicker()
		return nil
	}
	if bareToolsSlashInput(txt) && len(m.toolLog) > 0 {
		m.openToolLog()
		return nil
	}
	if m.slashHandle != nil {
		handled, out, quit, modelSubmit, hints, err := m.slashHandle(txt)
		if handled {
			if err != nil {
				m.appendError(fmt.Sprintf("error: %v", err))
			} else {
				if hints.TUIClearTranscript {
					m.clearTranscriptLikeCtrlL()
				}
				if strings.TrimSpace(out) != "" {
					if hints.TUIDocOverlay {
						m.openDocOverlay(hints.TUIDocTitle, out)
					} else if slashcmd.PlainHelpREPLRequest(txt) {
						m.openHelpDocOverlay(out)
					} else {
						m.appendSystem(out)
					}
				}
			}
			m.applySlashHints(hints)
			if len(hints.ModelSubmitQueue) > 0 {
				for i := len(hints.ModelSubmitQueue) - 1; i >= 0; i-- {
					q := strings.TrimSpace(hints.ModelSubmitQueue[i])
					if q != "" {
						m.messageQueue = append([]string{q}, m.messageQueue...)
					}
				}
			}
			if strings.TrimSpace(modelSubmit) != "" && m.submitter != nil && m.submitter.fn != nil {
				m.runModelSubmit(modelSubmit, m.interactMode)
			}
			if quit {
				return tea.Quit
			}
			return nil
		}
	}
	if m.submitter != nil && m.submitter.fn != nil {
		m.runModelSubmit(txt, m.interactMode)
	} else {
		m.appendAssistant("(TUI wired; missing submit handler)")
	}
	return nil
}

func (m *Model) drainMessageQueue() tea.Cmd {
	for len(m.messageQueue) > 0 && !m.streaming {
		txt := m.messageQueue[0]
		m.messageQueue = m.messageQueue[1:]
		m.appendSeparator()
		m.appendUser(txt)
		if cmd := m.runDispatchAfterUserEcho(txt); cmd != nil {
			return cmd
		}
	}
	return nil
}

func (m *Model) replHistoryNavUp() bool {
	if m.streaming || m.pending != nil || len(m.replLines) == 0 {
		return false
	}
	if strings.Contains(m.input.Value(), "\n") {
		return false
	}
	if m.replHistPos == 0 {
		m.replHistDraft = m.input.Value()
	}
	if m.replHistPos >= len(m.replLines) {
		return true
	}
	m.replHistPos++
	idx := len(m.replLines) - m.replHistPos
	m.input.SetValue(m.replLines[idx])
	m.input.CursorEnd()
	return true
}

func (m *Model) replHistoryNavDown() bool {
	if m.replHistPos == 0 {
		return false
	}
	m.replHistPos--
	if m.replHistPos == 0 {
		m.input.SetValue(m.replHistDraft)
		m.replHistDraft = ""
		m.input.CursorEnd()
		return true
	}
	idx := len(m.replLines) - m.replHistPos
	if idx >= 0 && idx < len(m.replLines) {
		m.input.SetValue(m.replLines[idx])
		m.input.CursorEnd()
	}
	return true
}

func (m *Model) runModelSubmit(userText, interactMode string) {
	ut := strings.TrimSpace(userText)
	if d := strings.TrimSpace(m.userConfigDir); d != "" && ut != "" {
		_ = replhistory.Append(d, ut)
		m.replLines = append(m.replLines, ut)
		if len(m.replLines) > replhistory.MaxLines {
			m.replLines = m.replLines[len(m.replLines)-replhistory.MaxLines:]
		}
	}
	m.replHistPos = 0
	m.replHistDraft = ""
	m.streaming = true
	m.agentState = AgentStateIdle
	m.lastAgentError = ""
	m.statusLine = ""
	m.lastThinkingPhase = ""
	m.curAssistant.Reset()
	m.curAssistantLineIdx = -1
	m.toolRunLineIdx = nil
	m.clearThinkingLine()
	m.submitter.fn(userText, config.NormalizeTUIInteractMode(interactMode))
}
