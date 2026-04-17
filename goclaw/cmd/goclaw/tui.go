package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/app"
	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/okuzpe/goclaw/internal/ui/chat"
)

type fullscreenChat struct{}

func (fullscreenChat) RunFullscreenChat(ctx context.Context, rt *app.ChatRuntime) error {
	approval := chat.NewApprovalBroker()

	orchOpts := slices.Clone(rt.OrchOpts)
	orchOpts = append(orchOpts, orchestrator.WithToolApprover(approval.ToolApprover()))
	orch := orchestrator.New(rt.Cfg, rt.Client, rt.Sess, rt.Reg, rt.Policy, rt.HookReg, rt.Profile, orchOpts...)

	focus := coordinator.NewFocusRouter()
	slashEnv := slashcmd.SlashEnv{
		Workdir:                     rt.Workdir,
		UserConfigDir:               rt.Cfg.UserConfigDir,
		FullscreenTUI:               true,
		DisableInteractiveThemePick: true,
		DisableInteractiveAgentPick: true,
		Profs:                       rt.Profs,
		UserAgentsDir:               rt.UserAgentsDir,
		ProjectAgentsDir:            rt.ProjectAgentsDir,
		Focus:                       focus,
		Doctor: func(ctx context.Context) (string, error) {
			return app.DoctorReportFromRuntime(ctx, rt), nil
		},
		OnProfileChange: func(p agents.Profile) {
			rt.Profile = p
		},
	}
	slashEnv.PlanGate = func() slashcmd.PlanGateConfig {
		return slashcmd.PlanGateConfig{
			RequireApplyApproval: rt.Cfg.PlanRequireApplyApproval,
			ApplyUseCoordinator:  rt.Cfg.PlanApplyUseCoordinator,
			AgentPickerHide:      rt.Cfg.AgentPickerHiddenProfiles,
		}
	}
	sess := rt.Sess
	slashEnv.SessionModel = func() string { return rt.Cfg.Model() }
	slashEnv.SetSessionModel = func(id string) error {
		return setSessionModel(rt, orch, id)
	}
	slashEnv.ChatSubtitle = func() string {
		return app.FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), orch.ProfileName())
	}

	slash := func(input string) (handled bool, out string, quit bool, modelSubmit string, hints slashcmd.UIHints, err error) {
		sc := slashcmd.SlashContext{SlashEnv: slashEnv, Mem: rt.MemStore, Orch: orch, Sess: &sess, Store: rt.Store}
		var hi slashcmd.UIHints
		h, o, q, ms, e := slashcmd.HandleSlash(ctx, sc, input, &hi)
		rt.Sess = sess
		if q && errors.Is(e, slashcmd.ErrReplQuit) {
			return h, o, q, ms, hi, nil
		}
		return h, o, q, ms, hi, e
	}

	submit := newChatSubmitter(rt, orch, focus)

	opts := chat.Options{
		TUIMouseScroll: rt.Cfg.TUIMouseScroll,
		TUIIcons:       rt.Cfg.TUIIcons,
		SlashContext: func() slashcmd.SlashContext {
			return slashcmd.SlashContext{SlashEnv: slashEnv, Mem: rt.MemStore, Orch: orch, Sess: &sess, Store: rt.Store}
		},
		PreSubmitSystemLines: func(userText string) []string {
			return app.MaybeCoordinatorToDirectProfile(rt, orch, userText, strings.TrimSpace(focus.Current()) != "")
		},
		Title:              app.FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), orch.ProfileName()),
		SessionID:          rt.Sess.ID,
		FooterStats:        func() string { return tuiFooterStats(rt, orch) },
		Workdir:            rt.Workdir,
		UserConfigDir:      rt.Cfg.UserConfigDir,
		UserAgentsDir:      rt.UserAgentsDir,
		ProjectAgentsDir:   rt.ProjectAgentsDir,
		AgentPickerHiddenProfiles: slices.Clone(rt.Cfg.AgentPickerHiddenProfiles),
		ActiveAgentProfile: orch.ProfileName(),
		Theme:              chat.NewThemeForAppearance(rt.Cfg.UIAppearance),
		Welcome:            welcomeOptions(rt, orch),
		FocusLine:          focus.Hint,
	}
	return chat.RunApp(ctx, opts, approval, submit, slash)
}

func setSessionModel(rt *app.ChatRuntime, orch *orchestrator.Orchestrator, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("model id is empty")
	}
	if strings.ToLower(strings.TrimSpace(rt.Cfg.Provider)) != "ollama" {
		return fmt.Errorf("/model applies to Ollama only (current provider: %s)", rt.Cfg.Provider)
	}
	rt.Cfg.OllamaModel = id
	orch.SetConfig(rt.Cfg)
	return nil
}

func welcomeOptions(rt *app.ChatRuntime, orch *orchestrator.Orchestrator) chat.WelcomeOptions {
	p := orch.ActiveProfile()
	// writeApprovalRequired is true when write tools are available in the profile
	// but the yolo_threshold is below the workspace-write risk score (60), meaning each
	// write_file / edit_file / patch call will trigger an interactive approval prompt.
	writeApprovalRequired := p.AllowsWorkspaceFileWrites() && rt.Cfg.YoloThreshold < 60
	return chat.WelcomeOptions{
		Version:               Version,
		Subtitle:              app.FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), orch.ProfileName()),
		Workdir:               rt.Workdir,
		Profile:               orch.ProfileName(),
		FileWriteToolsHidden:  !p.AllowsWorkspaceFileWrites(),
		HubDelegatesCoding:    p.AllowsSpawnAgentDelegation(),
		WriteApprovalRequired: writeApprovalRequired,
		OllamaWarning:         app.FormatOllamaWelcomeWarning(rt),
	}
}

func tuiFooterStats(rt *app.ChatRuntime, orch *orchestrator.Orchestrator) string {
	if rt.Sess == nil {
		return ""
	}
	n := rt.Sess.Len()
	if n <= 0 {
		return ""
	}
	live := rt.Sess.StreamingAssistantChars()

	base := "1 msg"
	if n != 1 {
		base = fmt.Sprintf("%d msgs", n)
	}
	if tok := orchestrator.SessionMessagesTokenEstimateLive(rt.Sess.Messages, live); tok >= 1 {
		base = fmt.Sprintf("%s · ~%d tokens", base, tok)
	}
	if pct, ok := orchestrator.SessionCompactionFillPercentLive(rt.Sess.Messages, rt.Cfg, live); ok {
		base = fmt.Sprintf("%s · compact~%d%%", base, pct)
	}
	if app.OllamaFunctionToolsDropped(rt) {
		base += " · Ollama text-only"
	}
	p := orch.ActiveProfile()
	hub := "hub:n"
	if strings.EqualFold(orch.ProfileName(), "coordinator") {
		hub = "hub:y"
	}
	toolsSummary := "all"
	if n := len(p.ToolAllowlist); n > 0 {
		toolsSummary = fmt.Sprintf("%d", n)
	}
	ro := "ro:n"
	if p.ReadOnly {
		ro = "ro:y"
	}
	base = fmt.Sprintf("%s · %s · %s · %s · tools:%s", base, orch.ProfileName(), ro, hub, toolsSummary)
	if role := orch.TaskRole(); role != "" && role != "default" {
		model := orch.TurnModel()
		if model == "" {
			model = rt.Cfg.Model()
		}
		tag := model
		if i := strings.LastIndex(model, ":"); i >= 0 {
			tag = model[i+1:]
		}
		base = fmt.Sprintf("%s · %s:%s", base, role, tag)
	}
	return base
}

func newChatSubmitter(rt *app.ChatRuntime, orch *orchestrator.Orchestrator, focus *coordinator.FocusRouter) chat.Submitter {
	augment := func(err error) error {
		return app.AugmentOrchestratorErr(rt.Cfg.Model(), err)
	}

	return func(ctx context.Context, userText string, sink orchestrator.StreamSink) (string, error) {
		if id := strings.TrimSpace(focus.Current()); id != "" {
			err := coordinator.DeliverWorkerMessage(ctx, id, userText, sink)
			if err != nil {
				if errors.Is(err, coordinator.ErrInteractiveWorkerNotFound) {
					focus.Detach()
				} else {
					return "", augment(err)
				}
			} else {
				return "", augment(err)
			}
		}
		if rt.Mock {
			reply, err := app.StreamMockAssistant(ctx, userText, sink, rt.Sess)
			return reply, augment(err)
		}
		if handled, err := app.RunLocalPrefixToolIfAny(ctx, rt.Mock, orch, rt.Sess, userText, sink, rt.Workdir); handled {
			return "", err
		}
		userText = app.ExpandInlineAtRefs(ctx, orch, userText)
		reply, err := orch.RunStreaming(ctx, userText, sink)
		return reply, augment(err)
	}
}
