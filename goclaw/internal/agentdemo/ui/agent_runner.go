package ui

import (
	"context"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/okuzpe/goclaw/internal/agentdemo"
	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
)

// AgentRunner wraps a real orchestrator for the agentdemo agent mode.
// It registers a safe read-only tool subset (read_file, glob, grep) and
// auto-approves all tools so the demo runs without permission prompts.
type AgentRunner struct {
	orch *orchestrator.Orchestrator
}

// NewAgentRunner constructs an AgentRunner from the agentdemo config.
// Returns an error only if the working directory cannot be resolved.
func NewAgentRunner(cfg agentdemo.Config) (*AgentRunner, error) {
	workdir := cfg.Workdir
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	goCfg := config.Default()
	goCfg.OllamaHost = cfg.OllamaHost
	goCfg.OllamaModel = cfg.Model

	client := llm.NewOllama(cfg.OllamaHost)
	sess := session.New()

	reg := tools.New()
	reg.Register(tools.NewReadFile(workdir))
	reg.Register(tools.NewGlob(workdir))
	reg.Register(tools.NewGrep(workdir))
	if cfg.Unsafe {
		reg.Register(tools.NewWriteFile(workdir))
		reg.Register(tools.NewEditFile(workdir))
		reg.Register(tools.NewBash())
	}

	policy := permissions.NewPolicy()
	for _, spec := range reg.Specs() {
		policy.Set(spec.Name, permissions.ModeAllow)
	}

	orch := orchestrator.New(
		goCfg,
		client,
		sess,
		reg,
		policy,
		hooks.New(),
		agents.GeneralPurpose,
		orchestrator.WithWorkdir(workdir),
	)

	return &AgentRunner{orch: orch}, nil
}

// Run executes one agent turn, streaming events to sink.
func (r *AgentRunner) Run(ctx context.Context, input string, sink orchestrator.StreamSink) error {
	_, err := r.orch.RunStreaming(ctx, input, sink)
	return err
}

// bubbleTeaSink implements orchestrator.StreamSink and forwards events to
// the Bubble Tea model via the post function.
type bubbleTeaSink struct {
	post func(tea.Msg)
}

func (s *bubbleTeaSink) OnThinkingStart(phase string) {
	s.post(agentThinkingMsg{phase: phase})
}

func (s *bubbleTeaSink) OnTextDelta(text string) {
	s.post(streamDeltaMsg{text: text})
}

func (s *bubbleTeaSink) OnToolUse(_, name, _ string) {
	s.post(agentToolStartMsg{name: name})
}

func (s *bubbleTeaSink) OnToolResult(_, name, content string, isError bool) {
	s.post(agentToolDoneMsg{name: name, content: content, isError: isError})
}

func (s *bubbleTeaSink) OnToolProgress(_ string, partial string) {
	if partial != "" {
		s.post(streamDeltaMsg{text: partial})
	}
}

// OnDone is intentionally a no-op: the goroutine in stream.go posts streamDoneMsg
// after Run() returns, which finalizes the transcript and resets phase state.
func (s *bubbleTeaSink) OnDone(_ string) {}

func (s *bubbleTeaSink) OnCompact(_ int) {}
