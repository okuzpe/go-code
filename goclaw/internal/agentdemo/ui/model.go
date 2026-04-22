package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipv2 "charm.land/lipgloss/v2"
	"github.com/okuzpe/goclaw/internal/agentdemo"
	"github.com/okuzpe/goclaw/internal/agentdemo/components"
	corellm "github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/replhistory"
)

// Phase represents the current agent execution phase.
type Phase string

const (
	PhaseIdle      Phase = "idle"
	PhaseThinking  Phase = "thinking"
	PhaseStreaming Phase = "streaming"
	PhaseExecuting Phase = "executing"
)

// maxBlocks caps the transcript block slice to prevent unbounded memory growth.
const maxBlocks = 500

// InteractMode is a demo-only UX mode (not goclaw agent profiles).
type InteractMode int

const (
	ModeChat InteractMode = iota
	ModeAgent
)

func (m InteractMode) String() string {
	if m == ModeAgent {
		return "agent"
	}
	return "chat"
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

	streaming     bool
	phase         Phase     // current execution phase
	currentTool   string    // name of the executing tool; empty when idle
	lastResult    string    // "ok" | "error" | "" — outcome of the last stream
	toolStartTime time.Time // when the current tool execution began

	tokenCount  int  // cached approx token estimate (invalidated by block mutations)
	tokensDirty bool // true when tokenCount needs recomputation

	mode InteractMode

	showLogOverlay bool
	logLines       []string

	history   []string
	histNav   int
	histDraft string

	cancelLLM context.CancelFunc

	agentRunner *AgentRunner

	// chatHistory carries conversation context across ModeChat turns.
	chatHistory []corellm.Message

	// demoConfigDir is used for persistent history via replhistory.
	demoConfigDir string

	inputLines int
}

func (m *Model) spinnerRunning() bool {
	return m.phase == PhaseThinking || m.phase == PhaseExecuting
}

func demoCfgDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".agentdemo")
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
	cfgDir := demoCfgDir()
	hist, _ := replhistory.Load(cfgDir)
	m := &Model{
		ctx:           ctx,
		cfg:           cfg,
		st:            st,
		spinner:       spin,
		viewport:      vp,
		input:         components.NewComposer(st.Accent),
		phase:         PhaseIdle,
		inputLines:    1,
		history:       hist,
		demoConfigDir: cfgDir,
	}
	m.welcomeBlock()
	runner, err := NewAgentRunner(cfg)
	if err != nil {
		m.appendSystemBlock("agent mode unavailable: " + err.Error())
	} else {
		m.agentRunner = runner
	}
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
