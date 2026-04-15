package orchestrator

import "strings"

// PhaseContext carries per-iteration state flags so ThinkingPhaseLine can
// emit a phase label that reflects the current step in the agent loop cycle.
type PhaseContext struct {
	// HadToolRound is true when at least one tool batch has already run this turn.
	HadToolRound bool
	// WorkspaceWriteOK is true when a write_file/edit_file/patch succeeded this turn.
	WorkspaceWriteOK bool
	// LastBatchReadOnly is true when the most recent tool batch contained only read-only tools.
	LastBatchReadOnly bool
}

// ThinkingPhaseLine is a short English status for the LLM streaming phase before tool calls or text deltas.
// iterZeroBased is the orchestrator loop index (0 = first LLM call in this user turn).
// taskRole is the resolved task_models role (e.g. code, explore, fix); may be empty when routing is off.
// ctx carries per-iteration state to derive a phase label that reflects the current agent loop step.
func ThinkingPhaseLine(iterZeroBased int, taskRole string, ctx PhaseContext) string {
	if iterZeroBased == 0 {
		switch strings.ToLower(strings.TrimSpace(taskRole)) {
		case "code", "fix":
			return "Analyzing repository"
		case "explore":
			return "Exploring"
		case "fast":
			// Short or conversational turns: not a repo scan; keep the label neutral.
			return "Thinking"
		case "reasoning", "creative":
			return "Deep reasoning"
		default:
			return "Planning"
		}
	}
	if ctx.WorkspaceWriteOK {
		return "Verifying changes"
	}
	if ctx.HadToolRound && !ctx.LastBatchReadOnly {
		return "Applying changes"
	}
	if ctx.HadToolRound {
		return "Analyzing findings"
	}
	return "Continuing"
}

// ToolPhaseHeadline is a very short English category for agent phase UX (CLI/TUI), keyed by tool name.
func ToolPhaseHeadline(toolName string) string {
	switch toolName {
	case "glob", "grep":
		return "Repository scan"
	case "read_file":
		return "Reading files"
	case "write_file", "edit_file", "patch":
		return "Workspace edit"
	case "bash", "script":
		return "Shell"
	case "web_fetch", "web_search":
		return "Network"
	case "spawn_agent", "stop_task":
		return "Agents"
	case "todo_write":
		return "Tasks"
	default:
		if strings.HasPrefix(toolName, "mcp__") {
			return "MCP"
		}
		return "Tool"
	}
}
