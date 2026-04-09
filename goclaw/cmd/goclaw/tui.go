package main

import (
	"context"
	"errors"

	"github.com/okuzpe/goclaw/internal/app"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/okuzpe/goclaw/internal/ui/chat"
)

type fullscreenChat struct{}

func (fullscreenChat) RunFullscreenChat(ctx context.Context, rt *app.ChatRuntime) error {
	approval := chat.NewApprovalBroker()
	orchOpts := append(append([]orchestrator.Option(nil), rt.OrchOpts...), orchestrator.WithToolApprover(approval.ToolApprover()))
	orch := orchestrator.New(rt.Cfg, rt.Client, rt.Sess, rt.Reg, rt.Policy, rt.HookReg, rt.Profile, orchOpts...)

	slashEnv := slashcmd.SlashEnv{
		Workdir: rt.Workdir,
		Profs:   rt.Profs,
		Doctor: func(ctx context.Context) (string, error) {
			return app.DoctorReportFromRuntime(ctx, rt), nil
		},
	}
	sess := rt.Sess
	slash := func(input string) (handled bool, out string, quit bool, modelSubmit string, err error) {
		sc := slashcmd.SlashContext{SlashEnv: slashEnv, Mem: rt.MemStore, Orch: orch, Sess: &sess, Store: rt.Store}
		h, o, q, ms, e := slashcmd.HandleSlash(ctx, sc, input)
		rt.Sess = sess
		if q && errors.Is(e, slashcmd.ErrReplQuit) {
			return h, o, q, ms, nil
		}
		return h, o, q, ms, e
	}
	var submit chat.Submitter
	if rt.Mock {
		submit = func(ctx context.Context, userText string, sink orchestrator.StreamSink) (string, error) {
			reply, err := app.StreamMockAssistant(ctx, userText, sink, rt.Sess)
			return reply, app.AugmentOrchestratorErr(rt.Cfg.Provider, rt.Cfg.Model(), err)
		}
	} else {
		submit = func(ctx context.Context, userText string, sink orchestrator.StreamSink) (string, error) {
			reply, err := orch.RunStreaming(ctx, userText, sink)
			return reply, app.AugmentOrchestratorErr(rt.Cfg.Provider, rt.Cfg.Model(), err)
		}
	}
	return chat.RunApp(ctx, chat.Options{
		Title:     app.FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), rt.Profile.Name),
		SessionID: rt.Sess.ID,
	}, approval, submit, slash)
}
