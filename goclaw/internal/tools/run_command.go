package tools

import "context"

// RunCommandTool runs the same allowlisted single-command execution as BashTool under a
// Claude Code–style name so prompts and docs can say run_command without a second implementation.
type RunCommandTool struct {
	bash *BashTool
}

// NewRunCommandWithTimeout returns a run_command tool sharing behavior with NewBashWithTimeout.
func NewRunCommandWithTimeout(timeoutSec int) *RunCommandTool {
	return &RunCommandTool{bash: NewBashWithTimeout(timeoutSec)}
}

var _ Tool = (*RunCommandTool)(nil)

func (*RunCommandTool) Name() string { return "run_command" }

func (*RunCommandTool) Description() string {
	return "Run one allowlisted shell command — same engine and limits as bash (single command, no pipes; use script for pipes, &&, or redirection). " +
		"Prefer run_command or bash interchangeably."
}

func (t *RunCommandTool) InputSchema() any {
	return t.bash.InputSchema()
}

func (t *RunCommandTool) Execute(ctx context.Context, input string) (Result, error) {
	return t.bash.Execute(ctx, input)
}
