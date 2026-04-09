package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/permissions"
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

func (o *Orchestrator) executeTool(ctx context.Context, tu *llm.ToolUse) toolOutcome {
	if o.profile.ReadOnly {
		if strings.HasPrefix(tu.Name, "mcp__") {
			return toolOutcome{
				Content: "mcp tools are disabled for read-only profiles",
				IsError: true,
			}
		}
		switch tu.Name {
		case "bash", "write_file", "edit_file":
			return toolOutcome{
				Content: fmt.Sprintf("%s is blocked for read-only profile", tu.Name),
				IsError: true,
			}
		}
	}

	switch o.perms.Evaluate(tu.Name) {
	case permissions.DecisionDeny:
		return toolOutcome{
			Content: fmt.Sprintf("permission denied for tool %q (policy mode deny)", tu.Name),
			IsError: true,
		}
	case permissions.DecisionAsk:
		// YOLO: auto-approve if the tool's risk score is within the configured threshold.
		if o.cfg.YoloThreshold >= 0 {
			score := permissions.RiskScore(tu.Name, tu.Input)
			if score <= o.cfg.YoloThreshold {
				slog.Debug("yolo: auto-approved", "tool", tu.Name, "score", score, "threshold", o.cfg.YoloThreshold)
				break
			}
		}
		if o.approver == nil {
			return toolOutcome{Err: fmt.Errorf("tool %q requires user approval; no approver configured", tu.Name)}
		}
		ok, err := o.approver(ctx, tu.Name, tu.Input)
		if err != nil {
			return toolOutcome{Err: fmt.Errorf("tool approval: %w", err)}
		}
		if !ok {
			return toolOutcome{
				Content: fmt.Sprintf("user declined execution of tool %q", tu.Name),
				IsError: true,
			}
		}
	}

	if err := o.hooks.Fire(ctx, hooks.Event{
		Type:     hooks.PreToolUse,
		ToolName: tu.Name,
		Input:    tu.Input,
	}); err != nil {
		return toolOutcome{Content: fmt.Sprintf("pre_tool_use hook blocked: %v", err), IsError: true}
	}

	t, ok := o.tools.Get(tu.Name)
	if !ok {
		return toolOutcome{Content: fmt.Sprintf("unknown tool %q", tu.Name), IsError: true}
	}

	res, err := t.Execute(ctx, tu.Input)

	hookEvent := hooks.Event{ToolName: tu.Name, Input: tu.Input, Output: res.Content}
	if err != nil {
		hookEvent.Type = hooks.PostToolUseFailure
		_ = o.hooks.Fire(ctx, hookEvent)
		return toolOutcome{Content: err.Error(), IsError: true}
	}
	if res.IsError {
		hookEvent.Type = hooks.PostToolUseFailure
	} else {
		hookEvent.Type = hooks.PostToolUse
	}
	_ = o.hooks.Fire(ctx, hookEvent)

	return toolOutcome{Content: res.Content, IsError: res.IsError}
}
