package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
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
	spinner  spinner.Model

	width  int
	height int

	lines        []string
	streaming  bool
	statusLine string

	// assistantPlaceholder is true while we show a dim "…" line before first token.
	assistantPlaceholder bool
	spinnerActive        bool

	theme *Theme

	slashHandle SlashHandler

	approval *ApprovalBroker
	pending  *ApprovalRequest

	submitter *submitRunner

	curAssistant strings.Builder
}

type submitRunner struct {
	fn  func(userText string)
	mu  sync.Mutex
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

type Options struct {
	Title string
	Theme *Theme
}

// SlashHandler: if modelSubmit is non-empty, send that text to the model after displaying out (e.g. /edit).
// When quit is true, err may be nil (caller normalizes /quit before the TUI).
type SlashHandler func(input string) (handled bool, out string, quit bool, modelSubmit string, err error)

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
	th := opts.Theme
	if th == nil {
		th = DefaultTheme()
	}

	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.MouseWheelEnabled = true

	in := textinput.New()
	in.Placeholder = "Message…  /help  /edit multiline"
	in.Prompt = th.InputPrompt
	in.Focus()

	spin := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(SpinnerAccentStyle()),
	)

	m := Model{
		ctx:       ctx,
		viewport:  vp,
		input:     in,
		spinner:   spin,
		theme:     th,
		lines:     nil,
		submitter: new(submitRunner),
	}
	if strings.TrimSpace(opts.Title) != "" {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#4F46E5", Dark: "#A5B4FC"})
		m.lines = append(m.lines, titleStyle.Render(opts.Title))
	}
	m.appendSystem("Ready. " + th.FooterHint())
	return m
}

func (m *Model) Init() tea.Cmd {
	if m.approval != nil {
		return waitForApproval(m.approval.Requests)
	}
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !m.spinnerActive {
			return m, nil
		}
		var sc tea.Cmd
		m.spinner, sc = m.spinner.Update(msg)
		return m, sc
	case tea.KeyMsg:
		// Ctrl+C during streaming cancels the in-flight request; at the prompt it quits.
		if msg.String() == "ctrl+c" {
			if m.streaming {
				m.submitter.cancel()
				return m, nil
			}
			return m, tea.Quit
		}
		if m.pending == nil && msg.String() == "esc" {
			return m, tea.Quit
		}
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
			default:
				// Modal approval: ignore other keys (including Enter) so they are not treated as a new chat message.
				return m, nil
			}
		}
		switch msg.String() {
		case "ctrl+l":
			m.lines = nil
			m.viewport.SetContent("")
			m.assistantPlaceholder = false
			m.spinnerActive = false
			return m, nil
		case "enter":
			txt := strings.TrimSpace(m.input.Value())
			if txt == "" {
				return m, nil
			}
			// Avoid overlapping RunStreaming calls on the same session (or sending while awaiting tools).
			if m.streaming {
				return m, nil
			}
			m.input.SetValue("")
			m.appendUser(txt)
			if m.slashHandle != nil {
				handled, out, quit, modelSubmit, err := m.slashHandle(txt)
				if handled {
					if err != nil {
						m.appendSystem(fmt.Sprintf("error: %v", err))
					} else if strings.TrimSpace(out) != "" {
						m.appendSystem(out)
					}
					m.viewport.GotoBottom()
					if strings.TrimSpace(modelSubmit) != "" && m.submitter != nil && m.submitter.fn != nil {
						m.runModelSubmit(modelSubmit)
					}
					if quit {
						return m, tea.Quit
					}
					return m, nil
				}
			}
			if m.submitter != nil && m.submitter.fn != nil {
				m.runModelSubmit(txt)
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
		var vcmd tea.Cmd
		m.viewport, vcmd = m.viewport.Update(msg)
		return m, vcmd
	case approvalMsg:
		r := ApprovalRequest(msg)
		m.pending = &r
		return m, nil
	case assistantPlaceholderMsg:
		m.streaming = true
		m.assistantPlaceholder = true
		m.statusLine = "thinking"
		m.curAssistant.Reset()
		m.appendAssistantDim("…")
		m.viewport.GotoBottom()
		m.spinner = spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(SpinnerAccentStyle()),
		)
		m.spinnerActive = true
		return m, func() tea.Msg { return m.spinner.Tick() }
	case assistantDeltaMsg:
		m.spinnerActive = false
		if m.assistantPlaceholder {
			m.stripAssistantPlaceholderLine()
			m.assistantPlaceholder = false
			m.statusLine = ""
		}
		m.curAssistant.WriteString(string(msg))
		m.refreshAssistantLine()
		return m, nil
	case assistantDoneMsg:
		m.streaming = false
		m.assistantPlaceholder = false
		m.spinnerActive = false
		m.statusLine = ""
		m.refreshAssistantLine()
		m.viewport.GotoBottom()
		return m, nil
	case toolUseMsg:
		// New LLM segment after tools: reset buffer so the next stream round does not repeat prior text.
		m.curAssistant.Reset()
		m.appendToolUseLine(msg.name, msg.preview)
		m.statusLine = fmt.Sprintf("running tool: %s", msg.name)
		m.viewport.GotoBottom()
		return m, nil
	case toolResultMsg:
		m.appendToolResultLine(msg.name, msg.bytes, msg.isError)
		if msg.isError {
			m.statusLine = fmt.Sprintf("tool result: %s (error, %d bytes)", msg.name, msg.bytes)
		} else {
			m.statusLine = fmt.Sprintf("tool result: %s (%d bytes)", msg.name, msg.bytes)
		}
		m.viewport.GotoBottom()
		return m, nil
	case errMsg:
		m.streaming = false
		m.assistantPlaceholder = false
		m.spinnerActive = false
		m.statusLine = ""
		m.stripAssistantPlaceholderLine()
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

func (m *Model) runModelSubmit(userText string) {
	m.streaming = true
	m.statusLine = ""
	m.curAssistant.Reset()
	m.submitter.fn(userText)
}

func (m *Model) View() tea.View {
	content := lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		m.footerView(),
	)
	v := tea.NewView(content)
	v.AltScreen = true
	// AllMotion emits SGR mouse sequences; on Windows/Git Bash they can leak into the shell after exit.
	v.MouseMode = tea.MouseModeNone
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

func (m *Model) footerView() string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	status := strings.TrimSpace(m.statusLine)
	if m.spinnerActive {
		status = strings.TrimSpace(m.spinner.View() + "  " + status)
		if status == "" {
			status = m.spinner.View()
		}
	}
	if m.streaming && !m.spinnerActive {
		if status != "" {
			status += " · "
		}
		status += "streaming…"
	}
	if status == "" {
		status = th.FooterHint()
	}
	footer := th.FooterDim.Render(status) + "\n" + m.input.View()
	if m.pending != nil {
		inner := fmt.Sprintf(
			"Allow tool?\n\n%s\n\n%s\n\n(y) allow  (n/esc) deny",
			th.ModalTitle.Render(m.pending.ToolName),
			th.ModalBody.Render(m.pending.Preview),
		)
		box := th.ModalBorder.Padding(1, 2).Render(inner)
		return footer + "\n\n" + box
	}
	return footer
}

func (m *Model) appendSystem(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.System.Render(s))
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) appendUser(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, fmt.Sprintf("%s %s", th.UserPrefix(), s))
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) appendAssistant(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, fmt.Sprintf("%s %s", th.AssistantPrefix(), s))
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) appendAssistantDim(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	dim := th.Dim.Render(s)
	m.lines = append(m.lines, fmt.Sprintf("%s %s", th.AssistantPrefix(), dim))
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) stripAssistantPlaceholderLine() {
	if len(m.lines) == 0 {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	last := m.lines[len(m.lines)-1]
	plain := stripANSI(last)
	pfx := th.AssistantPlainPrefix()
	if strings.HasPrefix(plain, pfx) && strings.Contains(plain, "…") {
		m.lines = m.lines[:len(m.lines)-1]
		m.viewport.SetContent(strings.Join(m.lines, "\n"))
	}
}

func (m *Model) appendToolUseLine(name, preview string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	preview = strings.TrimSpace(preview)
	if len(preview) > 220 {
		preview = preview[:220] + "…"
	}
	tag := th.ToolTag.Render("[tool]")
	line := fmt.Sprintf("%s %s", tag, th.Assistant.Render(name))
	if preview != "" {
		line += " " + th.Dim.Render(preview)
	}
	m.lines = append(m.lines, line)
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) appendToolResultLine(name string, nbytes int, isError bool) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	tag := th.ToolTag.Render("[result]")
	status := "ok"
	if isError {
		status = th.Dim.Render("error")
	} else {
		status = th.Dim.Render("ok")
	}
	line := fmt.Sprintf("%s %s — %d bytes — %s", tag, th.Assistant.Render(name), nbytes, status)
	m.lines = append(m.lines, line)
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) refreshAssistantLine() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	prefix := th.AssistantPrefix()
	rendered := fmt.Sprintf("%s %s", prefix, m.curAssistant.String())
	pfxPlain := th.AssistantPlainPrefix()

	if len(m.lines) > 0 && strings.HasPrefix(stripANSI(m.lines[len(m.lines)-1]), pfxPlain) {
		m.lines[len(m.lines)-1] = rendered
	} else {
		m.lines = append(m.lines, rendered)
	}
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func stripANSI(s string) string {
	return ansi.Strip(s)
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i] + "…"
		}
		n++
	}
	return s
}

type approvalMsg ApprovalRequest

func waitForApproval(ch <-chan ApprovalRequest) tea.Cmd {
	return func() tea.Msg {
		req := <-ch
		return approvalMsg(req)
	}
}

type assistantPlaceholderMsg struct{}

type assistantDeltaMsg string

type assistantDoneMsg struct{}

type toolUseMsg struct {
	name    string
	preview string
}

type toolResultMsg struct {
	name    string
	bytes   int
	isError bool
}

type errMsg struct {
	err error
}

type Submitter func(ctx context.Context, userText string, sink orchestrator.StreamSink) (string, error)

// RunApp runs a chat TUI wired to an orchestrator submitter and an optional slash handler.
func RunApp(ctx context.Context, opts Options, approval *ApprovalBroker, submit Submitter, slash SlashHandler) error {
	m := New(ctx, opts)
	m.approval = approval
	m.slashHandle = slash

	p := tea.NewProgram(&m)
	m.submitter.fn = func(userText string) {
		reqCtx, cancel := context.WithCancel(ctx)
		m.submitter.setCancel(cancel)
		go func() {
			defer func() {
				m.submitter.setCancel(nil)
				cancel()
			}()
			p.Send(assistantPlaceholderMsg{})
			sink := newBatchedProgramSink(p)
			_, err := submit(reqCtx, userText, sink)
			sink.flush()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					// User pressed Ctrl+C — clear streaming state cleanly.
					p.Send(assistantDoneMsg{})
				} else {
					p.Send(errMsg{err: err})
				}
			}
		}()
	}

	_, err := p.Run()
	return err
}
