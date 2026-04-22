package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) appendLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > 200 {
		m.logLines = m.logLines[len(m.logLines)-200:]
	}
}

func (m *Model) appendBlock(content string, style lipgloss.Style) {
	m.blocks = append(m.blocks, style.Render(content))
	if len(m.blocks) > maxBlocks {
		m.blocks = m.blocks[len(m.blocks)-maxBlocks:]
	}
	m.tokensDirty = true
}

func (m *Model) appendUserBlock(text string) {
	m.blocks = append(m.blocks,
		m.st.User.Render("You")+"\n"+m.st.Dim.Render(text))
	m.tokensDirty = true
}

func (m *Model) appendSystemBlock(text string) {
	m.appendBlock(text, m.st.Dim)
}

func (m *Model) appendErrorBlock(text string) {
	m.appendBlock(text, m.st.Err)
}

func (m *Model) appendToolRunning(name string) {
	m.phase = PhaseExecuting
	line := m.st.Tool.Render("⚙ tool ") + m.st.Dim.Render(name+" …")
	m.blocks = append(m.blocks, line)
	m.tokensDirty = true
}

func (m *Model) appendToolSummary(summary string) {
	if m.phase == PhaseExecuting {
		m.phase = PhaseThinking
	}
	m.appendBlock("  "+summary, m.st.ToolDim)
}

func (m *Model) streamAppend(delta string) {
	m.streamMu.Lock()
	m.stream.WriteString(delta)
	m.streamMu.Unlock()
}

func (m *Model) streamReset() {
	m.streamMu.Lock()
	m.stream.Reset()
	m.streamMu.Unlock()
}

func (m *Model) streamSnapshot() string {
	m.streamMu.Lock()
	s := m.stream.String()
	m.streamMu.Unlock()
	return s
}

func (m *Model) glamourMarkdown(src string) (string, error) {
	w := m.width - 6
	if w < 36 {
		w = 36
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(w),
		glamour.WithStandardStyle("dark"),
	)
	if err != nil {
		return "", err
	}
	out, err := r.Render(strings.TrimSpace(src))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (m *Model) finalizeAssistantTurn() {
	raw := m.streamSnapshot()
	m.streamReset()
	if strings.TrimSpace(raw) == "" {
		return
	}
	rendered, err := m.glamourMarkdown(raw)
	if err != nil || strings.TrimSpace(rendered) == "" {
		rendered = m.st.Assistant.Render(raw)
	}
	m.blocks = append(m.blocks, m.st.Accent.Render("Assistant")+"\n"+rendered)
	m.tokensDirty = true
}

func (m *Model) approxTokenEstimate() int {
	if m.tokensDirty || m.streaming {
		n := 0
		for _, b := range m.blocks {
			n += utf8.RuneCountInString(b)
		}
		n += utf8.RuneCountInString(m.streamSnapshot())
		m.tokenCount = max(1, n*3/4)
		m.tokensDirty = false
	}
	return m.tokenCount
}

func (m *Model) welcomeBlock() {
	host := strings.TrimSpace(m.cfg.OllamaHost)
	if host == "" {
		host = "http://127.0.0.1:11434"
	}
	mod := strings.TrimSpace(m.cfg.Model)
	tools := "read_file · glob · grep"
	if m.cfg.Unsafe {
		tools += " · write_file · edit_file · bash"
	}
	header := m.st.Accent.Render("◆ agentdemo") + "  " + m.st.Dim.Render(host+"  "+mod)
	keybinds := m.st.Dim.Render("  Ctrl+M") + "  toggle chat/agent" + "    " +
		m.st.Dim.Render("Ctrl+E") + "  logs" + "    " +
		m.st.Dim.Render("Ctrl+C") + "  cancel/quit" + "    " +
		m.st.Dim.Render("↑↓") + "  history"
	modes := m.st.Dim.Render("  chat") + "  multi-turn conversation\n" +
		m.st.Dim.Render("  agent") + " LLM → tool (" + tools + ") → result → LLM → …"
	block := m.st.Panel.Render(header + "\n\n" + keybinds + "\n" + modes)
	m.blocks = append(m.blocks, block)
	m.tokensDirty = true
}
