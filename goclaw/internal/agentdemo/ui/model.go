package ui

import (
	"context"
	"strings"
	"sync"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	lipv2 "charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"
	"github.com/okuzpe/goclaw/internal/agentdemo"
	"github.com/okuzpe/goclaw/internal/agentdemo/components"
)

// InteractMode is a demo-only UX mode (not goclaw agent profiles).
type InteractMode int

const (
	ModeChat InteractMode = iota
	ModeCode
	ModeAgent
)

func (m InteractMode) String() string {
	switch m {
	case ModeCode:
		return "code"
	case ModeAgent:
		return "agent"
	default:
		return "chat"
	}
}

// Model is the Bubble Tea root model for agentdemo.
type Model struct {
	ctx context.Context
	cfg agentdemo.Config

	sendMu sync.Mutex
	send   func(tea.Msg)

	st Styles

	width  int
	height int

	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model

	blocks   []string
	streamMu sync.Mutex
	stream   strings.Builder

	streaming    bool
	phase        string
	toolRunning  bool

	mode InteractMode

	showLogOverlay bool
	logLines       []string

	history   []string
	histNav   int
	histDraft string

	cancelLLM context.CancelFunc

	spinnerActive bool

	statusErr string

	inputLines int
}

// New builds a demo model. SetSender must be called before Run.
func New(ctx context.Context, cfg agentdemo.Config) *Model {
	st := defaultStyles()
	spin := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipv2.NewStyle().Foreground(lipv2.Color("86"))),
	)
	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.MouseWheelEnabled = true
	vp.SoftWrap = true
	m := &Model{
		ctx:        ctx,
		cfg:        cfg,
		st:         st,
		spinner:    spin,
		viewport:   vp,
		input:      components.NewComposer(st.Accent),
		phase:      "idle",
		inputLines: 1,
	}
	m.welcomeBlock()
	return m
}

// SetSender wires async tea.Send (set from Run after tea.NewProgram).
func (m *Model) SetSender(fn func(tea.Msg)) {
	m.sendMu.Lock()
	m.send = fn
	m.sendMu.Unlock()
}

func (m *Model) post(msg tea.Msg) {
	m.sendMu.Lock()
	fn := m.send
	m.sendMu.Unlock()
	if fn != nil {
		fn(msg)
	}
}
