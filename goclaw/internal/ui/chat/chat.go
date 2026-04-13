package chat

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
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
	lineMeta   []lineMeta // parallel to lines; used to reflow width-sensitive rows on resize
	streaming  bool
	statusLine string
	// footerHint is a one-line reminder under the session footer (e.g. after /plan save); cleared on next send.
	footerHint string
	// idleTranscriptHint reminds how to scroll long replies (PgUp, Alt+arrows, optional wheel); cleared on next send.
	idleTranscriptHint string

	// tuiMouseScroll mirrors config: wheel scroll on transcript (requires cell mouse mode in View).
	tuiMouseScroll bool
	// transcriptBrowse (Ctrl+B): input blurred; arrows/j/k/PgUp scroll the transcript like a pager.
	transcriptBrowse bool

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

	// toolRunLineIdx is parallel to toolWaitQueue (FIFO): transcript line index for each IN-flight tool row.
	// Tool results replace rows in the same order; if server result order ever diverged from use order, correlate by tool_use_id (not done here).
	toolRunLineIdx []int

	// thinkingLineIdx is the transcript line showing LLM prefill timing (-1 when none).
	thinkingLineIdx int

	// toolWaitStartedAt is when the first pending tool in the queue began (for elapsed seconds in the footer).
	toolWaitStartedAt time.Time

	// toolLog accumulates all completed tool calls in this session for the Ctrl+T overlay.
	toolLog       []toolLogEntry
	toolLogStart  time.Time // when the current in-flight tool started (for elapsed)
	toolLogOpen   bool
	toolLogCursor int  // index into toolLog; -1 = none
	toolLogDetail bool // true = showing full content of toolLog[toolLogCursor]
	toolLogText   string

	sessionID string

	focusLine   func() string
	footerStats func() string

	// preambleEnd is the exclusive line index after the static startup block (welcome dashboard
	// including its trailing blank, or title+intro+ready when no welcome). Session transcript updates preserve lines[:preambleEnd].
	preambleEnd int

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
	// footerStatsLine caches opts.FooterStats() so we do not re-scan the full session on every keystroke
	// (token estimate is O(n) in message count). Refreshed on footerTickMsg and stream/session events.
	footerStatsLine string
	// footerStatsStreamAt throttles cache refresh while streaming (live assistant bytes change often).
	footerStatsStreamAt time.Time

	// inputResizeLineCount avoids redundant layout() when textarea line count is unchanged (typing on one line).
	inputResizeLineCount int

	// @ path picker: throttle filesystem walks (WalkDir is too heavy to run every keypress).
	atSuggestLastWalk time.Time
	atSuggestLastOut  string
	// lastTranscript avoids redundant viewport.SetContent when only the footer/spinner changed (reduces flicker).
	lastTranscript string
	// lastReflowWidth is the terminal width used for the last transcript reflow (-1 = not yet).
	lastReflowWidth int

	// messageQueue holds user texts submitted while the model is busy; shown above the compose box until
	// assistantDoneMsg drains them (appendSeparator + appendUser + runDispatchAfterUserEcho in order).
	messageQueue []string

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

// toolLogEntry is a completed tool call stored in the session-scoped tool history.
type toolLogEntry struct {
	name    string
	summary string // input preview (from OnToolUse)
	content string // full result string (from OnToolResult; capped at display)
	isError bool
	elapsed time.Duration
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
	// FooterStats optional; when non-nil, its return value is shown in the idle footer (e.g. message count).
	FooterStats func() string
	// TUIMouseScroll enables mouse wheel on the transcript (see config.TUIMouseScroll).
	TUIMouseScroll bool
}

// SlashHandler runs a slash command. If modelSubmit is non-empty, send that text to the model after displaying out (e.g. /edit).
// When quit is true, err may be nil (caller normalizes /quit before the TUI).
// hints describe optional UI updates (welcome bar, transcript reload); ignored by callers that do not render a transcript.
type SlashHandler func(input string) (handled bool, out string, quit bool, modelSubmit string, hints slashcmd.UIHints, err error)

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
		preview := text.TruncateRunes(
			orchestrator.FormatToolUsePreview(toolName, toolInput),
			tuiToolApprovalPreviewMaxRunes,
		)
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

// idleTranscriptHintMinRunes triggers a one-line scroll hint after long assistant replies (e.g. plans).
const idleTranscriptHintMinRunes = 1200

const (
	tuiToolApprovalPreviewMaxRunes = 400
	approvalOverlayPreviewMaxRunes = 240
	approvalOverlayPreviewMinRunes = 8
	approvalOverlayPreviewStep     = 8
	toolQueueSummaryMaxRunes       = 55

	defaultTerminalWidthFallback = 80
	composePlaceholderNarrowMaxW = 60

	mdLineRunawayIndentMin = 4
	mdTabIndentSpaces      = 4
)

// minComposeWidth is the smallest textarea width when the buffer is short (avoids a tiny box).
const minComposeWidth = 28

// syncInputComposeWidth sets the compose textarea to the usable terminal width so rules,
// transcript, and input align edge-to-edge (soft-wrap still applies inside the widget).
// The InputBorder style uses a full rounded border (1 left-border + 1 left-pad + content +
// 1 right-pad + 1 right-border = content + 4), so subtract 4 from the terminal width.
func (m *Model) syncInputComposeWidth() {
	if m.width <= 4 {
		m.input.SetWidth(0)
		return
	}
	maxW := m.width - 4
	if maxW < minComposeWidth {
		maxW = minComposeWidth
	}
	m.input.SetWidth(maxW)
}

// maxSlashSuggestRows caps the TUI slash-command picker (single-line / buffer); keeps the UI compact.
const maxSlashSuggestRows = 5

// atSuggestWalkMinInterval limits how often TUI @ path suggestions rescan the workspace (WalkDir).
const atSuggestWalkMinInterval = 180 * time.Millisecond

func placeholderForWidth(termWidth int) string {
	const full = "Ask anything…  ! @ & /btw /help · Ctrl+B scroll · Shift+Enter newline"
	const narrow = "Ask anything…  /help · Ctrl+B scroll"
	if termWidth > 0 && termWidth < composePlaceholderNarrowMaxW {
		return narrow
	}
	return full
}

// transcriptScrollNavHint is a short footer line for reading long assistant output in the TUI.
func (m *Model) transcriptScrollNavHint() string {
	s := "Transcript: Ctrl+B browse · PgUp/PgDn · Alt+arrows"
	if m.tuiMouseScroll {
		return s + " · mouse wheel"
	}
	return s
}

func (m *Model) exitTranscriptBrowse() {
	if !m.transcriptBrowse {
		return
	}
	m.transcriptBrowse = false
	m.input.Focus()
	m.syncViewportKeyMapForCompose()
}

func (m *Model) enterTranscriptBrowse() {
	if m.transcriptBrowse {
		return
	}
	m.transcriptBrowse = true
	m.input.Blur()
	m.syncViewportKeyMapForCompose()
}

// composeViewportKeyMap avoids stealing Space, arrows, j/k/h/l, u, d, or b from the compose textarea.
// Half-page keys avoid ctrl+u / ctrl+d (textarea delete before/after cursor). alt+pg* helps when PgUp
// is not delivered by the host terminal. Plain PgUp/PgDn are routed to the transcript only (see Update).
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

func New(ctx context.Context, opts Options) Model {
	th := opts.Theme
	if th == nil {
		th = DefaultTheme()
	}

	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	// Wheel requires MouseModeCellMotion in View(); off by default (see config TUIMouseScroll / tui_mouse_scroll).
	vp.MouseWheelEnabled = opts.TUIMouseScroll
	vp.SoftWrap = true
	vp.KeyMap = composeViewportKeyMap()

	// Use textarea for multi-line input support (modern CLI standard).
	in := textarea.New()
	in.Placeholder = placeholderForWidth(0)
	in.Prompt = th.InputPrompt
	in.ShowLineNumbers = true
	in.SetHeight(1) // start compact, grows dynamically
	in.SetWidth(0)
	in.CharLimit = 0 // no char limit
	in.Focus()
	styles := in.Styles()
	styles.Focused.Prompt = styles.Focused.Prompt.Foreground(th.UserTag.GetForeground())
	styles.Blurred.Prompt = styles.Blurred.Prompt.Foreground(th.Dim.GetForeground())
	styles.Focused.LineNumber = styles.Focused.LineNumber.Foreground(th.FooterDim.GetForeground())
	styles.Focused.CursorLineNumber = styles.Focused.CursorLineNumber.Foreground(th.UserTag.GetForeground())
	styles.Blurred.LineNumber = styles.Blurred.LineNumber.Foreground(th.Dim.GetForeground())
	styles.Blurred.CursorLineNumber = styles.Blurred.CursorLineNumber.Foreground(th.Dim.GetForeground())
	in.SetStyles(styles)

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
		lineMeta:            nil,
		submitter:           new(submitRunner),
		curAssistantLineIdx: -1,
		thinkingLineIdx:     -1,
		sessionID:           strings.TrimSpace(opts.SessionID),
		focusLine:           opts.FocusLine,
		footerStats:         opts.FooterStats,
		appVersion:          strings.TrimSpace(opts.Welcome.Version),
		userConfigDir:       strings.TrimSpace(opts.UserConfigDir),
		workdir:             strings.TrimSpace(opts.Workdir),
		userAgentsDir:       strings.TrimSpace(opts.UserAgentsDir),
		projectAgentsDir:    strings.TrimSpace(opts.ProjectAgentsDir),
		activeAgentProfile:  strings.TrimSpace(opts.ActiveAgentProfile),
		lastReflowWidth:     -1,
		tuiMouseScroll:      opts.TUIMouseScroll,
	}
	if strings.TrimSpace(opts.Welcome.Version) != "" {
		if dash := WelcomeDashboardLines(th, opts.Welcome, 0); len(dash) > 0 {
			m.welcomeOpts = opts.Welcome
			for _, ln := range dash {
				m.lines = append(m.lines, ln)
				m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindPlain})
			}
			m.lines = append(m.lines, "")
			m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindPlain})
			m.welcomeBlockEnd = len(m.lines)
		}
	}
	if m.welcomeBlockEnd == 0 {
		if strings.TrimSpace(opts.Title) != "" {
			m.lines = append(m.lines, th.ModalTitle.Render(opts.Title))
			m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindPlain})
			m.lines = append(m.lines, th.SeparatorLine(0))
			m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindSeparator})
		}
		intro := SessionIntroSystemText(opts.Workdir)
		// Do not pin to bottom during startup — stick=true scrolls past a tall welcome panel.
		m.appendSystemStick(intro, false)
		m.appendSystemStick("Ready.", false)
		m.viewport.GotoTop()
	}
	if m.welcomeBlockEnd > 0 {
		m.preambleEnd = m.welcomeBlockEnd
		// First layout() will SetContent; keep scroll intent at top until user scrolls or sends.
		m.viewport.GotoTop()
	} else {
		m.preambleEnd = len(m.lines)
	}
	m.refreshFooterStatsCache()
	return m
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

// ctrlCExitArmExpiredMsg clears the exit-confirm window when the scheduled deadline passes.
// expected must match m.exitConfirmDeadline so restarts of the 3s window ignore stale ticks.
type ctrlCExitArmExpiredMsg struct {
	expected time.Time
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
		if mdl, cmd, handled := m.handleKeyString(msg.String()); handled {
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
		m.footerStatsStreamAt = time.Time{}
		m.refreshFooterStatsCache()
		m.exitTranscriptBrowse()
		m.streaming = true
		m.assistantPlaceholder = true
		m.statusLine = ""
		m.curAssistant.Reset()
		m.curAssistantLineIdx = -1
		m.appendThinkingRow()
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
		m.refreshAssistantLine()
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
		m.clearThinkingLine()
		rawLen := utf8.RuneCountInString(m.curAssistant.String())
		// Finalize the current segment with markdown rendering.
		m.finalizeCurrentSegment()
		if rawLen >= idleTranscriptHintMinRunes {
			m.idleTranscriptHint = m.transcriptScrollNavHint()
		}
		var drainCmd tea.Cmd
		if !msg.aborted {
			drainCmd = m.drainMessageQueue()
		} else if n := len(m.messageQueue); n > 0 {
			m.messageQueue = nil
			m.appendSystem(fmt.Sprintf("(cleared %d queued message(s) after cancel)", n))
		}
		return m, drainCmd
	case toolUseMsg:
		m.clearThinkingLine()
		// Finalize the pre-tool text with markdown rendering BEFORE resetting.
		m.finalizeCurrentSegment()
		// Reset buffer for the next assistant segment (post-tool).
		m.curAssistant.Reset()
		m.curAssistantLineIdx = -1
		m.toolWaitQueue = append(m.toolWaitQueue, pendingTool{name: msg.name, summary: msg.preview})
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
		job, ok := m.popToolJob()
		if !ok {
			job = pendingTool{name: msg.name, summary: ""}
		}
		if len(m.toolRunLineIdx) > 0 {
			lineIdx := m.toolRunLineIdx[0]
			m.toolRunLineIdx = m.toolRunLineIdx[1:]
			if lineIdx >= 0 && lineIdx < len(m.lineMeta) && m.lineMeta[lineIdx].kind == lineKindToolRunning {
				m.replaceToolRunningWithCard(lineIdx, job.name, job.summary, msg.content, msg.isError)
			} else {
				m.appendToolDoneLine(job.name, job.summary, msg.content, msg.isError)
			}
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
		if len(m.toolWaitQueue) > 0 {
			return m, tickToolWait()
		}
		return m, nil
	case compactNoticeMsg:
		m.appendSystem(fmt.Sprintf("⟳ Context compacted — %d messages summarized", msg.removed))
		return m, nil
	case errMsg:
		m.refreshFooterStatsCache()
		m.exitTranscriptBrowse()
		m.streaming = false
		m.assistantPlaceholder = false
		m.spinnerActive = false
		m.statusLine = ""
		m.clearThinkingLine()
		m.toolRunLineIdx = nil
		m.toolWaitQueue = nil
		if n := len(m.messageQueue); n > 0 {
			m.messageQueue = nil
			m.appendSystem(fmt.Sprintf("(dropped %d queued message(s) after error)", n))
		}
		m.idleTranscriptHint = ""
		m.stripAssistantPlaceholderLine()
		m.appendError(fmt.Sprintf("✗ %v", msg.err))
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

	// Drag-and-drop interception: convert file/dir paths pasted from the OS into @relpath tokens.
	if paste, isPaste := msg.(tea.PasteMsg); isPaste &&
		!m.streaming && m.pending == nil &&
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

func (m *Model) handleKeyString(k string) (tea.Model, tea.Cmd, bool) {
	if m.pending != nil {
		switch k {
		case "y", "Y", "enter":
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
		if m.transcriptBrowse {
			m.exitTranscriptBrowse()
			m.layout()
			return m, nil, true
		}
		return m, tea.Quit, true
	}
	switch k {
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
		m.exitTranscriptBrowse()
		m.lines = nil
		m.lineMeta = nil
		m.toolWaitQueue = nil
		m.assistantPlaceholder = false
		m.spinnerActive = false
		m.curAssistantLineIdx = -1
		m.idleTranscriptHint = ""
		m.setLinesContent(true)
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
	case "ctrl+t":
		if m.streaming || m.pending != nil {
			return m, nil, true
		}
		m.openToolLog()
		return m, nil, true
	case "ctrl+p":
		if m.streaming || m.pending != nil {
			return m, nil, true
		}
		m.openAgentPicker()
		return m, nil, true
	case "enter":
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

// runDispatchAfterUserEcho runs theme/agents/slash/model for a user line already appended to the transcript.
// Returns tea.Quit when a slash handler requests exit (same as the Enter path).
func (m *Model) runDispatchAfterUserEcho(txt string) tea.Cmd {
	if bareThemeSlashInput(txt) && strings.TrimSpace(m.userConfigDir) != "" {
		m.openThemePicker()
		return nil
	}
	if bareAgentsSlashInput(txt) && m.slashHandle != nil {
		m.openAgentPicker()
		return nil
	}
	if m.slashHandle != nil {
		handled, out, quit, modelSubmit, hints, err := m.slashHandle(txt)
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
			m.applySlashHints(hints)
			if strings.TrimSpace(modelSubmit) != "" && m.submitter != nil && m.submitter.fn != nil {
				m.runModelSubmit(modelSubmit)
			}
			if quit {
				return tea.Quit
			}
			return nil
		}
	}
	if m.submitter != nil && m.submitter.fn != nil {
		m.runModelSubmit(txt)
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

func (m *Model) runModelSubmit(userText string) {
	m.streaming = true
	m.statusLine = ""
	m.curAssistant.Reset()
	m.curAssistantLineIdx = -1
	m.toolRunLineIdx = nil
	m.clearThinkingLine()
	m.submitter.fn(userText)
}

// refreshFooterStatsCache recomputes the idle footer stats line (token / compact hints). Call after
// session changes or on the periodic footer tick — not on every keystroke (see tuiFooterStats).
func (m *Model) refreshFooterStatsCache() {
	if m.footerStats == nil {
		m.footerStatsLine = ""
		return
	}
	m.footerStatsLine = strings.TrimSpace(m.footerStats())
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
	if m.inputResizeLineCount == lineCount {
		return
	}
	m.inputResizeLineCount = lineCount
	m.input.SetHeight(lineCount)
	m.layout()
}

// viewportSetJoinedContent updates transcript content in the viewport, preserving scroll offset
// when the user was not pinned to the bottom (so periodic refreshes do not yank the view).
func (m *Model) viewportSetJoinedContent(joined string, stickToBottom bool) {
	// Empty viewport reports AtBottom() == true; do not jump to end on first paint (tall welcome
	// in a short CMD window should start scrolled to the top).
	stick := stickToBottom || (m.viewport.TotalLineCount() > 0 && m.viewport.AtBottom())
	preserveY := m.viewport.YOffset()
	m.viewport.SetContent(joined)
	m.lastTranscript = joined
	if stick {
		m.viewport.GotoBottom()
	} else {
		m.viewport.SetYOffset(preserveY)
	}
}

// setLinesContent refreshes the transcript in the viewport. If stickToBottom is true or the user
// was already at the bottom, scroll stays pinned to the latest output.
func (m *Model) setLinesContent(stickToBottom bool) {
	m.viewportSetJoinedContent(strings.Join(m.lines, "\n"), stickToBottom)
}

func (m *Model) syncInputPlaceholder() {
	m.input.Placeholder = placeholderForWidth(m.width)
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
	if m.tuiMouseScroll {
		// Enables wheel delivery to the bubbles viewport (selection behavior may change per terminal).
		v.MouseMode = tea.MouseModeCellMotion
	} else {
		// MouseModeNone keeps the host terminal’s normal mouse selection and copy.
		v.MouseMode = tea.MouseModeNone
	}
	return v
}

func (m *Model) applySlashHints(hints slashcmd.UIHints) {
	if hints.RefreshWelcome {
		if hints.WelcomeProfile != "" {
			m.welcomeOpts.Profile = hints.WelcomeProfile
			m.activeAgentProfile = hints.WelcomeProfile
		}
		if hints.WelcomeSubtitle != "" {
			m.welcomeOpts.Subtitle = hints.WelcomeSubtitle
		}
		if hints.WelcomeFileWriteToolsHidden != nil {
			m.welcomeOpts.FileWriteToolsHidden = *hints.WelcomeFileWriteToolsHidden
		}
		if hints.WelcomeHubDelegatesCoding != nil {
			m.welcomeOpts.HubDelegatesCoding = *hints.WelcomeHubDelegatesCoding
		}
		if hints.WelcomeWriteApprovalRequired != nil {
			m.welcomeOpts.WriteApprovalRequired = *hints.WelcomeWriteApprovalRequired
		}
		if m.welcomeBlockEnd > 0 {
			m.rebuildWelcomeForWidth()
			m.preambleEnd = m.welcomeBlockEnd
		}
	}
	if hints.ReloadTranscript != nil {
		m.rebuildTranscriptForSession(hints.ReloadTranscript)
	}
	if hints.FooterHint != nil {
		m.footerHint = strings.TrimSpace(*hints.FooterHint)
	}
}

func (m *Model) rebuildTranscriptForSession(s *session.Session) {
	if s == nil {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	var head []string
	var metaHead []lineMeta
	if m.preambleEnd > 0 && m.preambleEnd <= len(m.lines) {
		head = append([]string(nil), m.lines[:m.preambleEnd]...)
		if m.preambleEnd <= len(m.lineMeta) {
			metaHead = append([]lineMeta(nil), m.lineMeta[:m.preambleEnd]...)
		} else {
			m.syncLineMetaLen()
			if m.preambleEnd <= len(m.lineMeta) {
				metaHead = append([]lineMeta(nil), m.lineMeta[:m.preambleEnd]...)
			}
		}
	}
	body := strings.TrimSpace(s.PlainTranscript())
	if body == "" {
		body = "(no messages in this session)"
	}
	msg := "Resumed session " + s.ID + " (" + strconv.Itoa(s.Len()) + " messages)\n" + body
	m.sessionID = s.ID
	m.lines = append(head, th.System.Render(msg))
	m.lineMeta = append(metaHead, lineMeta{kind: lineKindPlain})
	m.setLinesContent(true)
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
	newPlain := make([]lineMeta, len(newHead))
	for i := range newPlain {
		newPlain[i] = lineMeta{kind: lineKindPlain}
	}
	if len(newHead) != oldEnd {
		tail := append([]string(nil), m.lines[oldEnd:]...)
		m.lines = append(newHead, tail...)
		var tailMeta []lineMeta
		if oldEnd <= len(m.lineMeta) {
			tailMeta = append([]lineMeta(nil), m.lineMeta[oldEnd:]...)
		}
		m.lineMeta = append(newPlain, tailMeta...)
	} else {
		copy(m.lines, newHead)
		for len(m.lineMeta) < len(newHead) {
			m.lineMeta = append(m.lineMeta, lineMeta{kind: lineKindPlain})
		}
		for i := 0; i < len(newHead); i++ {
			m.lineMeta[i] = lineMeta{kind: lineKindPlain}
		}
	}
	m.welcomeBlockEnd = len(newHead)
}

// reflowTitleSeparator widens the rule under the banner when the terminal resizes.
func (m *Model) reflowTitleSeparator() {
	if m.width <= 0 || len(m.lines) < 2 {
		return
	}
	// Welcome rows include "goclaw" in subtitles; replacing the next line would corrupt the panel.
	if m.welcomeBlockEnd > 0 {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	for idx := 0; idx < len(m.lines)-1; idx++ {
		plain := strings.TrimSpace(stripANSI(m.lines[idx]))
		if strings.HasPrefix(plain, "goclaw") {
			m.syncLineMetaLen()
			m.lines[idx+1] = th.SeparatorLine(m.width)
			m.lineMeta[idx+1] = lineMeta{kind: lineKindSeparator}
			return
		}
	}
}

func (m *Model) layout() {
	m.syncInputComposeWidth()
	foot := m.footerView()
	m.footerRendered = foot
	footerH := lipgloss.Height(foot)
	h := m.height - footerH
	if h < 1 {
		h = 1
	}
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(h)
	if m.toolLogOpen {
		m.viewport.SetContent(m.toolLogText)
		m.lastTranscript = m.toolLogText
		return
	}
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
		m.viewportSetJoinedContent(joined, false)
	}
}

func (m *Model) openHelpOverlay(replBody string) {
	m.exitTranscriptBrowse()
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
	m.syncViewportKeyMapForOverlay()
	m.layout()
	m.viewport.GotoTop()
}

func (m *Model) closeHelpOverlay() {
	m.helpOpen = false
	m.helpFullText = ""
	m.syncViewportKeyMapForCompose()
	m.layout()
	m.viewport.GotoBottom()
}

func (m *Model) footerView() string {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	if m.toolLogOpen {
		var line string
		if m.toolLogDetail {
			line = "Esc back · Ctrl+C quit"
		} else {
			line = "↑↓ move · Enter view · Esc close · Ctrl+C quit"
		}
		if m.width > 4 {
			return th.FooterDim.Width(m.width).Render(line)
		}
		return th.FooterDim.Render(line)
	}
	if m.helpOpen {
		line := "Esc · Ctrl+C quit"
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
	stats := strings.TrimSpace(m.footerStatsLine)

	var b strings.Builder

	// Status row: spinner/thinking indicator with accent label, right-aligned stats.
	// Shown only while active so idle UI does not grow an extra blank line.
	if primary != "" {
		var statusRow string
		if stats != "" && fw > 40 {
			// Right-align stats next to the primary status when there is room.
			statW := lipgloss.Width(stats)
			primW := lipgloss.Width(primary)
			gap := fw - primW - statW
			if gap < 2 {
				gap = 2
			}
			statusRow = primary + strings.Repeat(" ", gap) + th.FooterDim.Render(stats)
		} else {
			statusRow = primary
		}
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(statusRow))
		} else {
			b.WriteString(th.FooterDim.Render(statusRow))
		}
		b.WriteString("\n")
	} else if stats != "" {
		// Idle: show stats (context %, session id) on their own line.
		session := footerline.AlignedHintsSession("", stats, "", "", fw)
		if strings.TrimSpace(session) != "" {
			if fw > 4 {
				b.WriteString(th.FooterDim.Width(fw).Render(session))
			} else {
				b.WriteString(th.FooterDim.Render(session))
			}
			b.WriteString("\n")
		}
	}

	if fh := strings.TrimSpace(m.footerHint); fh != "" {
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(fh))
		} else {
			b.WriteString(th.FooterDim.Render(fh))
		}
		b.WriteString("\n")
	}
	if sh := strings.TrimSpace(m.idleTranscriptHint); sh != "" {
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(sh))
		} else {
			b.WriteString(th.FooterDim.Render(sh))
		}
		b.WriteString("\n")
	}
	if m.transcriptBrowse {
		line := "Browse: ↑↓ j/k PgUp · Ctrl+B editor · Esc back"
		if m.tuiMouseScroll {
			line += " · wheel"
		}
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(line))
		} else {
			b.WriteString(th.FooterDim.Render(line))
		}
		b.WriteString("\n")
	}

	if m.focusLine != nil {
		if fh := strings.TrimSpace(m.focusLine()); fh != "" {
			if fw > 4 {
				b.WriteString(th.FooterDim.Width(fw).Render(fh))
			} else {
				b.WriteString(th.FooterDim.Render(fh))
			}
			b.WriteString("\n")
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
	if qs := m.messageQueueStripView(); qs != "" {
		b.WriteString("\n")
		b.WriteString(qs)
	}

	// One line break before compose so hints/approval do not run into the input strip.
	b.WriteString("\n")

	inputView := m.input.View()
	if m.width > 4 {
		// Do not use Width() here: it pads the line with spaces to the terminal edge.
		inputView = th.InputBorder.Render(inputView)
	}
	b.WriteString(inputView)
	return b.String()
}

// prefixSuggestStripView shows @ path picks, / slash picks, or short ! / & hints above the input.
func (m *Model) prefixSuggestStripView() string {
	if m.toolLogOpen || m.helpOpen || m.themePickOpen || m.agentPickOpen || m.streaming || m.pending != nil || m.transcriptBrowse {
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
		m.atSuggestLastWalk = time.Time{}
		return ""
	}
	now := time.Now()
	if !m.atSuggestLastWalk.IsZero() && now.Sub(m.atSuggestLastWalk) < atSuggestWalkMinInterval {
		return m.atSuggestLastOut
	}
	sugs := inputprefix.TUIAtPathSuggestions(m.workdir, frag)
	if len(sugs) == 0 {
		m.atSuggestLastWalk = now
		m.atSuggestLastOut = ""
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
	out := b.String()
	m.atSuggestLastWalk = now
	m.atSuggestLastOut = out
	return out
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

const maxMessageQueueStripLines = 5

func queuePreviewOneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

// messageQueueStripView lists pending sends above the compose box (FIFO) while the model is busy.
func (m *Model) messageQueueStripView() string {
	n := len(m.messageQueue)
	if n == 0 {
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
	fw := m.width
	if fw <= 0 {
		fw = defaultTerminalWidthFallback
	}
	maxW := fw - 4
	if maxW < 16 {
		maxW = fw
	}

	var b strings.Builder
	b.WriteString(th.SeparatorLine(ruleW))
	b.WriteString("\n")
	msgWord := "messages"
	if n == 1 {
		msgWord = "message"
	}
	b.WriteString(th.SlashPickerDesc.Render(fmt.Sprintf("Queued · %d %s · sent in order when idle", n, msgWord)))
	b.WriteString("\n")

	show := n
	extra := 0
	if show > maxMessageQueueStripLines {
		extra = show - maxMessageQueueStripLines
		show = maxMessageQueueStripLines
	}
	for i := 0; i < show; i++ {
		preview := queuePreviewOneLine(m.messageQueue[i])
		num := fmt.Sprintf("%d.", i+1)
		budget := maxW - utf8.RuneCountInString(num) - 1
		if budget < 8 {
			budget = 8
		}
		line := num + " " + text.TruncateRunes(preview, budget)
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(line))
		} else {
			b.WriteString(th.FooterDim.Render(line))
		}
		b.WriteString("\n")
	}
	if extra > 0 {
		more := fmt.Sprintf("… and %d more", extra)
		if fw > 4 {
			b.WriteString(th.FooterDim.Width(fw).Render(more))
		} else {
			b.WriteString(th.FooterDim.Render(more))
		}
	}
	return strings.TrimRight(b.String(), "\n")
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

	previewRunes := utf8.RuneCountInString(previewPlain)
	if previewRunes > approvalOverlayPreviewMaxRunes {
		previewRunes = approvalOverlayPreviewMaxRunes
	}
	for previewRunes >= approvalOverlayPreviewMinRunes {
		prevShow := text.TruncateRunes(previewPlain, previewRunes)
		if s, ok := try(toolShow, prevShow); ok {
			return s
		}
		previewRunes -= approvalOverlayPreviewStep
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
	m.appendSeparatorMeta()
	m.setLinesContent(true)
}

func (m *Model) appendSystem(s string) {
	m.appendSystemStick(s, true)
}

func (m *Model) appendSystemStick(s string, stickToBottom bool) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.System.Render(s))
	m.appendPlainMeta()
	m.setLinesContent(stickToBottom)
}

func (m *Model) appendError(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, th.ErrorStyle.Render(s))
	m.appendPlainMeta()
	m.setLinesContent(true)
}

func (m *Model) appendUser(s string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	m.lines = append(m.lines, fmt.Sprintf("%s %s", th.UserPrefix(), renderAtRefChips(s, th)))
	m.appendPlainMeta()
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
	b.Grow(len(s) + 8)
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
	m.appendPlainMeta()
	m.setLinesContent(true)
}

func (m *Model) widthOrDefault() int {
	if m.width > 0 {
		return m.width
	}
	return defaultTerminalWidthFallback
}

func (m *Model) appendThinkingRow() {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	now := time.Now()
	idx := len(m.lines)
	m.lines = append(m.lines, th.RenderThinkingRow(0, m.widthOrDefault()))
	m.appendThinkingMeta(now)
	m.thinkingLineIdx = idx
	m.setLinesContent(false)
}

func (m *Model) removeTranscriptLineAt(at int) {
	if at < 0 || at >= len(m.lines) {
		return
	}
	for i := range m.toolRunLineIdx {
		if m.toolRunLineIdx[i] > at {
			m.toolRunLineIdx[i]--
		}
	}
	if m.curAssistantLineIdx > at {
		m.curAssistantLineIdx--
	}
	if m.thinkingLineIdx == at {
		m.thinkingLineIdx = -1
	} else if m.thinkingLineIdx > at {
		m.thinkingLineIdx--
	}
	m.lines = append(m.lines[:at], m.lines[at+1:]...)
	if at < len(m.lineMeta) {
		m.lineMeta = append(m.lineMeta[:at], m.lineMeta[at+1:]...)
	}
}

func (m *Model) clearThinkingLine() {
	if m.thinkingLineIdx < 0 {
		return
	}
	idx := m.thinkingLineIdx
	if idx >= len(m.lines) || idx >= len(m.lineMeta) {
		m.thinkingLineIdx = -1
		return
	}
	if m.lineMeta[idx].kind != lineKindThinking {
		m.thinkingLineIdx = -1
		return
	}
	m.removeTranscriptLineAt(idx)
	m.thinkingLineIdx = -1
	m.setLinesContent(false)
}

func (m *Model) refreshThinkingTranscriptRow() {
	if m.thinkingLineIdx < 0 || !m.assistantPlaceholder {
		return
	}
	idx := m.thinkingLineIdx
	if idx < 0 || idx >= len(m.lines) || idx >= len(m.lineMeta) {
		return
	}
	if m.lineMeta[idx].kind != lineKindThinking {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	elapsed := int(time.Since(m.lineMeta[idx].startedAt).Seconds())
	m.lines[idx] = th.RenderThinkingRow(elapsed, m.widthOrDefault())
	m.setLinesContent(false)
}

func (m *Model) refreshToolRunningTranscriptRows() {
	if len(m.toolRunLineIdx) == 0 || len(m.toolWaitQueue) == 0 {
		return
	}
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	changed := false
	for i := range m.toolRunLineIdx {
		if i >= len(m.toolWaitQueue) {
			break
		}
		lineIdx := m.toolRunLineIdx[i]
		if lineIdx < 0 || lineIdx >= len(m.lines) || lineIdx >= len(m.lineMeta) {
			continue
		}
		if m.lineMeta[lineIdx].kind != lineKindToolRunning {
			continue
		}
		job := m.toolWaitQueue[i]
		elapsed := int(time.Since(m.lineMeta[lineIdx].startedAt).Seconds())
		label := orchestrator.ToolWorkingPhrase(job.name)
		m.lines[lineIdx] = th.RenderToolInProgressRow(label, job.summary, elapsed, m.widthOrDefault())
		changed = true
	}
	if changed {
		m.setLinesContent(false)
	}
}

func (m *Model) appendToolRunningTranscriptRow(toolName, preview string) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	now := time.Now()
	lineIdx := len(m.lines)
	label := orchestrator.ToolWorkingPhrase(toolName)
	m.lines = append(m.lines, th.RenderToolInProgressRow(label, preview, 0, m.widthOrDefault()))
	m.appendToolRunningMeta(toolName, preview, now)
	m.toolRunLineIdx = append(m.toolRunLineIdx, lineIdx)
	m.setLinesContent(false)
}

func (m *Model) replaceToolRunningWithCard(lineIdx int, toolName, summary, content string, isError bool) {
	th := m.theme
	if th == nil {
		th = DefaultTheme()
	}
	label := orchestrator.ToolFinishedPhrase(toolName)
	truncatedSummary := ""
	if s := strings.TrimSpace(summary); s != "" {
		truncatedSummary = text.TruncateRunes(s, 96)
	}
	if lineIdx < 0 || lineIdx >= len(m.lines) {
		m.appendToolDoneLine(toolName, summary, content, isError)
		return
	}
	card := th.RenderToolCard(label, truncatedSummary, isError, m.widthOrDefault())
	m.lines[lineIdx] = card
	m.syncLineMetaLen()
	if lineIdx < len(m.lineMeta) {
		m.lineMeta[lineIdx] = lineMeta{
			kind:        lineKindToolCard,
			toolName:    toolName,
			toolSummary: summary,
			toolError:   isError,
		}
	}
	m.setLinesContent(false)
	m.appendToToolLog(toolName, summary, content, isError)
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
		if len(m.lineMeta) == len(m.lines)+1 {
			m.lineMeta = m.lineMeta[:len(m.lineMeta)-1]
		}
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
	job := m.toolWaitQueue[0]
	base := orchestrator.ToolWorkingPhrase(job.name)
	if summary := strings.TrimSpace(job.summary); summary != "" {
		summary = text.TruncateRunes(summary, toolQueueSummaryMaxRunes)
		base = base + " · " + summary
	}
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

// appendToolDoneLine renders a completed tool call as a compact card (claw-code style)
// and records it in the session tool log for Ctrl+T drill-down.
func (m *Model) appendToolDoneLine(toolName, summary, content string, isError bool) {
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
	m.appendToolCardMeta(toolName, summary, isError)
	m.setLinesContent(false)
	m.appendToToolLog(toolName, summary, content, isError)
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
		m.appendPlainMeta()
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
		if n >= mdLineRunawayIndentMin {
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
		switch r {
		case ' ':
			n++
		case '\t':
			n += mdTabIndentSpaces
		default:
			return n
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
	finalRendered := renderAssistantMarkdownSegment(th, raw, m.width, prefix, prefixW)
	if strings.TrimSpace(finalRendered) == "" {
		return
	}

	// Replace the tracked streaming line with the markdown version.
	if m.curAssistantLineIdx >= 0 && m.curAssistantLineIdx < len(m.lines) {
		m.lines[m.curAssistantLineIdx] = finalRendered
		m.setAssistantMDMetaAt(m.curAssistantLineIdx, raw)
	} else {
		m.lines = append(m.lines, finalRendered)
		m.appendAssistantMDMeta(raw)
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

// assistantDoneMsg ends one model submit turn. aborted is true when the submit goroutine exited with context.Canceled (Esc / Ctrl+C).
type assistantDoneMsg struct {
	aborted bool
}

type toolUseMsg struct {
	name    string
	preview string
}

type toolResultMsg struct {
	name    string
	content string // full result string (used for tool log drill-down)
	isError bool
}

// compactNoticeMsg is sent when automatic context compaction removes messages.
type compactNoticeMsg struct{ removed int }

type errMsg struct {
	err error
}

// footerTickMsg refreshes the footer (token/compact hints) on a timer so values update during long streams or tool waits.
type footerTickMsg struct{}

func footerStatsTickCmd() tea.Cmd {
	return tea.Tick(600*time.Millisecond, func(time.Time) tea.Msg { return footerTickMsg{} })
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
					p.Send(assistantDoneMsg{aborted: true})
				} else {
					p.Send(errMsg{err: err})
				}
			}
		}()
	}

	_, err := p.Run()
	return err
}
