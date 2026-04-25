package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BashTool runs a broad but allowlisted set of shell commands with a timeout.
type BashTool struct {
	// timeoutSec is execution cap in seconds; zero means BashTimeoutSec from limits.go.
	timeoutSec int
}

// NewBash returns the bash tool with the default timeout (BashTimeoutSec).
func NewBash() *BashTool { return &BashTool{} }

// NewBashWithTimeout returns a bash tool with the given timeout in seconds.
// Non-positive values fall back to BashTimeoutSec; values above maxBashTimeoutSec are clamped.
func NewBashWithTimeout(timeoutSec int) *BashTool {
	return &BashTool{timeoutSec: normalizeBashTimeoutSec(timeoutSec)}
}

func normalizeBashTimeoutSec(sec int) int {
	const maxBashTimeoutSec = 3600
	if sec <= 0 {
		return 0
	}
	if sec > maxBashTimeoutSec {
		return maxBashTimeoutSec
	}
	return sec
}

func (b BashTool) bashTimeout() time.Duration {
	sec := b.timeoutSec
	if sec <= 0 {
		sec = BashTimeoutSec
	}
	return time.Duration(sec) * time.Second
}

var _ Tool = (*BashTool)(nil)

func (BashTool) Name() string { return "bash" }

func (BashTool) Description() string {
	return "Run one allowlisted shell command (non-interactive, output capped at 256 KiB, 30 s timeout). " +
		"On Windows, goclaw first looks for a working POSIX shell (Git Bash or MSYS sh). " +
		"Without one, it falls back to cmd.exe with Windows quoting semantics and a reduced command surface. " +
		"Allowed categories: filesystem (ls, find, stat, mkdir, cp, mv, rm, touch, chmod, diff, tar, zip...), " +
		"text (cat, head, tail, grep, rg, sed, awk, cut, sort, uniq, jq...), " +
		"build (go, make, cmake, cargo, npm, yarn, pnpm, node, npx, bun, python3, pip, uv, java, mvn, gradle, ruby, gem...), " +
		"network (curl, wget), git (status, log, diff, add, commit, push, pull, fetch, checkout, merge, rebase, stash, clone, remote...), " +
		"and utilities (echo, sleep, pwd, date, env, which, uname, gh). " +
		"Prefer read_file / glob / grep tools for pure read operations. " +
		"For multi-step commands requiring pipes (|), &&, or redirections, use the script tool if available."
}

func (BashTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type": "string",
				"description": "Single simple command: one allowlisted binary plus arguments. " +
					"No shell pipes (|), command separators (;, &&), redirections (>, <), subshells, or command substitution - " +
					"quote URLs that contain &. On Windows cmd.exe fallback, single quotes do not protect shell metacharacters.",
			},
			"cwd": map[string]any{
				"type":        "string",
				"description": "Working directory (optional, defaults to process cwd)",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "One short sentence (<= 80 chars) describing what this command does. Used for display only - not passed to the shell.",
			},
		},
		"required": []string{"command"},
	}
}

type bashInput struct {
	Command string `json:"command"`
	Cwd     string `json:"cwd"`
}

// Execute implements Tool.
func (b BashTool) Execute(ctx context.Context, input string) (Result, error) {
	var in bashInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: "", IsError: true}, fmt.Errorf("invalid json input: %w", err)
	}
	cmdLine := strings.TrimSpace(in.Command)
	if cmdLine == "" {
		return Result{Content: "command is required", IsError: true}, nil
	}
	shell := activeShellSpec()
	if err := validateAllowlistedCommand(cmdLine, shell); err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}
	if err := rejectShellMetacharacters(cmdLine, shell.Kind); err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}
	if err := rejectDangerousArgs(cmdLine); err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}
	if err := rejectSSRFInNetworkArgs(cmdLine); err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}

	cwd := strings.TrimSpace(in.Cwd)
	if cwd != "" {
		var err error
		cwd, err = filepath.Abs(cwd)
		if err != nil {
			return Result{Content: fmt.Sprintf("cwd: %v", err), IsError: true}, nil
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, b.bashTimeout())
	defer cancel()

	execName, args := shell.invocation(cmdLine)
	c := newExecCommandContext(timeoutCtx, execName, args...)
	if cwd != "" {
		c.Dir = cwd
	}
	c.Env = os.Environ()

	reporter := ProgressReporterFromContext(ctx)
	if reporter == nil {
		// No progress sink - use simple CombinedOutput for non-interactive callers.
		out, err := c.CombinedOutput()
		if len(out) > MaxBashOutput {
			out = out[:MaxBashOutput]
			out = append(out, []byte("\n[output truncated]")...)
		}
		s := string(out)
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return Result{Content: s + "\n[timeout]", IsError: true}, nil
		}
		if err != nil {
			if s == "" {
				s = err.Error()
			} else {
				s += "\n" + err.Error()
			}
			return Result{Content: s, IsError: true}, nil
		}
		return Result{Content: s, IsError: false}, nil
	}

	out, waitErr := runCommandWithProgressPipes(c, reporter)
	if waitErr != nil && len(out) == 0 {
		return Result{Content: waitErr.Error(), IsError: true}, nil
	}
	s := string(out)
	if timeoutCtx.Err() == context.DeadlineExceeded {
		return Result{Content: s + "\n[timeout]", IsError: true}, nil
	}
	if waitErr != nil {
		if s == "" {
			s = waitErr.Error()
		} else {
			s += "\n" + waitErr.Error()
		}
		return Result{Content: s, IsError: true}, nil
	}
	return Result{Content: s, IsError: false}, nil
}
