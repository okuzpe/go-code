package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/ui/footerline"
)

type Model struct {
	ctx context.Context

	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model

	width  int
	height int

	lines      []string
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

	// curAssistantLineIdx tracks which index in m.lines the current streaming
	// assistant text occupies. -1 means no active assistant line.
	curAssistantLineIdx int

	// toolWaitQueue pairs each OnToolUse with the following OnToolResult (same order).
	toolWaitQueue []pendingTool

	sessionID string
}

// pendingTool holds human-readable tool activity for compact status + done lines.
type pendingTool struct {
	name    string
	summary string
}

type submitRunner struct {
	fn         func(userText string)
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

type Options struct {
	Title     string
	SessionID string
	Theme     *Theme
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
		preview := orchestrator.FormatToolUsePreview(toolName, toolInput)
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

// inputMaxHeight is the maximum number of visible lines in the input textarea.
const inputMaxHeight = 6

func New(ctx context.Context, opts Options) Model {
	th := opts.Theme
	if th == nil {
		th = DefaultTheme()
	}

	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.MouseWheelEnabled = true

	// Use textarea for multi-line input support (modern CLI standard).
	in := textarea.New()
	in.Placeholder = "Message goclaw…  /help  Ctrl+J newline"
	in.Prompt = th.InputPrompt
	in.ShowLineNumbers = false
	in.SetHeight(1) // start compact, grows dynamically
	in.SetWidth(0)
	in.CharLimit = 0 // no char limit
	in.Focus()

	// Override key bindings: Enter submits, Alt+Enter / Ctrl+J inserts newline
	km := in.KeyMap
	km.InsertNewline.SetKeys("ctrl+j", "alt+enter")
	in.KeyMap = km

	spin := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(SpinnerAccentStyle()),
	)

	m := Model{
		ctx:                 ctx,
		viewport:            vp,
		input:               in,
		spinner:             spin,
		theme:               th,
		lines:               nil,
		submitter:           new(submitRunner),
		curAssistantLineIdx: -1,
		sessionID:           strings.TrimSpace(opts.SessionID),
	}
	if strings.TrimSpace(opts.Title) != "" {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C4B5FD"})
		m.lines = append(m.lines, titleStyle.Render(opts.Title))
		m.lines = append(m.lines, th.SeparatorLine(0))
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
		if mdl, cmd, handled := m.handleKeyString(msg.String()); handled {
			return mdl, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(m.width - 2) // leave room for border
		m.reflowTitleSeparator()
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
		m.statusLine = ""
		m.curAssistant.Reset()
		m.curAssistantLineIdx = -1
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
		m.viewport.GotoBottom()
		return m, nil
	case assistantDoneMsg:
		m.streaming = false
		m.assistantPlaceholder = false
		m.spinnerActive = false
		m.statusLine = ""
		// Finalize the current segment with markdown rendering.
		m.finalizeCurrentSegment()
		m.viewport.GotoBottom()
		return m, nil
	case toolUseMsg:
		// Finalize the pre-tool text with markdown rendering BEFORE resetting.
		m.finalizeCurrentSegment()
		// Reset buffer for the next assistant segment (post-tool).
		m.curAssistant.Reset()
		m.curAssistantLineIdx = -1
		m.toolWaitQueue = append(m.toolWaitQueue, pendingTool{name: msg.name, summary: msg.preview})
		m.statusLine = m.toolQueueStatusLine()
		m.viewport.GotoBottom()
		return m, nil
	case toolResultMsg:
		job, ok := m.popToolJob()
		if !ok {
			job = pendingTool{name: msg.name, summary: ""}
		}
		m.appendToolDoneLine(job.name, job.summary, msg.isError)
		if len(m.toolWaitQueue) > 0 {
			m.statusLine = m.toolQueueStatusLine()
		} else {
			m.statusLine = ""
		}
		m.viewport.GotoBottom()
		return m, nil
	case errMsg:
		m.streaming = false
		m.assistantPlaceholder = false
		m.spinnerActive = false
		m.statusLine = ""
		m.toolWaitQueue = nil
		m.stripAssistantPlaceholderLine()
		m.appendError(fmt.Sprintf("✗ %v", msg.err))
		m.curAssistantLineIdx = -1
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

	// Dynamic input height: grow textarea as user types more lines (up to inputMaxHeight).
	m.resizeInput()

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKeyString(k string) (tea.Model, tea.Cmd, bool) {
	// Ctrl+C during streaming cancels the in-flight request; at the prompt it quits.
	if k == "ctrl+c" {
		if m.streaming {
			m.submitter.cancel()
			return m, nil, true
		}
		return m, tea.Quit, true
	}
	if m.pending == nil && k == "esc" {
		return m, tea.Quit, true
	}
	if m.pending != nil {
		switch k {
		case "y", "Y":
			select {
			case m.pending.Resp <- true:
			default:
			}
			m.pending = nil
			return m, waitForApproval(m.approval.Requests), true
		case "n", "N", "esc":
			select {
			case m.pending.Resp <- false:
			default:
			}
			m.pending = nil
			return m, waitForApproval(m.approval.Requests), true
		default:
			// Modal approval: ignore other keys (including Enter) so they are not treated as a new chat message.
			return m, nil, true
		}
	}
	switch k {
	case "ctrl+l":
		m.lines = nil
		m.toolWaitQueue = nil
		m.viewport.SetContent("")
		m.assistantPlaceholder = false
		m.spinnerActive = false
		m.curAssistantLineIdx = -1
		return m, nil, true
	case "enter":
		txt := strings.TrimSpace(m.input.Value())
		if txt == "" {
			return m, nil, true
		}
		// Avoid overlapping RunStreaming calls on the same session (or sending while awaiting tools).
		if m.streaming {
			return m, nil, true
		}
		m.input.Reset()
		m.resizeInput() // shrink back to 1 line after submit
		m.appendSeparator()
		m.appendUser(txt)
		if m.slashHandle != nil {
			handled, out, quit, modelSubmit, err := m.slashHandle(txt)
			if handled {
				if err != nil {
					m.appendError(fmt.Sprintf("error: %v", err))
				} else if strings.TrimSpace(out) != "" {
					m.appendSystem(out)
				}
				m.viewport.GotoBottom()
				if strings.TrimSpace(modelSubmit) != "" && m.submitter != nil && m.submitter.fn != nil {
					m.runModelSubmit(modelSubmit)
				}
				if quit {
					return m, tea.Quit, true
				}
				return m, nil, true
			}
		}
		if m.submitter != nil && m.submitter.fn != nil {
			m.runModelSubmit(txt)
		} else {
			m.appendAssistant("(TUI wired; missing submit handler)")
		}
		m.viewport.GotoBottom()
		return m, nil, true
	}
	return m, nil, false
}

func (m *Model) runModelSubmit(userText string) {
	m.streaming = true
	m.statusLine = ""
	m.curAssistant.Reset()
	m.curAssistantLineIdx = -1
	m.submitter.fn(userText)
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
	m.input.SetHeight(lineCount)
	m.layout()
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

// reflowTitleSeparator widens the rule under the banner when the terminal resizes.
func (m *Model) reflowTitleSeparator() {
	if m.width <= 0 || len(m.lines) < 2 {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	plain0 := stripANSI(m.lines[0])
	if !strings.HasPrefix(strings.TrimSpace(plain0), "goclaw") {
		return
	}
	m.lines[1] = th.SeparatorLine(m.width)
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
		spin := strings.TrimSpace(m.spinner.View())
		base := strings.TrimSpace(status)
		if base == "" {
			base = "Thinking…"
		}
		status = strings.TrimSpace(spin + "  " + base)
	}
	if m.streaming && !m.spinnerActive && strings.TrimSpace(status) == "" {
		status = "Responding…"
	}
	if status == "" {
		status = th.FooterHint()
	}

	status = footerline.Join(status, m.sessionID, m.width)

	// Input area with a subtle border
	inputView := m.input.View()
	if m.width > 4 {
		inputView = th.InputBorder.Width(m.width - 4).Render(inputView)
	}

	footer := th.FooterDim.Render(status) + "\n" + inputView
	if m.pending != nil {
		inner := fmt.Sprintf(
			"%s\n\n%s\n\n%s\n\n%s",
			th.ModalTitle.Render("⚡ Allow tool execution?"),
			th.ModalBody.Render("Tool: "+th.Tool.Render(m.pending.ToolName)),
			th.ModalBody.Render(m.pending.Preview),
			th.Dim.Render("(y) allow  (n/esc) deny"),
		)
		box := th.ModalBorder.Padding(1, 2).Render(inner)
		return footer + "\n\n" + box
	}
	return footer
}

// ─── Line append helpers ────────────────────────────────────────────────────

func (m *Model) appendSeparator() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.SeparatorLine(m.width))
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) appendSystem(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.System.Render(s))
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

func (m *Model) appendError(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.ErrorStyle.Render(s))
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

func (m *Model) toolQueueStatusLine() string {
	if len(m.toolWaitQueue) == 0 {
		return ""
	}
	return orchestrator.ToolWorkingPhrase(m.toolWaitQueue[0].name) + "…"
}

func (m *Model) popToolJob() (pendingTool, bool) {
	if len(m.toolWaitQueue) == 0 {
		return pendingTool{}, false
	}
	j := m.toolWaitQueue[0]
	m.toolWaitQueue = m.toolWaitQueue[1:]
	return j, true
}

// appendToolDoneLine adds one compact Claude-style line after a tool finishes (no JSON).
func (m *Model) appendToolDoneLine(toolName, summary string, isError bool) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	label := orchestrator.ToolFinishedPhrase(toolName)
	var line string
	if isError {
		line = fmt.Sprintf("  %s  %s",
			th.ToolResultErr.Render("✗"),
			label)
	} else {
		suffix := ""
		if s := strings.TrimSpace(summary); s != "" {
			suffix = "  " + th.Dim.Render(truncateRunes(s, 96))
		}
		line = fmt.Sprintf("  %s  %s%s",
			th.ToolResultOk.Render("✓"),
			label,
			suffix)
	}
	m.lines = append(m.lines, line)
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

// refreshAssistantLine updates or appends a line with streaming content.
// Uses curAssistantLineIdx to track which line we're updating.
func (m *Model) refreshAssistantLine() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	prefix := th.AssistantPrefix()
	rendered := fmt.Sprintf("%s %s", prefix, m.curAssistant.String())

	if m.curAssistantLineIdx >= 0 && m.curAssistantLineIdx < len(m.lines) {
		// We know exactly which line to update.
		m.lines[m.curAssistantLineIdx] = rendered
	} else {
		// Start a new assistant line.
		m.lines = append(m.lines, rendered)
		m.curAssistantLineIdx = len(m.lines) - 1
	}
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

// finalizeCurrentSegment renders the current curAssistant buffer as markdown
// and replaces the streaming line with the formatted version. Called both
// before a tool use (to finalize pre-tool text) and on assistantDone.
func (m *Model) finalizeCurrentSegment() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	raw := m.curAssistant.String()
	if strings.TrimSpace(raw) == "" {
		return
	}

	prefix := th.AssistantPrefix()
	pfxPlain := th.AssistantPlainPrefix()

	// Render markdown
	rendered := th.RenderMarkdown(raw, m.width)

	// Indent continuation lines to align with the prefix
	mdLines := strings.Split(rendered, "\n")
	padStr := strings.Repeat(" ", len([]rune(pfxPlain))+1)
	var finalLines []string
	for i, line := range mdLines {
		if i == 0 {
			finalLines = append(finalLines, fmt.Sprintf("%s %s", prefix, line))
		} else {
			finalLines = append(finalLines, padStr+line)
		}
	}
	finalRendered := strings.Join(finalLines, "\n")

	// Replace the tracked streaming line with the markdown version.
	if m.curAssistantLineIdx >= 0 && m.curAssistantLineIdx < len(m.lines) {
		m.lines[m.curAssistantLineIdx] = finalRendered
	} else {
		m.lines = append(m.lines, finalRendered)
	}
	m.curAssistantLineIdx = -1
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
