package ui

import (
	"context"
	"errors"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/okuzpe/goclaw/internal/agentdemo/components"
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
		if m.spinnerActive {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case streamDeltaMsg:
		m.streamAppend(msg.text)
		if m.phase == "thinking" {
			m.phase = "streaming"
		}
		m.syncTranscript()
		return m, nil

	case streamDoneMsg:
		if m.cancelLLM != nil {
			m.cancelLLM = nil
		}
		m.streaming = false
		m.spinnerActive = false
		m.phase = "idle"
		if msg.err != nil && errors.Is(msg.err, context.Canceled) {
			m.streamReset()
			m.blocks = append(m.blocks, m.st.Dim.Render("(assistant stream canceled)"))
		} else {
			if msg.err != nil {
				m.appendErrorBlock(msg.err.Error())
			}
			m.finalizeAssistantTurn()
		}
		m.syncTranscript()
		return m, nil

	case demoToolDoneMsg:
		m.appendToolSummary(msg.summary)
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
		switch m.mode {
		case ModeChat:
			m.mode = ModeCode
		case ModeCode:
			m.mode = ModeAgent
		default:
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
		m.layout()
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
	m.histNav = 0
	m.histDraft = ""
	m.input.Reset()
	m.resizeInput()

	switch txt {
	case "quit":
		return tea.Quit
	case "clear":
		m.blocks = nil
		m.streamReset()
		m.appendLog("transcript cleared")
		m.layout()
		return nil
	case "help":
		m.appendUserBlock(txt)
		m.appendSystemBlock("Built-in: clear, demo-tool, help, quit.\nKeys: Tab completes, Ctrl+M cycles UI label (chat/code/agent), Ctrl+E log overlay, Ctrl+C cancel or quit.\nFull agent: run goclaw from a repo.")
		m.layout()
		return nil
	case "demo-tool":
		m.appendUserBlock(txt)
		m.appendLog("demo-tool: list_files")
		m.appendToolRunning("list_files")
		m.layout()
		return tea.Batch(
			m.spinner.Tick,
			tea.Tick(850*time.Millisecond, func(time.Time) tea.Msg {
				return demoToolDoneMsg{summary: "README.md, go.mod, cmd/ (stub)"}
			}),
		)
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
	m.phase = "thinking"
	m.spinnerActive = true
	m.streamReset()
	m.syncTranscript()
	go m.runLLMStream(ctx, user)
}
