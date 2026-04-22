package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/okuzpe/goclaw/internal/agentdemo/components"
	"github.com/okuzpe/goclaw/internal/replhistory"
)

func (m *Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case spinner.TickMsg:
		if m.spinnerRunning() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case streamDeltaMsg:
		m.streamAppend(msg.text)
		if m.phase == PhaseThinking {
			m.phase = PhaseStreaming
		}
		m.syncTranscript()
		return m, nil

	case streamDoneMsg:
		m.cancelLLM = nil
		m.streaming = false
		m.phase = PhaseIdle
		m.currentTool = ""
		if msg.err != nil && errors.Is(msg.err, context.Canceled) {
			m.lastResult = ""
			m.streamReset()
			m.blocks = append(m.blocks, m.st.Dim.Render("(assistant stream canceled)"))
			m.tokensDirty = true
		} else {
			if msg.err != nil {
				m.lastResult = "error"
				m.appendErrorBlock(msg.err.Error())
			} else {
				m.lastResult = "ok"
			}
			m.finalizeAssistantTurn()
		}
		m.syncTranscript()
		return m, nil

	case agentThinkingMsg:
		m.phase = PhaseThinking
		if msg.phase != "" {
			m.appendLog("agent: " + msg.phase)
		}
		m.syncTranscript()
		return m, nil

	case agentToolStartMsg:
		m.currentTool = msg.name
		m.toolStartTime = time.Now()
		m.appendToolRunning(msg.name)
		m.syncTranscript()
		return m, nil

	case agentToolDoneMsg:
		elapsed := time.Since(m.toolStartTime)
		m.currentTool = ""
		dur := fmt.Sprintf("  %dms", elapsed.Milliseconds())
		summary := msg.name
		if msg.isError {
			summary = "✗ " + summary + dur
		} else {
			summary = "✓ " + summary + dur
			if len(msg.content) > 0 {
				preview := msg.content
				if len(preview) > 60 {
					preview = preview[:60] + "…"
				}
				summary += "  " + preview
			}
		}
		m.appendToolSummary(summary)
		m.syncTranscript()
		return m, nil

	default:
		if key, ok := msg.(tea.KeyMsg); ok {
			if mod, cmd, handled := m.handleKey(key); handled {
				return mod, cmd
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.resizeInput()
		return m, cmd
	}
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	k := msg.String()
	if m.showLogOverlay && k == "esc" {
		m.showLogOverlay = false
		return m, nil, true
	}

	switch k {
	case "ctrl+c":
		if m.streaming && m.cancelLLM != nil {
			m.cancelLLM()
			m.appendLog("cancel requested (Ctrl+C)")
			return m, nil, true
		}
		return m, tea.Quit, true

	case "ctrl+e":
		m.showLogOverlay = !m.showLogOverlay
		m.appendLog("log overlay toggled")
		return m, nil, true

	case "ctrl+m":
		if m.mode == ModeChat {
			m.mode = ModeAgent
		} else {
			m.mode = ModeChat
		}
		m.appendLog("mode: " + m.mode.String())
		return m, nil, true

	case "tab":
		if m.streaming {
			return m, nil, true
		}
		if repl, ok := TabExpand(m.input.Value()); ok {
			m.input.SetValue(repl)
			m.input.CursorEnd()
			m.resizeInput()
			return m, nil, true
		}
		return m, nil, false

	case "up":
		if m.tryHistUp() {
			m.resizeInput()
			return m, nil, true
		}
		return m, nil, false

	case "down":
		if m.tryHistDown() {
			m.resizeInput()
			return m, nil, true
		}
		return m, nil, false

	case "enter", "return":
		if components.EnterInsertsNewline(msg) {
			return m, nil, false
		}
		if m.streaming {
			return m, nil, true
		}
		cmd := m.handleSubmit()
		return m, cmd, true
	}
	return m, nil, false
}

func (m *Model) tryHistUp() bool {
	if len(m.history) == 0 || m.streaming {
		return false
	}
	if m.histNav == 0 {
		m.histDraft = m.input.Value()
	}
	m.histNav++
	if m.histNav > len(m.history) {
		m.histNav = len(m.history)
	}
	idx := len(m.history) - m.histNav
	m.input.SetValue(m.history[idx])
	m.input.CursorEnd()
	return true
}

func (m *Model) tryHistDown() bool {
	if m.histNav == 0 {
		return false
	}
	m.histNav--
	if m.histNav == 0 {
		m.input.SetValue(m.histDraft)
		m.input.CursorEnd()
		return true
	}
	idx := len(m.history) - m.histNav
	m.input.SetValue(m.history[idx])
	m.input.CursorEnd()
	return true
}

func (m *Model) handleSubmit() tea.Cmd {
	txt := strings.TrimSpace(m.input.Value())
	if txt == "" {
		return nil
	}
	m.history = append(m.history, txt)
	_ = replhistory.Append(m.demoConfigDir, txt)
	m.histNav = 0
	m.histDraft = ""
	m.input.Reset()
	m.resizeInput()

	switch txt {
	case "quit":
		return tea.Quit
	case "clear":
		m.blocks = nil
		m.chatHistory = nil
		m.streamReset()
		m.appendLog("transcript and chat history cleared")
		m.layout()
		return nil
	case "help":
		m.appendUserBlock(txt)
		tools := "read_file · glob · grep"
		if m.cfg.Unsafe {
			tools += " · write_file · edit_file · bash"
		}
		m.appendSystemBlock("Commands: clear, help, quit.\n" +
			"Keys: Ctrl+M toggle chat/agent · Ctrl+E logs · Ctrl+C cancel/quit · Tab complete · ↑↓ history.\n" +
			"Chat mode: multi-turn conversation (context preserved across turns, cleared by 'clear').\n" +
			"Agent mode: LLM → tool (" + tools + ") → result → LLM → …")
		m.layout()
		return nil
	default:
		m.appendUserBlock(txt)
		m.appendLog("llm: " + txt)
		m.layout()
		m.startStream(txt)
		return m.spinner.Tick
	}
}

func (m *Model) startStream(user string) {
	if m.cancelLLM != nil {
		m.cancelLLM()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancelLLM = cancel
	m.streaming = true
	m.phase = PhaseThinking
	m.lastResult = ""
	m.currentTool = ""
	m.streamReset()
	m.syncTranscript()
	if m.mode == ModeAgent && m.agentRunner != nil {
		go m.runAgentTurn(ctx, user)
	} else {
		go m.runLLMStream(ctx, user)
	}
}
