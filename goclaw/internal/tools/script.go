package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ScriptTool runs a multi-line shell script, allowing pipes, &&, redirections,
// and other shell composition that the bash tool intentionally forbids.
// It is opt-in via AllowScript in Config (allow_script: true in settings.json).
type ScriptTool struct {
	timeoutSec int
}

// NewScript returns a ScriptTool with the default bash timeout.
func NewScript() *ScriptTool { return &ScriptTool{} }

// NewScriptWithTimeout returns a ScriptTool with the given timeout in seconds.
// Non-positive values fall back to BashTimeoutSec; values above 3600 are clamped.
func NewScriptWithTimeout(timeoutSec int) *ScriptTool {
	return &ScriptTool{timeoutSec: normalizeBashTimeoutSec(timeoutSec)}
}

var _ Tool = (*ScriptTool)(nil)

func (ScriptTool) Name() string { return "script" }

func (ScriptTool) Description() string {
	return "Execute a multi-line shell script. " +
		"Use this when a task requires pipes (|), command chaining (&&, ||), " +
		"redirections (>, >>), or other shell composition — " +
		"for example: `go build ./... && go test ./...` or `find . -name '*.go' | xargs wc -l`. " +
		"The script is written to a temp file and executed by bash (or cmd.exe on Windows). " +
		"Output is capped at 256 KiB. " +
		"Prefer read_file, glob, grep, and edit_file for file operations — use script only when shell composition is genuinely needed."
}

func (ScriptTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"script": map[string]any{
				"type":        "string",
				"description": "The shell script to execute. May contain multiple lines, pipes, &&, redirections, etc.",
			},
			"cwd": map[string]any{
				"type":        "string",
				"description": "Working directory for the script (optional, defaults to process cwd).",
			},
		},
		"required": []string{"script"},
	}
}

type scriptInput struct {
	Script string `json:"script"`
	Cwd    string `json:"cwd"`
}

func (s ScriptTool) scriptTimeout() int {
	if s.timeoutSec > 0 {
		return s.timeoutSec
	}
	return BashTimeoutSec
}

// Execute implements Tool.
func (s ScriptTool) Execute(ctx context.Context, input string) (Result, error) {
	var in scriptInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: "", IsError: true}, fmt.Errorf("invalid json input: %w", err)
	}
	scriptContent := strings.TrimSpace(in.Script)
	if scriptContent == "" {
		return Result{Content: "script is required", IsError: true}, nil
	}

	// Write script to a temp file.
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".bat"
	}
	tmpFile, err := os.CreateTemp("", "goclaw-script-*"+ext)
	if err != nil {
		return Result{Content: fmt.Sprintf("failed to create temp script: %v", err), IsError: true}, nil
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(scriptContent + "\n"); err != nil {
		tmpFile.Close()
		return Result{Content: fmt.Sprintf("failed to write temp script: %v", err), IsError: true}, nil
	}
	if err := tmpFile.Close(); err != nil {
		return Result{Content: fmt.Sprintf("failed to close temp script: %v", err), IsError: true}, nil
	}

	// Make executable on Unix.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o700); err != nil {
			return Result{Content: fmt.Sprintf("failed to chmod temp script: %v", err), IsError: true}, nil
		}
	}

	cwd := strings.TrimSpace(in.Cwd)

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(s.scriptTimeout())*time.Second)
	defer cancel()

	name, args := scriptInvocation(tmpPath)
	cmd := exec.CommandContext(timeoutCtx, name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = os.Environ()

	out, execErr := cmd.CombinedOutput()
	if len(out) > MaxBashOutput {
		out = out[:MaxBashOutput]
		out = append(out, []byte("\n[output truncated]")...)
	}
	output := string(out)

	if timeoutCtx.Err() == context.DeadlineExceeded {
		return Result{Content: output + "\n[timeout]", IsError: true}, nil
	}
	if execErr != nil {
		if output == "" {
			output = execErr.Error()
		} else {
			output += "\n" + execErr.Error()
		}
		return Result{Content: output, IsError: true}, nil
	}
	return Result{Content: output, IsError: false}, nil
}

// scriptInvocation returns the interpreter and arguments to run a script file.
func scriptInvocation(scriptPath string) (string, []string) {
	if runtime.GOOS == "windows" {
		// .sh files: prefer bash if available.
		if strings.HasSuffix(scriptPath, ".sh") {
			if p, err := exec.LookPath("bash"); err == nil {
				return p, []string{scriptPath}
			}
			if p, err := exec.LookPath("sh"); err == nil {
				return p, []string{scriptPath}
			}
		}
		// .bat files: use cmd.exe.
		return "cmd", []string{"/C", scriptPath}
	}
	return "/bin/bash", []string{scriptPath}
}
