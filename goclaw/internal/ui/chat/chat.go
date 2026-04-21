package chat

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/replhistory"
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/okuzpe/goclaw/internal/ui/icons"
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
	// browseToolCardLine is a transcript line index (lineKindToolCard) focused for [ ] / e in browse mode; -1 when none.
	browseToolCardLine int

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

	// lastThinkingPhase is the latest orchestrator thinking-phase label for the streaming footer.
	lastThinkingPhase string

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

	// docOverlay shows markdown (glamour) in the viewport for /help, /capabilities, /doctor, etc.
	docOverlayOpen     bool
	docOverlayTitle    string
	docOverlaySourceMD string
	appVersion         string

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
	agentPickerHidden  []string

	// profileBarRendered is built in layout() with the same height math as View() (profile strip between transcript and footer).
	profileBarRendered string

	// footerRendered is built once in layout() so View() joins the same string as used for height math
	// (avoids footer/sizing mismatch if textarea state differed between two footerView calls).
	footerRendered string
	// footerStatsLine caches opts.FooterStats() so we do not re-scan the full session on every keystroke
	// (token estimate is O(n) in message count). Refreshed on footerTickMsg and stream/session events.
	footerStatsLine string
	// footerStatsStreamAt throttles cache refresh while streaming (live assistant bytes change often).
	footerStatsStreamAt time.Time

	// turnHadWorkspaceWrite is true if this user turn completed at least one successful write_file / edit_file / patch (for post-turn git diff --stat in the transcript).
	turnHadWorkspaceWrite bool

	// inputResizeLineCount avoids redundant layout() when textarea line count is unchanged (typing on one line).
	inputResizeLineCount int

	// @ path picker: throttle filesystem walks (WalkDir is too heavy to run every keypress).
	atSuggestLastWalk time.Time
	atSuggestLastOut  string
	// lastTranscript avoids redundant viewport.SetContent when only the footer/spinner changed (reduces flicker).
	lastTranscript string

	// streamPaintAt / streamPaintSkip throttle full transcript joins during assistant streaming
	// (strings.Join of m.lines is O(n); NDJSON can still deliver many small deltas after sink batching).
	streamPaintAt   time.Time
	streamPaintSkip int
	// assistantHoldLastPaint throttles setLinesContent while assistantStreamHold shows a placeholder row.
	assistantHoldLastPaint time.Time
	// assistantRevealRunes / assistantRevealPos drive a short post-stream plain-text reveal before markdown.
	assistantRevealRunes []rune
	assistantRevealPos   int
	// pendingAssistantDone* holds assistantDone follow-up until reveal animation finishes.
	pendingAssistantDone    assistantDoneMsg
	pendingAssistantDoneSet bool
	pendingAssistantRaw     string
	// lastReflowWidth is the terminal width used for the last transcript reflow (-1 = not yet).
	lastReflowWidth int

	// cycleAgentProfileFn runs /profile-style switches from Ctrl+Shift+←/→ (nil disables shortcut + bar).
	cycleAgentProfileFn func(backward bool) (slashcmd.UIHints, error)

	// interactMode is a UI-only footer label (chat | code | agent), cycled with Ctrl+M.
	interactMode string

	// slashContextFn returns SlashContext for / argument completion (nil disables argument picker).
	slashContextFn func() slashcmd.SlashContext
	// preSubmitSystemLines optional; see Options.PreSubmitSystemLines.
	preSubmitSystemLines func(userText string) []string

	// messageQueue holds user texts submitted while the model is busy; shown above the compose box until
	// assistantDoneMsg drains them (appendSeparator + appendUser + runDispatchAfterUserEcho in order).
	messageQueue []string

	// replLines stores prior user submits (oldest first, newest last), loaded from ~/.goclaw/history and updated on each model turn.
	replLines     []string
	replHistDraft string
	replHistPos   int // 0 = editing freely; >=1 indexes backward from the newest line in replLines

	// exitConfirmDeadline is the wall-clock instant until which a second Ctrl+C quits the TUI (double Ctrl+C).
	exitConfirmDeadline time.Time
	// ctrlCMsgRendered holds the rendered "Press Ctrl+C again…" line so it can be removed when the deadline expires.
	ctrlCMsgRendered string
}

type toolTickMsg struct{}

// pendingTool holds human-readable tool activity for compact status + done lines.
type pendingTool struct {
	toolUseID string
	name      string
	summary   string
}

// toolLogEntry is a completed tool call stored in the session-scoped tool history.
type toolLogEntry struct {
	name    string
	summary string // input preview (from OnToolUse)
	outcome string // one-line result hint (same as transcript tool cards)
	content string // full result string (from OnToolResult; capped at display)
	isError bool
	elapsed time.Duration
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
	// AgentPickerHiddenProfiles excludes these profile names from the Ctrl+P picker (see config agent_picker_hidden_profiles).
	AgentPickerHiddenProfiles []string
	// Welcome optional bordered panel before the title (version + tips).
	Welcome WelcomeOptions
	// FocusLine optional; when non-nil, its return value is shown in the footer (e.g. worker focus).
	FocusLine func() string
	// FooterStats optional; when non-nil, its return value is shown in the idle footer (e.g. message count).
	FooterStats func() string
	// TUIMouseScroll enables mouse wheel on the transcript (see config.TUIMouseScroll).
	TUIMouseScroll bool
	// TUIIcons selects footer/workspace glyphs: emoji, unicode, ascii, nerd (canonical; see config.TUIIcons).
	TUIIcons string
	// SlashContext supplies live slash-command argument suggestions (optional; fullscreen TUI from cmd/goclaw).
	SlashContext func() slashcmd.SlashContext
	// PreSubmitSystemLines optional; each non-empty line is shown as a system transcript row before the
	// assistant placeholder for this user send (e.g. coordinator auto-profile notice).
	PreSubmitSystemLines func(userText string) []string
	// CycleAgentProfile switches to the next or previous agent profile (fullscreen TUI; Ctrl+Shift+←/→).
	CycleAgentProfile func(backward bool) (slashcmd.UIHints, error)
	// TUIInteractMode is the initial UI-only interact mode label (chat | code | agent); Ctrl+M cycles.
	// Empty defaults to chat (see config.NormalizeTUIInteractMode).
	TUIInteractMode string
}

// SlashHandler runs a slash command. If modelSubmit is non-empty, send that text to the model after displaying out (e.g. /edit).
// When quit is true, err may be nil (caller normalizes /quit before the TUI).
// hints describe optional UI updates (welcome bar, transcript reload); ignored by callers that do not render a transcript.
type SlashHandler func(input string) (handled bool, out string, quit bool, modelSubmit string, hints slashcmd.UIHints, err error)

// inputMaxHeight is the maximum number of visible lines in the input textarea.
const inputMaxHeight = 6

// assistantStreamHold: while true, assistant deltas only grow an in-memory buffer and the transcript
// shows a single lightweight placeholder row (avoids O(n) repaints on long sessions). On assistantDone,
// we briefly reveal plain text then markdown-finalize (see assistantRevealMinRunes).
const assistantStreamHold = true

const (
	assistantHoldPaintMin     = 160 * time.Millisecond
	assistantRevealMinRunes   = 120
	assistantRevealTick       = 16 * time.Millisecond
	assistantRevealChunkRunes = 32
)

// idleTranscriptHintMinRunes triggers a one-line scroll hint after long assistant replies (e.g. plans).
const idleTranscriptHintMinRunes = 1200

const (
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

// maxSlashSuggestRows caps the TUI slash-command picker (single-line / buffer); keeps the UI compact.
const maxSlashSuggestRows = 5

// atSuggestWalkMinInterval limits how often TUI @ path suggestions rescan the workspace (WalkDir).
const atSuggestWalkMinInterval = 180 * time.Millisecond

func placeholderForWidth(termWidth int) string {
	const full = "Message…  @ paths · /commands · Tab completes · Ctrl+B transcript · Ctrl+M ui mode · Shift+Enter newline · /help for more"
	const narrow = "Message…  /help · Ctrl+B scroll"
	if termWidth > 0 && termWidth < composePlaceholderNarrowMaxW {
		return narrow
	}
	return full
}

func New(ctx context.Context, opts Options) Model {
	th := opts.Theme
	if th == nil {
		th = DefaultTheme()
	}
	th.Icons = icons.SetFromCanonical(opts.TUIIcons)

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
		ctx:                  ctx,
		viewport:             vp,
		input:                in,
		spinner:              spin,
		theme:                th,
		lines:                nil,
		lineMeta:             nil,
		submitter:            new(submitRunner),
		browseToolCardLine:   -1,
		curAssistantLineIdx:  -1,
		thinkingLineIdx:      -1,
		sessionID:            strings.TrimSpace(opts.SessionID),
		focusLine:            opts.FocusLine,
		footerStats:          opts.FooterStats,
		appVersion:           strings.TrimSpace(opts.Welcome.Version),
		userConfigDir:        strings.TrimSpace(opts.UserConfigDir),
		workdir:              strings.TrimSpace(opts.Workdir),
		userAgentsDir:        strings.TrimSpace(opts.UserAgentsDir),
		projectAgentsDir:     strings.TrimSpace(opts.ProjectAgentsDir),
		activeAgentProfile:   strings.TrimSpace(opts.ActiveAgentProfile),
		agentPickerHidden:    slices.Clone(opts.AgentPickerHiddenProfiles),
		lastReflowWidth:      -1,
		tuiMouseScroll:        opts.TUIMouseScroll,
		cycleAgentProfileFn:   opts.CycleAgentProfile,
		slashContextFn:        opts.SlashContext,
		preSubmitSystemLines:  opts.PreSubmitSystemLines,
		interactMode:            config.NormalizeTUIInteractMode(opts.TUIInteractMode),
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
	if d := strings.TrimSpace(opts.UserConfigDir); d != "" {
		if lines, err := replhistory.Load(d); err == nil && len(lines) > 0 {
			m.replLines = lines
		}
	}
	m.refreshFooterStatsCache()
	return m
}

type approvalMsg ApprovalRequest

func waitForApproval(ch <-chan ApprovalRequest) tea.Cmd {
	return func() tea.Msg {
		req := <-ch
		return approvalMsg(req)
	}
}

type assistantPlaceholderMsg struct{}

// systemTranscriptMsg appends one system-style line before streaming starts (e.g. profile auto-switch).
type systemTranscriptMsg string

type assistantDeltaMsg string

// assistantDoneMsg ends one model submit turn. aborted is true when the submit goroutine exited with context.Canceled (Esc / Ctrl+C).
type assistantDoneMsg struct {
	aborted bool
}

// assistantRevealTickMsg advances the post-stream plain-text reveal before markdown finalize.
type assistantRevealTickMsg struct{}

func assistantRevealTickCmd() tea.Cmd {
	return tea.Tick(assistantRevealTick, func(time.Time) tea.Msg { return assistantRevealTickMsg{} })
}

type toolUseMsg struct {
	toolUseID string
	name      string
	preview   string
}

type toolResultMsg struct {
	toolUseID string
	name      string
	content   string // full result string (used for tool log drill-down)
	isError   bool
}

// thinkingPhaseMsg updates the streaming footer and the in-transcript thinking row label.
type thinkingPhaseMsg struct {
	phase string
}

// thinkingRestartMsg re-shows the "Thinking…" row between tool iterations
// (the first iteration is handled by assistantPlaceholderMsg).
type thinkingRestartMsg struct {
	phase string
}

// toolProgressMsg carries an incremental output chunk from a running tool (e.g. a bash stdout line).
type toolProgressMsg struct {
	name    string
	partial string
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

type Submitter func(ctx context.Context, userText string, interactMode string, sink orchestrator.StreamSink) (string, error)

// RunApp runs a chat TUI wired to an orchestrator submitter and an optional slash handler.
func RunApp(ctx context.Context, opts Options, approval *ApprovalBroker, submit Submitter, slash SlashHandler) error {
	m := New(ctx, opts)
	m.approval = approval
	m.slashHandle = slash

	p := tea.NewProgram(&m)
	m.submitter.fn = func(userText, interactMode string) {
		reqCtx, cancel := context.WithCancel(ctx)
		m.submitter.setCancel(cancel)
		// Async model work must stay outside Model.Update; only send tea.Msg back into the program.
		go func(line, mode string) {
			defer func() {
				m.submitter.setCancel(nil)
				cancel()
			}()
			if m.preSubmitSystemLines != nil {
				for _, ln := range m.preSubmitSystemLines(line) {
					if t := strings.TrimSpace(ln); t != "" {
						p.Send(systemTranscriptMsg(t))
					}
				}
			}
			p.Send(assistantPlaceholderMsg{})
			sink := newBatchedProgramSink(p)
			_, err := submit(reqCtx, line, mode, sink)
			sink.flush()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					// User pressed Ctrl+C — clear streaming state cleanly.
					p.Send(assistantDoneMsg{aborted: true})
				} else {
					p.Send(errMsg{err: err})
				}
			}
		}(userText, interactMode)
	}

	_, err := p.Run()
	return err
}
