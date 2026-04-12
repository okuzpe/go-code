package main

import (
	"context"
	"errors"
	"fmt"
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
	slashEnv.SessionModel = func() string { return rt.Cfg.Model() }
	slashEnv.SetSessionModel = func(id string) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("model id is empty")
		}
		switch strings.ToLower(strings.TrimSpace(rt.Cfg.Provider)) {
		case "ollama":
			rt.Cfg.OllamaModel = id
		case "openai_compatible":
			rt.Cfg.OpenAICompatModel = id
		default:
			return fmt.Errorf("/model applies to provider ollama or openai_compatible only (current: %s)", rt.Cfg.Provider)
		}
		orch.SetConfig(rt.Cfg)
		return nil
	}
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
		FooterStats: func() string {
			if rt.Sess == nil {
				return ""
			}
			n := rt.Sess.Len()
			if n <= 0 {
				return ""
			}
			tok := orchestrator.SessionMessagesTokenEstimate(rt.Sess.Messages, rt.Cfg.Provider)
			var base string
			if n == 1 {
				base = "1 msg"
			} else {
				base = fmt.Sprintf("%d msgs", n)
			}
			if tok >= 1 {
				base = fmt.Sprintf("%s · ~%d tokens", base, tok)
			}
			if pct, ok := orchestrator.SessionCompactionFillPercent(rt.Sess.Messages, rt.Cfg); ok {
				base = fmt.Sprintf("%s · compact~%d%%", base, pct)
			}
			if app.OllamaFunctionToolsDropped(rt) {
				return base + " · Ollama text-only"
			}
			return base
		},
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
