package app

import (
	"context"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/orchestrator"
)

const localPrefixToolUseID = "prefix-invoke"

type sessionTurnOptions struct {
	ApplyAutoProfile bool
	ToolTrace        *[]orchestrator.JSONToolCall
}

func newAutomationOrchestrator(rt *ChatRuntime) *orchestrator.Orchestrator {
	return orchestrator.New(
		rt.Cfg,
		rt.Client,
		rt.Sess,
		rt.Reg,
		rt.Policy,
		rt.HookReg,
		rt.Profile,
		withAutomationOutputToolApprover(rt.OrchOpts)...,
	)
}

// runSessionTurn applies the shared pre-model pipeline for one user turn:
// optional auto-profile switching, mock mode, local prefix tools, inline @ refs,
// then the normal orchestrator loop.
func runSessionTurn(
	ctx context.Context,
	rt *ChatRuntime,
	orch *orchestrator.Orchestrator,
	userText string,
	sink orchestrator.StreamSink,
	opts sessionTurnOptions,
) (string, error) {
	if opts.ApplyAutoProfile {
		_ = MaybeCoordinatorToDirectProfile(rt, orch, userText, false)
	}
	if rt.Mock {
		return StreamMockAssistant(ctx, userText, sink, rt.Sess)
	}
	if prefixResult, handled, err := RunLocalPrefixToolIfAny(ctx, rt.Mock, orch, rt.Sess, userText, sink); handled {
		if err != nil {
			return "", err
		}
		appendLocalPrefixToolTrace(opts.ToolTrace, prefixResult)
		if prefixResult == nil {
			return "", nil
		}
		return prefixResult.Reply, nil
	}
	userText = ExpandInlineAtRefs(ctx, orch, userText)
	restoreProfile := maybeRouteBuildLiteTurnToPlan(rt, orch, userText)
	defer restoreProfile()
	if opts.ToolTrace != nil {
		return orch.RunStreamingToolTrace(ctx, userText, sink, opts.ToolTrace)
	}
	return orch.RunStreaming(ctx, userText, sink)
}

func appendLocalPrefixToolTrace(trace *[]orchestrator.JSONToolCall, result *LocalPrefixToolResult) {
	if trace == nil || result == nil {
		return
	}
	*trace = append(*trace, orchestrator.JSONToolCall{
		ID:      localPrefixToolUseID,
		Name:    result.ToolName,
		Input:   result.ToolInputJSON,
		Result:  result.Content,
		IsError: result.IsError,
	})
}

func maybeRouteBuildLiteTurnToPlan(rt *ChatRuntime, orch *orchestrator.Orchestrator, userText string) func() {
	if rt == nil || orch == nil {
		return func() {}
	}
	if !agents.IsBuildLiteProfileName(orch.ProfileName()) {
		return func() {}
	}
	res := orchestrator.ClassifyProfileIntentRules(userText)
	if res.Intent != orchestrator.ProfileIntentPlanOnly {
		return func() {}
	}
	plan, ok := rt.Profs[agents.Plan.Name]
	if !ok {
		return func() {}
	}
	prev := orch.ActiveProfile()
	orch.SetProfile(plan)
	rt.Profile = plan
	return func() {
		orch.SetProfile(prev)
		rt.Profile = prev
	}
}
