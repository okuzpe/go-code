package orchestrator

import (
	"context"
	"fmt"

	"github.com/okuzpe/goclaw/internal/llm"
)

// RunToolInvocation executes a single tool the same way the agent loop does:
// read-only profile checks, permission policy, optional approver, PreToolUse /
// PostToolUse hooks, registry dispatch, and StreamSink notifications.
// toolInputJSON is the raw JSON arguments string passed to Tool.Execute.
// It returns fatal errors (e.g. approval failure) separately from tool-level
// failures (returned as content with isError=true).
func (o *Orchestrator) RunToolInvocation(ctx context.Context, toolName, toolInputJSON string, sink StreamSink) (content string, isError bool, err error) {
	if toolName == "" {
		return "", false, fmt.Errorf("run tool invocation: empty tool name")
	}
	tu := &llm.ToolUse{
		ID:    "prefix-invoke",
		Name:  toolName,
		Input: toolInputJSON,
	}
	if sink != nil {
		preview := FormatToolUsePreview(toolName, toolInputJSON)
		sink.OnToolUse(toolName, preview)
	}
	out := o.executeTool(ctx, tu, sink)
	if out.Err != nil {
		return "", false, out.Err
	}
	if o.afterTool != nil {
		o.afterTool(toolName, toolInputJSON, len(out.Content), out.IsError)
	}
	if sink != nil {
		sink.OnToolResult(toolName, len(out.Content), out.IsError)
	}
	return out.Content, out.IsError, nil
}
