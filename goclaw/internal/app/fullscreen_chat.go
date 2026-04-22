package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/slashcmd"
	"github.com/okuzpe/goclaw/internal/ui/chat"
)

// FullscreenChat wires the production Bubble Tea fullscreen TUI (same binary surface as
// cmd/goclaw). Version is shown in the welcome panel (build with -ldflags "-X main.Version=...").
type FullscreenChat struct {
	Version string
}

// RunFullscreenChat implements FullscreenChatRunner.
func (f FullscreenChat) RunFullscreenChat(ctx context.Context, rt *ChatRuntime) error {
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
			return DoctorReportFromRuntime(ctx, rt), nil
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
		return fullscreenSetSessionModel(rt, orch, id)
	}
	slashEnv.ChatSubtitle = func() string {
		return FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), orch.ProfileName())
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

	submit := fullscreenNewChatSubmitter(rt, orch, focus)

	opts := chat.Options{
		TUIInteractMode: rt.Cfg.TUIInteractMode,
		TUIMouseScroll:  rt.Cfg.TUIMouseScroll,
		TUIIcons:        rt.Cfg.TUIIcons,
		CycleAgentProfile: func(backward bool) (slashcmd.UIHints, error) {
			hi, err := cycleTUIAgentProfile(ctx, rt, orch, &slashEnv, &sess, backward)
			rt.Sess = sess
			return hi, err
		},
		SlashContext: func() slashcmd.SlashContext {
			return slashcmd.SlashContext{SlashEnv: slashEnv, Mem: rt.MemStore, Orch: orch, Sess: &sess, Store: rt.Store}
		},
		PreSubmitSystemLines: func(userText string) []string {
			return MaybeCoordinatorToDirectProfile(rt, orch, userText, strings.TrimSpace(focus.Current()) != "")
		},
		Title:                     FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), orch.ProfileName()),
		SessionID:                 rt.Sess.ID,
		FooterStats:               func() string { return fullscreenTuiFooterStats(rt, orch) },
		Workdir:                   rt.Workdir,
		UserConfigDir:             rt.Cfg.UserConfigDir,
		UserAgentsDir:             rt.UserAgentsDir,
		ProjectAgentsDir:          rt.ProjectAgentsDir,
		AgentPickerHiddenProfiles: slices.Clone(rt.Cfg.AgentPickerHiddenProfiles),
		ActiveAgentProfile:        orch.ProfileName(),
		Theme:                     chat.NewThemeForAppearance(rt.Cfg.UIAppearance),
		Welcome:                   fullscreenWelcomeOptions(f.Version, rt, orch),
		FocusLine:                 focus.Hint,
	}
	return chat.RunApp(ctx, opts, approval, submit, slash)
}

func fullscreenSetSessionModel(rt *ChatRuntime, orch *orchestrator.Orchestrator, id string) error {
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

func fullscreenWelcomeOptions(uiVersion string, rt *ChatRuntime, orch *orchestrator.Orchestrator) chat.WelcomeOptions {
	p := orch.ActiveProfile()
	writeApprovalRequired := p.AllowsWorkspaceFileWrites() && rt.Cfg.YoloThreshold < 60
	sub := FormatChatWindowTitle(rt.Cfg.Provider, rt.Cfg.Model(), orch.ProfileName())
	if hint := coordinatorShortcutSubtitleHint(orch.ProfileName()); hint != "" {
		sub += hint
	}
	return chat.WelcomeOptions{
		Version:               uiVersion,
		Subtitle:              sub,
		Workdir:               rt.Workdir,
		Profile:               orch.ProfileName(),
		FileWriteToolsHidden:  !p.AllowsWorkspaceFileWrites(),
		HubDelegatesCoding:    p.AllowsSpawnAgentDelegation(),
		WriteApprovalRequired: writeApprovalRequired,
		OllamaWarning:         FormatOllamaWelcomeWarning(rt),
	}
}

func coordinatorShortcutSubtitleHint(profileName string) string {
	switch strings.ToLower(strings.TrimSpace(profileName)) {
	case "coordinator", "plan", "":
		return ""
	default:
		return " · hub: /profile coordinator"
	}
}

func fullscreenTuiFooterStats(rt *ChatRuntime, orch *orchestrator.Orchestrator) string {
	if rt.Sess == nil {
		return ""
	}
	n := rt.Sess.Len()
	if n <= 0 {
		return ""
	}
	live := rt.Sess.StreamingAssistantChars()
	switch config.NormalizeTUIFooterDensity(rt.Cfg.TUIFooterDensity) {
	case config.TUIFooterDensityDebug:
		return fullscreenTuiFooterStatsDebug(rt, orch, n, live)
	case config.TUIFooterDensityMinimal:
		return fullscreenTuiFooterStatsMinimal(rt, orch, n, live)
	default:
		return fullscreenTuiFooterStatsStandard(rt, orch, n, live)
	}
}

func fullscreenTuiFooterStatsMsgLine(n int) string {
	if n == 1 {
		return "1 msg"
	}
	return fmt.Sprintf("%d msgs", n)
}

func fullscreenTuiFooterStatsMinimal(rt *ChatRuntime, orch *orchestrator.Orchestrator, n int, live int) string {
	base := fmt.Sprintf("%s · %s", fullscreenTuiFooterStatsMsgLine(n), orch.ProfileName())
	if OllamaFunctionToolsDropped(rt) {
		base += " · Ollama text-only (no tool wire · /doctor)"
	}
	tok := orchestrator.SessionMessagesTokenEstimateLive(rt.Sess.Messages, live)
	pct, ok := orchestrator.SessionCompactionFillPercentLive(rt.Sess.Messages, rt.Cfg, live)
	if ok && pct >= 75 {
		if tok >= 1 {
			base = fmt.Sprintf("%s · ~%d tokens", base, tok)
		}
		base = fmt.Sprintf("%s · compact~%d%%", base, pct)
	}
	return base
}

func fullscreenTuiFooterStatsStandard(rt *ChatRuntime, orch *orchestrator.Orchestrator, n int, live int) string {
	base := fullscreenTuiFooterStatsMsgLine(n)
	if tok := orchestrator.SessionMessagesTokenEstimateLive(rt.Sess.Messages, live); tok >= 1 {
		base = fmt.Sprintf("%s · ~%d tokens", base, tok)
	}
	if pct, ok := orchestrator.SessionCompactionFillPercentLive(rt.Sess.Messages, rt.Cfg, live); ok {
		base = fmt.Sprintf("%s · compact~%d%%", base, pct)
	}
	if OllamaFunctionToolsDropped(rt) {
		base += " · Ollama text-only (no tool wire · /doctor)"
	}
	return fmt.Sprintf("%s · %s", base, orch.ProfileName())
}

func fullscreenTuiFooterStatsDebug(rt *ChatRuntime, orch *orchestrator.Orchestrator, n int, live int) string {
	base := fullscreenTuiFooterStatsMsgLine(n)
	if tok := orchestrator.SessionMessagesTokenEstimateLive(rt.Sess.Messages, live); tok >= 1 {
		base = fmt.Sprintf("%s · ~%d tokens", base, tok)
	}
	if pct, ok := orchestrator.SessionCompactionFillPercentLive(rt.Sess.Messages, rt.Cfg, live); ok {
		base = fmt.Sprintf("%s · compact~%d%%", base, pct)
	}
	if OllamaFunctionToolsDropped(rt) {
		base += " · Ollama text-only (no tool wire · /doctor)"
	}
	p := orch.ActiveProfile()
	hub := "hub:n"
	if strings.EqualFold(orch.ProfileName(), "coordinator") {
		hub = "hub:y"
	}
	toolsSummary := "all"
	if na := len(p.ToolAllowlist); na > 0 {
		toolsSummary = fmt.Sprintf("%d", na)
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

func fullscreenNewChatSubmitter(rt *ChatRuntime, orch *orchestrator.Orchestrator, focus *coordinator.FocusRouter) chat.Submitter {
	augment := func(err error) error {
		return AugmentOrchestratorErr(rt.Cfg.Model(), err)
	}

	return func(ctx context.Context, userText, interactMode string, sink orchestrator.StreamSink) (string, error) {
		orch.SetTurnInteractMode(interactMode)
		defer orch.ClearTurnInteractMode()
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
			reply, err := StreamMockAssistant(ctx, userText, sink, rt.Sess)
			return reply, augment(err)
		}
		if handled, err := RunLocalPrefixToolIfAny(ctx, rt.Mock, orch, rt.Sess, userText, sink, rt.Workdir); handled {
			return "", err
		}
		userText = ExpandInlineAtRefs(ctx, orch, userText)
		reply, err := orch.RunStreaming(ctx, userText, sink)
		return reply, augment(err)
	}
}
