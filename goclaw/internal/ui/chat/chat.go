package chat

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/okuzpe/goclaw/internal/orchestrator"
)

type Model struct {
	ctx context.Context

	viewport viewport.Model
	input    textinput.Model

	width  int
	height int

	lines        []string
	streaming    bool
	statusLine   string
	helpVisible  bool

	onSubmit    func(userText string)
	slashHandle SlashHandler

	approval *ApprovalBroker
	pending  *ApprovalRequest

	// streaming buffer for the current assistant message
	curAssistant strings.Builder
}

type Options struct {
	Title string
}

type SlashHandler func(input string) (handled bool, out string, quit bool, err error)

type ApprovalRequest struct {
	ToolName string
	Preview  string
	Resp     chan bool
}

type ApprovalBroker struct {
	Requests chan ApprovalRequest
}

func NewApprovalBroker() *ApprovalBroker {
	return &ApprovalBroker{Requests: make(chan ApprovalRequest, 8)}
}

func (b *ApprovalBroker) ToolApprover() orchestrator.ToolApprover {
	return func(ctx context.Context, toolName, toolInput string) (bool, error) {
		preview := toolInput
		if len(preview) > 400 {
			preview = preview[:400] + "…"
		}
		req := ApprovalRequest{
			ToolName: toolName,
			Preview:  preview,
			Resp:     make(chan bool, 1),
		}
		select {
		case b.Requests <- req:
		case <-ctx.Done():
			return false, ctx.Err()
		}
		select {
		case ok := <-req.Resp:
			return ok, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}

func New(ctx context.Context, opts Options) Model {
	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.MouseWheelEnabled = true

	in := textinput.New()
	in.Placeholder = "Type a message… (/help)"
	in.Prompt = "> "
	in.Focus()

	m := Model{
		ctx:      ctx,
		viewport: vp,
		input:    in,
		lines:    nil,
	}
	if strings.TrimSpace(opts.Title) != "" {
		m.lines = append(m.lines, lipgloss.NewStyle().Bold(true).Render(opts.Title))
	}
	m.appendSystem("Ready. Press Ctrl+C to exit.")
	return m
}

func (m Model) Init() tea.Cmd {
	if m.approval != nil {
		return waitForApproval(m.approval.Requests)
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.pending != nil {
			switch msg.String() {
			case "y", "Y":
				select {
				case m.pending.Resp <- true:
				default:
				}
				m.pending = nil
				return m, waitForApproval(m.approval.Requests)
			case "n", "N", "esc":
				select {
				case m.pending.Resp <- false:
				default:
				}
				m.pending = nil
				return m, waitForApproval(m.approval.Requests)
			}
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+l":
			m.lines = nil
			m.viewport.SetContent("")
			return m, nil
		case "enter":
			txt := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			if txt == "" {
				return m, nil
			}
			m.appendUser(txt)
			if m.slashHandle != nil {
				if handled, out, quit, err := m.slashHandle(txt); handled {
					if err != nil {
						m.appendSystem(fmt.Sprintf("error: %v", err))
					} else if strings.TrimSpace(out) != "" {
						m.appendSystem(out)
					}
					m.viewport.GotoBottom()
					if quit {
						return m, tea.Quit
					}
					return m, nil
				}
			}
			if m.onSubmit != nil {
				m.streaming = true
				m.statusLine = ""
				m.curAssistant.Reset()
				m.onSubmit(txt)
			} else {
				m.appendAssistant("(TUI wired; missing submit handler)")
			}
			m.viewport.GotoBottom()
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		// allow viewport to recompute internal state too
		var vcmd tea.Cmd
		m.viewport, vcmd = m.viewport.Update(msg)
		return m, vcmd
	case approvalMsg:
		r := ApprovalRequest(msg)
		m.pending = &r
		return m, nil
	case assistantDeltaMsg:
		m.curAssistant.WriteString(string(msg))
		m.refreshAssistantLine()
		return m, nil
	case assistantDoneMsg:
		m.streaming = false
		m.statusLine = ""
		m.refreshAssistantLine()
		m.viewport.GotoBottom()
		return m, nil
	case toolUseMsg:
		m.statusLine = fmt.Sprintf("running tool: %s", msg.name)
		return m, nil
	case toolResultMsg:
		if msg.isError {
			m.statusLine = fmt.Sprintf("tool result: %s (error, %d bytes)", msg.name, msg.bytes)
		} else {
			m.statusLine = fmt.Sprintf("tool result: %s (%d bytes)", msg.name, msg.bytes)
		}
		return m, nil
	case errMsg:
		m.streaming = false
		m.statusLine = ""
		m.appendSystem(fmt.Sprintf("error: %v", msg.err))
		return m, nil
	}

	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	content := lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		m.footerView(),
	)
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

func (m *Model) layout() {
	footerH := lipgloss.Height(m.footerView())
	h := m.height - footerH
	if h < 1 {
		h = 1
	}
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(h)
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
	m.viewport.GotoBottom()
}

func (m Model) footerView() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#4b5563", Dark: "#9ca3af"})
	status := strings.TrimSpace(m.statusLine)
	if m.streaming {
		if status != "" {
			status += " · "
		}
		status += "streaming…"
	}
	if status == "" {
		status = "Enter: send · Ctrl+L: clear · Ctrl+C: exit"
	}
	footer := dim.Render(status) + "\n" + m.input.View()
	if m.pending != nil {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			BorderForeground(lipgloss.Color("214")).
			Render(fmt.Sprintf("Allow tool?\n\n%s\n\n%s\n\n(y) allow  (n/esc) deny",
				lipgloss.NewStyle().Bold(true).Render(m.pending.ToolName),
				lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#d1d5db"}).Render(m.pending.Preview),
			))
		return footer + "\n\n" + box
	}
	return footer
}

func (m *Model) appendSystem(s string) {
	m.lines = append(m.lines, lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"}).Render(s))
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) appendUser(s string) {
	prefix := lipgloss.NewStyle().Bold(true).Render("you")
	m.lines = append(m.lines, fmt.Sprintf("%s %s", prefix, s))
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) appendAssistant(s string) {
	prefix := lipgloss.NewStyle().Bold(true).Render("goclaw")
	m.lines = append(m.lines, fmt.Sprintf("%s %s", prefix, s))
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) refreshAssistantLine() {
	// Remove the last assistant line if it was the streaming one; then re-add.
	prefix := lipgloss.NewStyle().Bold(true).Render("goclaw")
	rendered := fmt.Sprintf("%s %s", prefix, m.curAssistant.String())

	if len(m.lines) > 0 && strings.HasPrefix(stripANSI(m.lines[len(m.lines)-1]), "goclaw ") {
		m.lines[len(m.lines)-1] = rendered
	} else {
		m.lines = append(m.lines, rendered)
	}
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func stripANSI(s string) string {
	return ansi.Strip(s)
}

type approvalMsg ApprovalRequest

func waitForApproval(ch <-chan ApprovalRequest) tea.Cmd {
	return func() tea.Msg {
		req := <-ch
		return approvalMsg(req)
	}
}

type assistantDeltaMsg string
type assistantDoneMsg struct{}

type toolUseMsg struct {
	name string
}

type toolResultMsg struct {
	name    string
	bytes   int
	isError bool
}

type errMsg struct {
	err error
}

type programSink struct {
	p *tea.Program
}

func (s programSink) OnTextDelta(text string) {
	s.p.Send(assistantDeltaMsg(text))
}

func (s programSink) OnToolUse(name, _ string) {
	s.p.Send(toolUseMsg{name: name})
}

func (s programSink) OnToolResult(name string, resultBytes int, isError bool) {
	s.p.Send(toolResultMsg{name: name, bytes: resultBytes, isError: isError})
}

func (s programSink) OnDone(_ string) {
	s.p.Send(assistantDoneMsg{})
}

type Submitter func(ctx context.Context, userText string, sink orchestrator.StreamSink) (string, error)

// RunApp runs a chat TUI wired to an orchestrator submitter and an optional slash handler.
func RunApp(ctx context.Context, opts Options, approval *ApprovalBroker, submit Submitter, slash SlashHandler) error {
	m := New(ctx, opts)
	m.approval = approval
	m.slashHandle = slash

	p := tea.NewProgram(m)
	m.onSubmit = func(userText string) {
		go func() {
			_, err := submit(ctx, userText, programSink{p: p})
			if err != nil {
				p.Send(errMsg{err: err})
			}
		}()
	}

	_, err := p.Run()
	return err
}

