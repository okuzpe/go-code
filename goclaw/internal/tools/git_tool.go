package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GitTool runs a limited, safe set of git sub-commands within the workspace.
// It intentionally excludes destructive or network operations (push, pull, reset --hard, clean -f).
type GitTool struct {
	scope      PathScope
	timeoutSec int
}

// NewGitTool returns git_tool scoped to root.
func NewGitTool(root string) *GitTool {
	return NewGitToolScope(PathScope{Root: root, RelativeBase: root})
}

// NewGitToolScope returns git_tool with explicit PathScope.
func NewGitToolScope(scope PathScope) *GitTool {
	s := NormalizePathScope(scope)
	if strings.TrimSpace(s.RelativeBase) == "" {
		s.RelativeBase = s.Root
	}
	return &GitTool{scope: s, timeoutSec: 30}
}

var _ Tool = (*GitTool)(nil)

func (GitTool) Name() string { return "git_tool" }

func (GitTool) Description() string {
	return "Run a safe subset of git commands within the workspace. " +
		"Supported sub-commands: status, diff, log, show, add, commit, branch, stash, blame, shortlog. " +
		"Destructive operations (push, pull, reset, clean, checkout, rebase, merge) are blocked — use bash for those when needed. " +
		"The 'args' list is passed directly to git after the sub-command; each element is a separate argument (no shell expansion)."
}

func (GitTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subcommand": map[string]any{
				"type": "string",
				"enum": []string{
					"status", "diff", "log", "show", "add", "commit",
					"branch", "stash", "blame", "shortlog",
				},
				"description": "The git sub-command to run.",
			},
			"args": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Additional arguments passed to git after the sub-command (each as a separate string, no shell expansion).",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Working directory for the git command (defaults to workspace root).",
			},
		},
		"required": []string{"subcommand"},
	}
}

// allowedGitSubcommands is the explicit allowlist.
var allowedGitSubcommands = map[string]struct{}{
	"status":   {},
	"diff":     {},
	"log":      {},
	"show":     {},
	"add":      {},
	"commit":   {},
	"branch":   {},
	"stash":    {},
	"blame":    {},
	"shortlog": {},
}

// blockedGitArgs contains flags that could make allowed sub-commands destructive.
var blockedGitArgs = map[string]struct{}{
	"--force":  {},
	"-f":       {},
	"--hard":   {},
	"--mixed":  {},
	"--soft":   {},
	"--delete": {},
	"-D":       {},
	"--mirror": {},
	"--all":    {}, // blocked only for stash drop/clear patterns
}

type gitToolInput struct {
	Subcommand string   `json:"subcommand"`
	Args       []string `json:"args"`
	Path       string   `json:"path"`
}

// Execute implements Tool.
func (t *GitTool) Execute(ctx context.Context, input string) (Result, error) {
	var in gitToolInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid json input: %v", err), IsError: true}, nil
	}

	sub := strings.TrimSpace(strings.ToLower(in.Subcommand))
	if _, allowed := allowedGitSubcommands[sub]; !allowed {
		return Result{
			Content: fmt.Sprintf("git_tool: sub-command %q is not in the allowed list (status, diff, log, show, add, commit, branch, stash, blame, shortlog)", in.Subcommand),
			IsError: true,
		}, nil
	}

	// Filter dangerous flags from args.
	safeArgs := make([]string, 0, len(in.Args))
	for _, arg := range in.Args {
		clean := strings.TrimSpace(arg)
		if clean == "" {
			continue
		}
		// Block known destructive flags.
		if _, blocked := blockedGitArgs[clean]; blocked {
			// Allow --all for log and shortlog (not destructive there).
			if clean == "--all" && (sub == "log" || sub == "shortlog") {
				safeArgs = append(safeArgs, clean)
				continue
			}
			return Result{
				Content: fmt.Sprintf("git_tool: argument %q is not permitted (potentially destructive flag)", clean),
				IsError: true,
			}, nil
		}
		// Block subshell injection characters.
		if strings.ContainsAny(clean, ";&|`$(){}") {
			return Result{
				Content: fmt.Sprintf("git_tool: argument %q contains disallowed characters", clean),
				IsError: true,
			}, nil
		}
		safeArgs = append(safeArgs, clean)
	}

	// Resolve working directory.
	runDir := t.scope.RelBase()
	if p := strings.TrimSpace(in.Path); p != "" {
		resolved, err := ResolveReadExistingPath(t.scope, p)
		if err != nil {
			return Result{Content: fmt.Sprintf("path error: %v", err), IsError: true}, nil
		}
		runDir = resolved
	}

	cmdArgs := append([]string{sub}, safeArgs...)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(t.timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "git", cmdArgs...)
	cmd.Dir = runDir
	out, err := cmd.CombinedOutput()

	output := strings.TrimRight(string(out), "\n")
	if runCtx.Err() == context.DeadlineExceeded {
		return Result{Content: "git_tool: timed out", IsError: true}, nil
	}
	if err != nil {
		if output == "" {
			output = err.Error()
		}
		return Result{Content: output, IsError: true}, nil
	}
	if output == "" {
		output = "(no output)"
	}
	return Result{Content: output}, nil
}
