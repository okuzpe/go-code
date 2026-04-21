package ui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/okuzpe/goclaw/internal/agentdemo"
)

// Run starts the fullscreen demo TUI.
func Run(ctx context.Context, cfg agentdemo.Config) error {
	if ctx == nil {
		ctx = context.Background()
	}
	m := New(ctx, cfg)
	var p *tea.Program
	m.SetSender(func(msg tea.Msg) {
		if p != nil {
			p.Send(msg)
		}
	})
	opts, cleanup := agentdemo.ProgramTTYOpts()
	defer cleanup()
	p = tea.NewProgram(m, opts...)
	_, err := p.Run()
	return err
}
