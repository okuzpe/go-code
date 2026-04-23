package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/toolpolicy"
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

// rejectToolNextStep returns a non-fatal error outcome shown to the LLM as a tool_result with is_error=true.
// When nextStep is empty, only msg is returned (no "next step:" line).
func rejectToolNextStep(msg, nextStep string) toolOutcome {
	msg = strings.TrimSpace(msg)
	nextStep = strings.TrimSpace(nextStep)
	if nextStep == "" || strings.Contains(strings.ToLower(msg), "next step:") {
		return toolOutcome{Content: msg, IsError: true}
	}
	return toolOutcome{Content: fmt.Sprintf("%s\nnext step: %s", msg, nextStep), IsError: true}
}

func blockedToolNextStep(toolName string) string {
	switch toolName {
	case "read_file", "glob", "grep", "web_fetch", "web_search":
		return "switch to an agent profile with this tool enabled, or use an allowed read-only tool"
	case "bash", "run_command", "script":
		return "switch mode for command execution, or ask the user to run the command manually"
	case "write_file", "write_files", "edit_file", "patch", "create_project", "run_tests", "git_tool":
		return "switch to a profile with write permissions, then retry"
	default:
		return "review active tool permissions and retry with an allowed tool"
	}
}

func unknownToolNextStep(toolName string) string {
	if strings.HasPrefix(toolName, "mcp__") {
		return "run /doctor to verify MCP connectivity and available MCP tools"
	}
	return "choose one of the available tools from this session and retry"
}

func executionErrorNextStep(toolName, message string) string {
	lower := strings.ToLower(message)
	switch toolName {
	case "read_file":
		if strings.Contains(lower, "not found") || strings.Contains(lower, "no such file") {
			return "verify the path, run glob first if needed, then retry read_file"
		}
		return "verify the file path and permissions, then retry read_file"
	case "edit_file", "patch", "write_file", "write_files":
		if strings.Contains(lower, "not found") || strings.Contains(lower, "no such file") {
			return "read_file the target first to confirm path and current content, then retry"
		}
		return "re-read the target content, adjust the edit scope, then retry"
	case "bash", "run_command", "script", "run_tests":
		return "fix the command or run a narrower command to isolate the failure"
	default:
		return "inspect tool input and retry with narrower, validated arguments"
	}
}

// sinkProgressAdapter bridges StreamSink.OnToolProgress into tools.ProgressReporter.
type sinkProgressAdapter struct {
	name string
	sink StreamSink
}

func (a *sinkProgressAdapter) OnProgress(_ string, partial string) {
	a.sink.OnToolProgress(a.name, partial)
}

// isTransientToolError reports whether a non-fatal tool error message looks like a
// temporary filesystem condition that a single retry may resolve.
func isTransientToolError(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "access is denied") ||
		strings.Contains(lower, "file is locked") ||
		strings.Contains(lower, "create temp file") ||
		strings.Contains(lower, "rename to target") ||
		strings.Contains(lower, "resource temporarily unavailable")
}

func (o *Orchestrator) executeTool(ctx context.Context, tu *llm.ToolUse, sink StreamSink) toolOutcome {
	out := o.executeToolOnce(ctx, tu, sink)
	if out.IsError && out.Err == nil && isTransientToolError(out.Content) {
		slog.Debug("transient tool error, retrying once", "tool", tu.Name, "err", out.Content)
		time.Sleep(150 * time.Millisecond)
		out = o.executeToolOnce(ctx, tu, sink)
	}
	return out
}

func (o *Orchestrator) executeToolOnce(ctx context.Context, tu *llm.ToolUse, sink StreamSink) toolOutcome {
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

	// Serve from per-turn cache for idempotent read-only tools (glob, grep, web_search).
	if toolpolicy.CacheableWithinTurn(tu.Name) && o.ut != nil {
		cacheKey := tu.Name + ":" + tu.Input
		if cached, hit := o.ut.toolCacheGet(cacheKey); hit {
			slog.Debug("tool cache hit", "tool", tu.Name)
			return toolOutcome{Content: cached}
		}
	}

	if err := o.hooks.Fire(ctx, hooks.Event{
		Type:     hooks.PreToolUse,
		ToolName: tu.Name,
		Input:    tu.Input,
	}); err != nil {
		return rejectToolNextStep(fmt.Sprintf("pre_tool_use hook blocked: %v", err), "fix or disable the blocking hook, then retry")
	}
	t, ok := o.tools.Get(tu.Name)
	if !ok {
		return rejectToolNextStep(fmt.Sprintf("unknown tool %q", tu.Name), unknownToolNextStep(tu.Name))
	}
	pendingCheckpoint, checkpointErr := o.capturePendingCheckpoint(tu.Name, tu.Input)
	if checkpointErr != nil {
		return rejectToolNextStep(fmt.Sprintf("checkpoint before %s failed: %v", tu.Name, checkpointErr), "retry with a narrower file change or inspect the session scratch directory")
	}
	res, execErr := t.Execute(ctx, tu.Input)
	if execErr != nil || res.IsError {
		o.discardCheckpoint(pendingCheckpoint)
	} else {
		o.commitCheckpoint(pendingCheckpoint)
	}
	outcome := o.finishToolExecution(ctx, tu.Name, tu.Input, res.Content, res.IsError, execErr)

	// Populate cache on success for cachable tools.
	if toolpolicy.CacheableWithinTurn(tu.Name) && !outcome.IsError && outcome.Err == nil && o.ut != nil {
		cacheKey := tu.Name + ":" + tu.Input
		o.ut.toolCacheSet(cacheKey, outcome.Content)
	}

	// Reveal hidden tools discovered via tool_search so they appear in the next LLM iteration.
	if tu.Name == "tool_search" && !outcome.IsError && outcome.Err == nil {
		o.revealToolSearchMatches(outcome.Content)
	}

	return outcome
}

// revealToolSearchMatches parses a tool_search result and marks matched hidden tools as
// visible for the remainder of this turn. The result format is:
//
//	tool_search matches (N):
//	- tool_name: description
func (o *Orchestrator) revealToolSearchMatches(content string) {
	if o.ut == nil {
		return
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		line = line[2:]
		name := line
		if idx := strings.Index(line, ":"); idx > 0 {
			name = strings.TrimSpace(line[:idx])
		}
		if name == "" || !isValidToolName(name) || !o.tools.IsHidden(name) {
			continue
		}
		o.ut.revealedToolNames[name] = true
	}
}

// isValidToolName reports whether s is a syntactically valid tool name:
// only lowercase letters, digits, underscores, and hyphens — no spaces or control chars.
// MCP tool names follow the pattern mcp__serverid__toolname where serverid may include hyphens.
func isValidToolName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// turnToolCacheMaxEntries caps per-turn cache size. Prevents unbounded memory growth
// during long explore turns with many unique glob/grep patterns.
const turnToolCacheMaxEntries = 128

// finishToolExecution runs post-tool hooks and maps execution result to a toolOutcome.
func (o *Orchestrator) finishToolExecution(ctx context.Context, toolName, input, content string, resultIsError bool, execErr error) toolOutcome {
	ev := hooks.Event{ToolName: toolName, Input: input, Output: content}
	if execErr != nil {
		ev.Type = hooks.PostToolUseFailure
		ev.FailureKind = hooks.FailureExecuteError
		if err := o.hooks.Fire(ctx, ev); err != nil {
			slog.WarnContext(ctx, "hook fire failed", "event", string(ev.Type), "tool", toolName, "err", err)
		}
		return rejectToolNextStep(execErr.Error(), executionErrorNextStep(toolName, execErr.Error()))
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
		return rejectToolNextStep("mcp tools are disabled for read-only profiles", "switch to a non read-only profile to use MCP tools"), true
	}
	switch toolName {
	case "bash", "run_command", "script", "write_file", "write_files", "create_project", "edit_file", "patch", "run_tests", "git_tool":
		return rejectToolNextStep(fmt.Sprintf("%s is blocked for read-only profile", toolName), blockedToolNextStep(toolName)), true
	}
	return toolOutcome{}, false
}

// checkApproval enforces permission policy, YOLO auto-approval, and interactive user approval.
// Returns (outcome, true) if the tool should be blocked; (zero, false) if it may proceed.
func (o *Orchestrator) checkApproval(ctx context.Context, tu *llm.ToolUse) (toolOutcome, bool) {
	switch o.perms.Evaluate(tu.Name) {
	case permissions.DecisionDeny:
		return rejectToolNextStep(fmt.Sprintf("permission denied for tool %q (policy mode deny)", tu.Name), blockedToolNextStep(tu.Name)), true
	case permissions.DecisionAsk:
		if o.writeTargetsUnderScratch(tu.Name, tu.Input) {
			return toolOutcome{}, false
		}
		if o.cfg.YoloThreshold >= 0 {
			score := permissions.RiskScore(tu.Name, tu.Input)
			if score <= o.cfg.YoloThreshold {
				slog.Debug("yolo: auto-approved", "tool", tu.Name, "score", score, "threshold", o.cfg.YoloThreshold)
				return toolOutcome{}, false
			}
		}
		if o.approver == nil {
			return rejectToolNextStep(fmt.Sprintf("tool %q requires approval but no interactive approver is available (running non-interactively)", tu.Name), "run interactively to approve, or set policy mode allow for this tool"), true
		}
		approved, err := o.approver(ctx, tu.Name, tu.Input)
		if err != nil {
			return toolOutcome{Err: fmt.Errorf("tool approval: %w", err)}, true
		}
		if !approved {
			return rejectToolNextStep(fmt.Sprintf("user declined execution of tool %q", tu.Name), "ask for approval again with safer/narrower input"), true
		}
	}
	return toolOutcome{}, false
}
