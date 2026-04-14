package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/tools"
)

// toolOutcome carries the result of one tool execution.
//
// Two distinct error cases:
//   - Non-fatal (IsError=true, Err=nil): the tool ran but returned an error.
//     Content holds the error message. The orchestrator surfaces it to the LLM
//     as a tool_result with is_error=true and continues the loop.
//   - Fatal (Err!=nil): a system-level failure (approval channel closed, context
//     cancelled, etc.). The orchestrator aborts the current Run and returns Err
//     to the caller. IsError and Content are ignored in this case.
type toolOutcome struct {
	Content string
	IsError bool  // non-fatal: tool executed but produced an error result
	Err     error // fatal: abort the orchestrator Run entirely
}

// rejectTool returns a non-fatal error outcome shown to the LLM as a tool_result with is_error=true.
func rejectTool(msg string) toolOutcome {
	return toolOutcome{Content: msg, IsError: true}
}

// sinkProgressAdapter bridges StreamSink.OnToolProgress into tools.ProgressReporter.
type sinkProgressAdapter struct {
	name string
	sink StreamSink
}

func (a *sinkProgressAdapter) OnProgress(_ string, partial string) {
	a.sink.OnToolProgress(a.name, partial)
}

func (o *Orchestrator) executeTool(ctx context.Context, tu *llm.ToolUse, sink StreamSink) toolOutcome {
	if sink != nil {
		ctx = ContextWithStreamSink(ctx, sink)
		ctx = tools.WithProgressReporter(ctx, &sinkProgressAdapter{name: tu.Name, sink: sink})
	}
	if outcome, blocked := o.checkReadOnly(tu.Name); blocked {
		return outcome
	}
	if outcome, blocked := o.checkApproval(ctx, tu); blocked {
		return outcome
	}
	if err := o.hooks.Fire(ctx, hooks.Event{
		Type:     hooks.PreToolUse,
		ToolName: tu.Name,
		Input:    tu.Input,
	}); err != nil {
		return rejectTool(fmt.Sprintf("pre_tool_use hook blocked: %v", err))
	}
	t, ok := o.tools.Get(tu.Name)
	if !ok {
		return rejectTool(fmt.Sprintf("unknown tool %q", tu.Name))
	}
	res, execErr := t.Execute(ctx, tu.Input)
	return o.finishToolExecution(ctx, tu.Name, tu.Input, res.Content, res.IsError, execErr)
}

// finishToolExecution runs post-tool hooks and maps execution result to a toolOutcome.
func (o *Orchestrator) finishToolExecution(ctx context.Context, toolName, input, content string, resultIsError bool, execErr error) toolOutcome {
	ev := hooks.Event{ToolName: toolName, Input: input, Output: content}
	if execErr != nil {
		ev.Type = hooks.PostToolUseFailure
		ev.FailureKind = hooks.FailureExecuteError
		if err := o.hooks.Fire(ctx, ev); err != nil {
			slog.WarnContext(ctx, "hook fire failed", "event", string(ev.Type), "tool", toolName, "err", err)
		}
		return rejectTool(execErr.Error())
	}
	if resultIsError {
		ev.Type = hooks.PostToolUseFailure
		ev.FailureKind = hooks.FailureErrorResult
	} else {
		ev.Type = hooks.PostToolUse
	}
	if err := o.hooks.Fire(ctx, ev); err != nil {
		slog.WarnContext(ctx, "hook fire failed", "event", string(ev.Type), "tool", toolName, "err", err)
	}
	return toolOutcome{Content: content, IsError: resultIsError}
}

// checkReadOnly returns a rejection outcome if the tool is blocked by the active read-only profile.
// Returns (zero, false) when the tool may proceed.
func (o *Orchestrator) checkReadOnly(toolName string) (toolOutcome, bool) {
	if !o.profile.ReadOnly {
		return toolOutcome{}, false
	}
	if strings.HasPrefix(toolName, "mcp__") {
		return rejectTool("mcp tools are disabled for read-only profiles"), true
	}
	switch toolName {
	case "bash", "write_file", "edit_file", "patch":
		return rejectTool(fmt.Sprintf("%s is blocked for read-only profile", toolName)), true
	}
	return toolOutcome{}, false
}

// checkApproval enforces permission policy, YOLO auto-approval, and interactive user approval.
// Returns (outcome, true) if the tool should be blocked; (zero, false) if it may proceed.
func (o *Orchestrator) checkApproval(ctx context.Context, tu *llm.ToolUse) (toolOutcome, bool) {
	switch o.perms.Evaluate(tu.Name) {
	case permissions.DecisionDeny:
		return rejectTool(fmt.Sprintf("permission denied for tool %q (policy mode deny)", tu.Name)), true
	case permissions.DecisionAsk:
		if o.cfg.YoloThreshold >= 0 {
			score := permissions.RiskScore(tu.Name, tu.Input)
			if score <= o.cfg.YoloThreshold {
				slog.Debug("yolo: auto-approved", "tool", tu.Name, "score", score, "threshold", o.cfg.YoloThreshold)
				return toolOutcome{}, false
			}
		}
		if o.approver == nil {
			return rejectTool(fmt.Sprintf("tool %q requires approval but no interactive approver is available (running non-interactively)", tu.Name)), true
		}
		approved, err := o.approver(ctx, tu.Name, tu.Input)
		if err != nil {
			return toolOutcome{Err: fmt.Errorf("tool approval: %w", err)}, true
		}
		if !approved {
			return rejectTool(fmt.Sprintf("user declined execution of tool %q", tu.Name)), true
		}
	}
	return toolOutcome{}, false
}
