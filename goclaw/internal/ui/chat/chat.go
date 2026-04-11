package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/okuzpe/goclaw/internal/text"
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

	// toolWaitStartedAt is when the first pending tool in the queue began (for elapsed seconds in the footer).
	toolWaitStartedAt time.Time

	sessionID string

	focusLine func() string

	// Welcome panel (optional): reflow when terminal width is first known or on resize.
	welcomeOpts     WelcomeOptions
	welcomeBlockEnd int // exclusive index in m.lines after dashboard + trailing blank line; 0 if none

	// TUI help overlay (/help): viewport shows helpFullText; transcript stays in m.lines underneath.
	helpOpen     bool
	helpFullText string
	appVersion   string

	// Interactive /theme overlay (arrow keys + Enter).
	themePickOpen     bool
	themePickCursor   int
	themePickFullText string
	userConfigDir     string
	workdir           string

	// Interactive /agents overlay (arrow keys + Enter).
	agentPickOpen      bool
	agentPickCursor    int
	agentPickFullText  string
	userAgentsDir      string
	projectAgentsDir   string
	activeAgentProfile string

	// footerRendered is built once in layout() so View() joins the same string as used for height math
	// (avoids footer/sizing mismatch if textarea state differed between two footerView calls).
	footerRendered string
	// lastTranscript avoids redundant viewport.SetContent when only the footer/spinner changed (reduces flicker).
	lastTranscript string

	// exitConfirmDeadline is the wall-clock instant until which a second Ctrl+C quits the TUI (double Ctrl+C).
	exitConfirmDeadline time.Time
	// ctrlCMsgRendered holds the rendered "Press Ctrl+C again…" line so it can be removed when the deadline expires.
	ctrlCMsgRendered string
}

type toolTickMsg struct{}

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
	// Workdir when set enables the cwd-aware session intro before "Ready."
	Workdir string
	// UserConfigDir is ~/.goclaw; when set, /theme opens an interactive picker in the fullscreen TUI.
	UserConfigDir string
	// UserAgentsDir and ProjectAgentsDir enable /agents picker (merged with built-ins).
	UserAgentsDir    string
	ProjectAgentsDir string
	// ActiveAgentProfile seeds the /agents picker cursor (typically the session start profile).
	ActiveAgentProfile string
	// Welcome optional bordered panel before the title (version + tips).
	Welcome WelcomeOptions
	// FocusLine optional; when non-nil, its return value is shown in the footer (e.g. worker focus).
	FocusLine func() string
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

// maxSlashSuggestRows caps the TUI slash-command picker (single-line / buffer); keeps the UI compact.
const maxSlashSuggestRows = 5

func placeholderForWidth(termWidth int) string {
	const full = "Ask anything…  ! @ & /btw /help · Shift+Enter newline"
	const narrow = "Ask anything…  /help"
	if termWidth > 0 && termWidth < 60 {
		return narrow
	}
	return full
}

func New(ctx context.Context, opts Options) Model {
	th := opts.Theme
	if th == nil {
		th = DefaultTheme()
	}

	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.MouseWheelEnabled = false // mouse mode is off; scroll via keyboard (PgUp/PgDn, j/k, Ctrl+U/D)
	vp.SoftWrap = true

	// Use textarea for multi-line input support (modern CLI standard).
	in := textarea.New()
	in.Placeholder = placeholderForWidth(0)
	in.Prompt = th.InputPrompt
	in.ShowLineNumbers = false
	in.SetHeight(1) // start compact, grows dynamically
	in.SetWidth(0)
	in.CharLimit = 0 // no char limit
	in.Focus()

	// Enter is handled in handleKeyString (submit). Newline: Shift+Enter / Alt+Enter.
	// Ctrl+J is omitted — many Windows/Git Bash terminals swallow it before Bubble Tea sees it.
	km := in.KeyMap
	km.InsertNewline.SetKeys("shift+enter", "alt+enter")
	in.KeyMap = km

	spin := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(th.SpinnerAccentV2()),
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
		focusLine:           opts.FocusLine,
		appVersion:          strings.TrimSpace(opts.Welcome.Version),
		userConfigDir:       strings.TrimSpace(opts.UserConfigDir),
		workdir:             strings.TrimSpace(opts.Workdir),
		userAgentsDir:       strings.TrimSpace(opts.UserAgentsDir),
		projectAgentsDir:    strings.TrimSpace(opts.ProjectAgentsDir),
		activeAgentProfile:  strings.TrimSpace(opts.ActiveAgentProfile),
	}
	if strings.TrimSpace(opts.Welcome.Version) != "" {
		if dash := WelcomeDashboardLines(th, opts.Welcome, 0); len(dash) > 0 {
			m.welcomeOpts = opts.Welcome
			m.lines = append(m.lines, dash...)
			m.lines = append(m.lines, "")
			m.welcomeBlockEnd = len(m.lines)
		}
	}
	if m.welcomeBlockEnd == 0 {
		if strings.TrimSpace(opts.Title) != "" {
			m.lines = append(m.lines, th.ModalTitle.Render(opts.Title))
			m.lines = append(m.lines, th.SeparatorLine(0))
		}
		intro := SessionIntroSystemText(opts.Workdir)
		m.appendSystem(intro)
		m.appendSystem("Ready.")
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	if m.approval != nil {
		return waitForApproval(m.approval.Requests)
	}
	return nil
}

func tickToolWait() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return toolTickMsg{} })
}

// ctrlCExitArmExpiredMsg clears the exit-confirm window when the scheduled deadline passes.
// expected must match m.exitConfirmDeadline so restarts of the 3s window ignore stale ticks.
type ctrlCExitArmExpiredMsg struct {
	expected time.Time
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
		if m.agentPickOpen {
			switch msg.String() {
			case "up":
				m.moveAgentPickCursor(-1)
				return m, nil
			case "down":
				m.moveAgentPickCursor(1)
				return m, nil
			case "enter":
				m.applyAgentPick()
				return m, nil
			case "esc":
				m.closeAgentPicker()
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				return m, nil
			}
		}
		if m.themePickOpen {
			switch msg.String() {
			case "up":
				m.moveThemePickCursor(-1)
				return m, nil
			case "down":
				m.moveThemePickCursor(1)
				return m, nil
			case "enter":
				out := m.applyThemePick()
				if strings.TrimSpace(out) != "" {
					m.appendSystem(out)
				}
				return m, nil
			case "esc":
				m.closeThemePicker()
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				return m, nil
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
		if mdl, cmd, handled := m.handleKeyString(msg.String()); handled {
			return mdl, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(m.width - 2) // leave room for border
		m.syncInputPlaceholder()
		m.rebuildWelcomeForWidth()
		m.reflowTitleSeparator()
		if m.themePickOpen {
			m.refreshThemePickOverlay()
		}
		if m.agentPickOpen {
			m.refreshAgentPickOverlay()
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
		m.streaming = true
		m.assistantPlaceholder = true
		m.statusLine = ""
		m.curAssistant.Reset()
		m.curAssistantLineIdx = -1
		m.appendAssistantDim("…")
		th := m.theme
		if th == nil {
			th = DefaultTheme()
		}
		m.spinner = spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(th.SpinnerAccentV2()),
		)
		m.spinnerActive = true
		return m, func() tea.Msg { return m.spinner.Tick() }
	case assistantDeltaMsg:
		if !m.streaming {
			// Drop stray deltas after completion (e.g. race with batching); avoids a second assistant row.
			return m, nil
		}
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
		// Finalize the current segment with markdown rendering.
		m.finalizeCurrentSegment()
		return m, nil
	case toolUseMsg:
		// Finalize the pre-tool text with markdown rendering BEFORE resetting.
		m.finalizeCurrentSegment()
		// Reset buffer for the next assistant segment (post-tool).
		m.curAssistant.Reset()
		m.curAssistantLineIdx = -1
		m.toolWaitQueue = append(m.toolWaitQueue, pendingTool{name: msg.name, summary: msg.preview})
		if len(m.toolWaitQueue) == 1 {
			m.toolWaitStartedAt = time.Now()
		}
		m.spinnerActive = true
		m.statusLine = m.toolQueueStatusLine()
		return m, tickToolWait()
	case toolTickMsg:
		if len(m.toolWaitQueue) == 0 {
			m.toolWaitStartedAt = time.Time{}
			return m, nil
		}
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
					break
				}
			}
			m.ctrlCMsgRendered = ""
			m.setLinesContent(false)
		}
		m.layout()
		return m, nil
	case toolResultMsg:
		job, ok := m.popToolJob()
		if !ok {
			job = pendingTool{name: msg.name, summary: ""}
		}
		m.appendToolDoneLine(job.name, job.summary, msg.isError)
		if len(m.toolWaitQueue) > 0 {
			m.toolWaitStartedAt = time.Now()
			m.statusLine = m.toolQueueStatusLine()
		} else {
			m.toolWaitStartedAt = time.Time{}
			m.statusLine = ""
		}
		if len(m.toolWaitQueue) > 0 {
			return m, tickToolWait()
		}
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

	// Drag-and-drop interception: convert file/dir paths pasted from the OS into @relpath tokens.
	if paste, isPaste := msg.(tea.PasteMsg); isPaste &&
		!m.streaming && m.pending == nil &&
		!m.helpOpen && !m.themePickOpen && !m.agentPickOpen {
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

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	if !m.helpOpen && !m.themePickOpen && !m.agentPickOpen {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
		// Dynamic input height: grow textarea as user types more lines (up to inputMaxHeight).
		m.resizeInput()
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKeyString(k string) (tea.Model, tea.Cmd, bool) {
	if m.pending != nil {
		switch k {
		case "y", "Y":
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
			// Modal approval: ignore other keys (including Enter) so they are not treated as a new chat message.
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
		return m, tea.Quit, true
	}
	switch k {
	case "ctrl+l":
		m.lines = nil
		m.toolWaitQueue = nil
		m.assistantPlaceholder = false
		m.spinnerActive = false
		m.curAssistantLineIdx = -1
		m.setLinesContent(true)
		return m, nil, true
	case "tab":
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
		if bareThemeSlashInput(txt) && strings.TrimSpace(m.userConfigDir) != "" {
			m.openThemePicker()
			return m, nil, true
		}
		if bareAgentsSlashInput(txt) && m.slashHandle != nil {
			m.openAgentPicker()
			return m, nil, true
		}
		if m.slashHandle != nil {
			handled, out, quit, modelSubmit, err := m.slashHandle(txt)
			if handled {
				if err != nil {
					m.appendError(fmt.Sprintf("error: %v", err))
				} else if strings.TrimSpace(out) != "" {
					if slashcmd.PlainHelpREPLRequest(txt) {
						m.openHelpOverlay(out)
					} else {
						m.appendSystem(out)
					}
				}
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
	m.setLinesContent(true)
	m.layout()
	return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return ctrlCExitArmExpiredMsg{expected: expected}
	}), true
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

// setLinesContent refreshes the transcript in the viewport. If stickToBottom is true or the user
// was already at the bottom, scroll stays pinned to the latest output.
func (m *Model) setLinesContent(stickToBottom bool) {
	joined := strings.Join(m.lines, "\n")
	m.lastTranscript = joined
	stick := stickToBottom || m.viewport.AtBottom()
	m.viewport.SetContent(joined)
	if stick {
		m.viewport.GotoBottom()
	}
}

func (m *Model) syncInputPlaceholder() {
	m.input.Placeholder = placeholderForWidth(m.width)
}

func (m *Model) footerBrand() string {
	v := strings.TrimSpace(m.appVersion)
	if v == "" {
		return "goclaw"
	}
	return "goclaw v" + v
}

func (m *Model) footerPrimaryStatus() string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	status := strings.TrimSpace(m.statusLine)
	if m.spinnerActive {
		spin := strings.TrimSpace(m.spinner.View())
		base := strings.TrimSpace(status)
		if base == "" {
			if m.assistantPlaceholder {
				base = th.StatusBarLabel.Render("Thinking") + th.FooterDim.Render("…")
			} else {
				base = th.StatusBarLabel.Render("Responding") + th.FooterDim.Render("…")
			}
		}
		return strings.TrimSpace(spin + "  " + base)
	}
	return status
}

func (m *Model) View() tea.View {
	// Keep viewport height in sync with the footer on every frame. Footer line count changes when
	// the spinner/status row appears or disappears; if we only relied on layout() from resize/typing,
	// the transcript viewport could be sized for the wrong footer and clip or overlap the UI.
	m.layout()
	content := lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		m.footerRendered,
	)
	v := tea.NewView(content)
	v.AltScreen = true
	// MouseModeNone lets the terminal handle mouse events natively so the user can select and copy text
	// without holding Shift. Scroll with PgUp/PgDn, j/k, Ctrl+U/Ctrl+D (or the wheel on terminals that
	// deliver wheel events without mouse-reporting active, e.g. Windows Terminal in some configurations).
	v.MouseMode = tea.MouseModeNone
	return v
}

// rebuildWelcomeForWidth rebuilds the top welcome panel using the real terminal width so layout
// does not rely on a too-wide two-column block before the first WindowSizeMsg.
func (m *Model) rebuildWelcomeForWidth() {
	if m.welcomeBlockEnd <= 0 || m.width <= 0 {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	dash := WelcomeDashboardLines(th, m.welcomeOpts, m.width)
	if len(dash) == 0 {
		return
	}
	newHead := append(append([]string(nil), dash...), "")
	oldEnd := m.welcomeBlockEnd
	if len(newHead) != oldEnd {
		tail := append([]string(nil), m.lines[oldEnd:]...)
		m.lines = append(newHead, tail...)
	} else {
		copy(m.lines, newHead)
	}
	m.welcomeBlockEnd = len(newHead)
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
	for idx := 0; idx < len(m.lines)-1; idx++ {
		plain := strings.TrimSpace(stripANSI(m.lines[idx]))
		if strings.HasPrefix(plain, "goclaw") {
			m.lines[idx+1] = th.SeparatorLine(m.width)
			return
		}
	}
}

func (m *Model) layout() {
	foot := m.footerView()
	m.footerRendered = foot
	footerH := lipgloss.Height(foot)
	h := m.height - footerH
	if h < 1 {
		h = 1
	}
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(h)
	if m.helpOpen {
		m.viewport.SetContent(m.helpFullText)
		m.lastTranscript = m.helpFullText
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
		stick := m.viewport.AtBottom()
		m.viewport.SetContent(joined)
		m.lastTranscript = joined
		if stick {
			m.viewport.GotoBottom()
		}
	}
}

func (m *Model) openHelpOverlay(replBody string) {
	m.exitConfirmDeadline = time.Time{}
	m.themePickOpen = false
	m.themePickFullText = ""
	m.agentPickOpen = false
	m.agentPickFullText = ""
	var b strings.Builder
	if m.appVersion != "" {
		b.WriteString("goclaw · v")
		b.WriteString(m.appVersion)
	} else {
		b.WriteString("goclaw")
	}
	b.WriteString("\n\n")
	b.WriteString(slashcmd.TUIHelpShortcutsText())
	b.WriteString("\n\n")
	b.WriteString(strings.TrimSpace(replBody))
	m.helpFullText = b.String()
	m.helpOpen = true
	m.layout()
	m.viewport.GotoTop()
}

func (m *Model) closeHelpOverlay() {
	m.helpOpen = false
	m.helpFullText = ""
	m.layout()
	m.viewport.GotoBottom()
}

func (m *Model) footerView() string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	if m.helpOpen {
		line := "Esc · scroll · Ctrl+C quit"
		if m.width > 4 {
			return th.FooterDim.Width(m.width).Render(line)
		}
		return th.FooterDim.Render(line)
	}
	if m.themePickOpen {
		line := "↑↓ · Enter apply · Esc cancel · Ctrl+C quit"
		if m.width > 4 {
			return th.FooterDim.Width(m.width).Render(line)
		}
		return th.FooterDim.Render(line)
	}
	if m.agentPickOpen {
		line := "↑↓ · Enter apply · Esc cancel · Ctrl+C quit"
		if m.width > 4 {
			return th.FooterDim.Width(m.width).Render(line)
		}
		return th.FooterDim.Render(line)
	}

	fw := m.width
	primary := strings.TrimSpace(m.footerPrimaryStatus())
	hints := m.footerBrand() + " · Esc · double Ctrl+C to quit · PgUp/PgDn scroll"
	session := footerline.HintsWithSession(hints, m.sessionID, m.width)

	var b strings.Builder

	// Show primary status (spinner/thinking) only when active; skip the extra line when idle.
	if primary != "" {
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(primary))
		} else {
			b.WriteString(th.FooterDim.Render(primary))
		}
		b.WriteString("\n")
	}
	if fw > 4 {
		b.WriteString(th.FooterDim.Width(fw).Render(session))
	} else {
		b.WriteString(th.FooterDim.Render(session))
	}

	if m.focusLine != nil {
		if fh := strings.TrimSpace(m.focusLine()); fh != "" {
			b.WriteString("\n")
			if fw > 4 {
				b.WriteString(th.FooterDim.Width(fw).Render(fh))
			} else {
				b.WriteString(th.FooterDim.Render(fh))
			}
		}
	}

	if strip := m.prefixSuggestStripView(); strip != "" {
		b.WriteString("\n")
		b.WriteString(strip)
	}
	if m.pending != nil {
		b.WriteString("\n")
		b.WriteString(m.approvalStripView())
	}

	inputView := m.input.View()
	if m.width > 4 {
		inputView = th.InputBorder.Width(m.width - 2).Render(inputView)
	}
	b.WriteString("\n")
	b.WriteString(inputView)
	return b.String()
}

// prefixSuggestStripView shows @ path picks, / slash picks, or short ! / & hints above the input.
func (m *Model) prefixSuggestStripView() string {
	if m.helpOpen || m.themePickOpen || m.agentPickOpen || m.streaming || m.pending != nil {
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
		return ""
	}
	sugs := inputprefix.TUIAtPathSuggestions(m.workdir, frag)
	if len(sugs) == 0 {
		return ""
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	ruleW := m.width
	const maxPickRule = 52
	if ruleW > maxPickRule {
		ruleW = maxPickRule
	}
	maxW := m.width - 4
	if maxW < 40 {
		maxW = m.width
	}
	if maxW < 24 {
		maxW = 72
	}
	more := 0
	if len(sugs) > maxSlashSuggestRows {
		more = len(sugs) - maxSlashSuggestRows
		sugs = sugs[:maxSlashSuggestRows]
	}
	var b strings.Builder
	b.WriteString(th.SeparatorLine(ruleW))
	b.WriteString("\n")
	b.WriteString(th.SlashPickerDesc.Render(fmt.Sprintf("@ paths · max %d · Tab completes · workspace", maxSlashSuggestRows)))
	for _, s := range sugs {
		name := "@" + s.RelPath
		if s.IsDir {
			name += "/"
		}
		snippet := "dir"
		if !s.IsDir {
			snippet = "file"
		}
		nameW := lipgloss.Width(th.SlashPickerName.Render(name))
		budget := maxW - nameW - 2
		if budget < 8 {
			budget = 8
		}
		snippet = text.TruncateRunes(snippet, budget)
		line := lipgloss.JoinHorizontal(lipgloss.Top, th.SlashPickerName.Render(name), th.SlashPickerDesc.Render("  "+snippet))
		if lipgloss.Width(line) > maxW {
			line = th.SlashPickerName.Render(name)
		}
		b.WriteString("\n")
		b.WriteString(line)
	}
	if more > 0 {
		b.WriteString("\n")
		b.WriteString(th.SlashPickerDesc.Render(fmt.Sprintf("… +%d more — keep typing", more)))
	}
	return b.String()
}

// slashSuggestStripView renders filtered /commands above the input (single-line buffer only).
func (m *Model) slashSuggestStripView() string {
	raw := m.input.Value()
	sugs := slashcmd.TUISlashSuggestions(raw)
	if len(sugs) == 0 {
		return ""
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	ruleW := m.width
	const maxPickRule = 52
	if ruleW > maxPickRule {
		ruleW = maxPickRule
	}
	maxW := m.width - 4
	if maxW < 40 {
		maxW = m.width
	}
	if maxW < 24 {
		maxW = 72
	}
	more := 0
	if len(sugs) > maxSlashSuggestRows {
		more = len(sugs) - maxSlashSuggestRows
		sugs = sugs[:maxSlashSuggestRows]
	}
	var b strings.Builder
	b.WriteString(th.SeparatorLine(ruleW))
	b.WriteString("\n")
	b.WriteString(th.SlashPickerDesc.Render(fmt.Sprintf("/ commands · max %d shown · Tab · type to narrow", maxSlashSuggestRows)))
	for _, s := range sugs {
		nameW := lipgloss.Width(th.SlashPickerName.Render(s.Name))
		budget := maxW - nameW - 2
		if budget < 8 {
			budget = 8
		}
		snippet := text.TruncateRunes(s.Summary, budget)
		line := lipgloss.JoinHorizontal(lipgloss.Top, th.SlashPickerName.Render(s.Name), th.SlashPickerDesc.Render("  "+snippet))
		if lipgloss.Width(line) > maxW {
			line = th.SlashPickerName.Render(s.Name)
		}
		b.WriteString("\n")
		b.WriteString(line)
	}
	if more > 0 {
		b.WriteString("\n")
		b.WriteString(th.SlashPickerDesc.Render(fmt.Sprintf("… +%d more — keep typing", more)))
	}
	return b.String()
}

// bangAmpHintStripView shows one-line hints for ! and & prefix modes.
func (m *Model) bangAmpHintStripView() string {
	raw := strings.TrimSpace(m.input.Value())
	if strings.Contains(raw, "\n") || raw == "" {
		return ""
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	ruleW := m.width
	const maxPickRule = 52
	if ruleW > maxPickRule {
		ruleW = maxPickRule
	}
	var b strings.Builder
	switch {
	case raw == "!":
		b.WriteString(th.SeparatorLine(ruleW))
		b.WriteString("\n")
		b.WriteString(th.SlashPickerDesc.Render("! — bash tool (allowlisted) · type command · Enter runs · @ shows path picks"))
		return b.String()
	case raw == "&":
		b.WriteString(th.SeparatorLine(ruleW))
		b.WriteString("\n")
		b.WriteString(th.SlashPickerDesc.Render("& — spawn_agent (general-purpose) · one line · coordinator profile"))
		return b.String()
	default:
		return ""
	}
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

	title := th.ModalTitle.Render("⚡ Allow")
	sep := th.ToolCardBorder.Render(" │ ")
	hint := th.Dim.Render("  y/n/esc")

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

	previewRunes := len([]rune(previewPlain))
	if previewRunes > 240 {
		previewRunes = 240
	}
	for previewRunes >= 8 {
		prevShow := text.TruncateRunes(previewPlain, previewRunes)
		if s, ok := try(toolShow, prevShow); ok {
			return s
		}
		previewRunes -= 8
	}
	if s, ok := try(toolShow, "…"); ok {
		return s
	}
	toolSt := th.Tool.Render(text.TruncateRunes(toolShow, 12))
	return title + " " + toolSt + hint
}

// ─── Line append helpers ────────────────────────────────────────────────────

func (m *Model) appendSeparator() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.SeparatorLine(m.width))
	m.setLinesContent(true)
}

func (m *Model) appendSystem(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.System.Render(s))
	m.setLinesContent(true)
}

func (m *Model) appendError(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.ErrorStyle.Render(s))
	m.setLinesContent(true)
}

func (m *Model) appendUser(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, fmt.Sprintf("%s %s", th.UserPrefix(), renderAtRefChips(s, th)))
	m.setLinesContent(true)
}

// renderAtRefChips styles @path tokens inside a user message with the AtRefChip theme style.
// Only @ tokens that are preceded by start-of-string or whitespace are styled,
// which avoids false-positives like email addresses.
func renderAtRefChips(s string, th *Theme) string {
	if !strings.Contains(s, "@") {
		return s
	}
	runes := []rune(s)
	var b strings.Builder
	i := 0
	for i < len(runes) {
		r := runes[i]
		// Only treat @ as a chip anchor when it's at the start or after whitespace.
		if r == '@' && (i == 0 || runes[i-1] == ' ' || runes[i-1] == '\t') {
			j := i + 1
			for j < len(runes) && runes[j] != ' ' && runes[j] != '\t' && runes[j] != '\n' {
				j++
			}
			if j > i+1 { // at least one char after @
				token := string(runes[i:j])
				b.WriteString(th.AtRefChip.Render(token))
				i = j
				continue
			}
		}
		b.WriteRune(r)
		i++
	}
	return b.String()
}

func (m *Model) appendAssistant(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, fmt.Sprintf("%s %s", th.AssistantPrefix(), s))
	m.setLinesContent(true)
}

func (m *Model) appendAssistantDim(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	dim := th.Dim.Render(s)
	m.lines = append(m.lines, fmt.Sprintf("%s %s", th.AssistantPrefix(), dim))
	m.setLinesContent(false)
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
	if isAssistantDimPlaceholderLine(plain, pfx) {
		m.lines = m.lines[:len(m.lines)-1]
		m.setLinesContent(false)
	}
}

// isAssistantDimPlaceholderLine reports whether plain text is our dim "…" (or ASCII "...") assistant row.
func isAssistantDimPlaceholderLine(plain, assistantPlainPrefix string) bool {
	if !strings.HasPrefix(plain, assistantPlainPrefix) {
		return false
	}
	rest := strings.TrimSpace(plain[len(assistantPlainPrefix):])
	if strings.Contains(rest, "…") && strings.TrimSpace(strings.ReplaceAll(rest, "…", "")) == "" {
		return true
	}
	// ASCII ellipsis only
	if strings.Trim(rest, ".") == "" && len(rest) > 0 {
		return true
	}
	return false
}

func (m *Model) toolQueueStatusLine() string {
	if len(m.toolWaitQueue) == 0 {
		return ""
	}
	base := orchestrator.ToolWorkingPhrase(m.toolWaitQueue[0].name)
	if !m.toolWaitStartedAt.IsZero() {
		secs := int(time.Since(m.toolWaitStartedAt).Seconds())
		if secs >= 1 {
			base = fmt.Sprintf("%s (%ds)", base, secs)
		}
	}
	return base + "…"
}

func (m *Model) popToolJob() (pendingTool, bool) {
	if len(m.toolWaitQueue) == 0 {
		return pendingTool{}, false
	}
	j := m.toolWaitQueue[0]
	m.toolWaitQueue = m.toolWaitQueue[1:]
	return j, true
}

// appendToolDoneLine renders a completed tool call as a compact card (claw-code style).
func (m *Model) appendToolDoneLine(toolName, summary string, isError bool) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	label := orchestrator.ToolFinishedPhrase(toolName)
	truncatedSummary := ""
	if s := strings.TrimSpace(summary); s != "" {
		truncatedSummary = text.TruncateRunes(s, 96)
	}
	card := th.RenderToolCard(label, truncatedSummary, isError, m.width)
	m.lines = append(m.lines, card)
	m.setLinesContent(false)
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
	m.setLinesContent(false)
}

// normalizeAssistantMarkdownLines removes glamour's heavy left padding on wrapped
// paragraphs so we do not stack it on top of our gutter padding (looked like a staircase).
// Lines inside fenced code blocks are left unchanged.
func normalizeAssistantMarkdownLines(lines []string) []string {
	out := make([]string, len(lines))
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			out[i] = line
			continue
		}
		if inFence {
			out[i] = line
			continue
		}
		if i == 0 {
			out[i] = strings.TrimLeft(line, " \t")
			continue
		}
		// Continuation lines: strip runaway indentation from word-wrap, keep shallow list indents.
		n := countLeadingASCIISpaces(line)
		if n >= 4 {
			out[i] = strings.TrimLeft(line, " \t")
		} else {
			out[i] = line
		}
	}
	return out
}

func countLeadingASCIISpaces(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
		} else if r == '\t' {
			n += 4
		} else {
			break
		}
	}
	return n
}

// dropLeadingBlankLines removes empty / whitespace-only lines glamour often emits
// before the first paragraph (otherwise the prefix sits alone on its own row).
func dropLeadingBlankLines(lines []string) []string {
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i == 0 {
		return lines
	}
	return lines[i:]
}

// dropTrailingBlankLines removes empty lines Glamour often emits after the last paragraph.
func dropTrailingBlankLines(lines []string) []string {
	j := len(lines)
	for j > 0 && strings.TrimSpace(lines[j-1]) == "" {
		j--
	}
	if j == len(lines) {
		return lines
	}
	return lines[:j]
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
	prefixW := lipgloss.Width(prefix)
	// Render markdown in the column width that fits next to the assistant gutter.
	rendered := th.RenderMarkdown(raw, m.width, prefixW)

	mdLines := strings.Split(rendered, "\n")
	mdLines = normalizeAssistantMarkdownLines(mdLines)
	mdLines = dropLeadingBlankLines(mdLines)
	mdLines = dropTrailingBlankLines(mdLines)
	if len(mdLines) == 0 {
		return
	}
	// Align continuation lines with the visual width of the styled prefix + one space.
	padStr := strings.Repeat(" ", prefixW+1)
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
	m.setLinesContent(false)
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
