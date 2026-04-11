package main

import (
	"context"
	"errors"
	"strings"

	"github.com/okuzpe/goclaw/internal/app"
	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/okuzpe/goclaw/internal/ui/chat"
)

type fullscreenChat struct{}

func (fullscreenChat) RunFullscreenChat(ctx context.Context, rt *app.ChatRuntime) error {
	approval := chat.NewApprovalBroker()
	orchOpts := append(append([]orchestrator.Option(nil), rt.OrchOpts...), orchestrator.WithToolApprover(approval.ToolApprover()))
	orch := orchestrator.New(rt.Cfg, rt.Client, rt.Sess, rt.Reg, rt.Policy, rt.HookReg, rt.Profile, orchOpts...)

	focus := coordinator.NewFocusRouter()
	slashEnv := slashcmd.SlashEnv{
		Workdir:                     rt.Workdir,
		UserConfigDir:               rt.Cfg.UserConfigDir,
		DisableInteractiveThemePick: true,
		DisableInteractiveAgentPick: true,
		Profs:                       rt.Profs,
		UserAgentsDir:               rt.UserAgentsDir,
		ProjectAgentsDir:            rt.ProjectAgentsDir,
		Focus:                       focus,
		Doctor: func(ctx context.Context) (string, error) {
			return app.DoctorReportFromRuntime(ctx, rt), nil
		},
	}
	sess := rt.Sess
	slashEnv.ChatSubtitle = func() string {
		return app.FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), orch.ProfileName())
	}
	slash := func(input string) (handled bool, out string, quit bool, modelSubmit string, err error, hints slashcmd.UIHints) {
		sc := slashcmd.SlashContext{SlashEnv: slashEnv, Mem: rt.MemStore, Orch: orch, Sess: &sess, Store: rt.Store}
		var hi slashcmd.UIHints
		h, o, q, ms, e := slashcmd.HandleSlash(ctx, sc, input, &hi)
		rt.Sess = sess
		if q && errors.Is(e, slashcmd.ErrReplQuit) {
			return h, o, q, ms, nil, hi
		}
		return h, o, q, ms, e, hi
	}
	var submit chat.Submitter
	if rt.Mock {
		submit = func(ctx context.Context, userText string, sink orchestrator.StreamSink) (string, error) {
			if id := strings.TrimSpace(focus.Current()); id != "" {
				err := coordinator.DeliverWorkerMessage(ctx, id, userText, sink)
				return "", app.AugmentOrchestratorErr(rt.Cfg.Provider, rt.Cfg.Model(), err)
			}
			reply, err := app.StreamMockAssistant(ctx, userText, sink, rt.Sess)
			return reply, app.AugmentOrchestratorErr(rt.Cfg.Provider, rt.Cfg.Model(), err)
		}
	} else {
		submit = func(ctx context.Context, userText string, sink orchestrator.StreamSink) (string, error) {
			if id := strings.TrimSpace(focus.Current()); id != "" {
				err := coordinator.DeliverWorkerMessage(ctx, id, userText, sink)
				return "", app.AugmentOrchestratorErr(rt.Cfg.Provider, rt.Cfg.Model(), err)
			}
			if handled, err := app.RunLocalPrefixToolIfAny(ctx, rt.Mock, orch, rt.Sess, userText, sink); handled {
				return "", err
			}
			userText = app.ExpandInlineAtRefs(ctx, orch, userText)
			reply, err := orch.RunStreaming(ctx, userText, sink)
			return reply, app.AugmentOrchestratorErr(rt.Cfg.Provider, rt.Cfg.Model(), err)
		}
	}
	return chat.RunApp(ctx, chat.Options{
		Title:              app.FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), rt.Profile.Name),
		SessionID:          rt.Sess.ID,
		Workdir:            rt.Workdir,
		UserConfigDir:      rt.Cfg.UserConfigDir,
		UserAgentsDir:      rt.UserAgentsDir,
		ProjectAgentsDir:   rt.ProjectAgentsDir,
		ActiveAgentProfile: rt.Profile.Name,
		Theme:              chat.NewThemeForAppearance(rt.Cfg.UIAppearance),
		Welcome: chat.WelcomeOptions{
			Version:  Version,
			Subtitle: app.FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), rt.Profile.Name),
			Workdir:  rt.Workdir,
			Profile:  rt.Profile.Name,
		},
		FocusLine: focus.Hint,
	}, approval, submit, slash)
}
