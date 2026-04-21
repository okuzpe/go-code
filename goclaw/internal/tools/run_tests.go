package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunTestsTool detects the project type under a given directory and runs its test suite,
// returning a structured PASS/FAIL summary with failure details.
type RunTestsTool struct {
	scope      PathScope
	timeoutSec int
}

// NewRunTests returns run_tests scoped to root with the default timeout (120 s).
func NewRunTests(root string) *RunTestsTool {
	return NewRunTestsScope(PathScope{Root: root, RelativeBase: root}, 0)
}

// NewRunTestsScope returns run_tests with explicit PathScope and timeout.
// timeoutSec 0 uses the built-in default (120 s); values above 3600 are clamped.
func NewRunTestsScope(scope PathScope, timeoutSec int) *RunTestsTool {
	s := NormalizePathScope(scope)
	if strings.TrimSpace(s.RelativeBase) == "" {
		s.RelativeBase = s.Root
	}
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	if timeoutSec > 3600 {
		timeoutSec = 3600
	}
	return &RunTestsTool{scope: s, timeoutSec: timeoutSec}
}

var _ Tool = (*RunTestsTool)(nil)

func (RunTestsTool) Name() string { return "run_tests" }

func (RunTestsTool) Description() string {
	return "Run the project test suite and return a PASS/FAIL summary with failure details. " +
		"Auto-detects project type from go.mod (go test ./...), package.json (npm test), " +
		"or pyproject.toml/requirements.txt (python -m pytest). " +
		"Use the 'path' field to target a subdirectory or specific package (e.g. './internal/tools/...'). " +
		"Set verbose:true to include all test output, not just failures."
}

func (RunTestsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to run tests in (defaults to workspace root). For Go, also accepts a package path like './internal/tools/...'.",
			},
			"verbose": map[string]any{
				"type":        "boolean",
				"description": "Include all test output, not just failures (default false).",
			},
		},
	}
}

type runTestsInput struct {
	Path    string `json:"path"`
	Verbose bool   `json:"verbose"`
}

// projectKind classifies the project type by presence of marker files.
type projectKind int

const (
	projectUnknown projectKind = iota
	projectGo
	projectNode
	projectPython
)

func detectProjectKind(dir string) projectKind {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return projectGo
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return projectNode
	}
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		return projectPython
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		return projectPython
	}
	return projectUnknown
}

// Execute implements Tool.
func (t *RunTestsTool) Execute(ctx context.Context, input string) (Result, error) {
	var in runTestsInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid json input: %v", err), IsError: true}, nil
	}

	// Resolve the test directory.
	testDir := strings.TrimSpace(in.Path)
	var runDir string
	if testDir == "" {
		runDir = t.scope.RelBase()
	} else {
		resolved, err := ResolveReadExistingPath(t.scope, testDir)
		if err != nil {
			return Result{Content: fmt.Sprintf("path error: %v", err), IsError: true}, nil
		}
		runDir = resolved
	}

	kind := detectProjectKind(runDir)

	var cmdName string
	var cmdArgs []string

	switch kind {
	case projectGo:
		cmdName = "go"
		pkgArg := "./..."
		if testDir != "" && !strings.HasPrefix(testDir, ".") && !strings.HasPrefix(testDir, "/") {
			// User passed a relative package path like "internal/tools/..."
			pkgArg = "./" + strings.TrimPrefix(testDir, "./")
			runDir = t.scope.RelBase()
		}
		cmdArgs = []string{"test"}
		if in.Verbose {
			cmdArgs = append(cmdArgs, "-v")
		}
		cmdArgs = append(cmdArgs, pkgArg)
	case projectNode:
		cmdName = "npm"
		cmdArgs = []string{"test", "--", "--no-coverage"}
	case projectPython:
		cmdName = "python"
		cmdArgs = []string{"-m", "pytest"}
		if !in.Verbose {
			cmdArgs = append(cmdArgs, "-q")
		}
	default:
		return Result{
			Content: "run_tests: could not detect project type (no go.mod, package.json, or pyproject.toml found in " + runDir + ")",
			IsError: true,
		}, nil
	}

	timeout := time.Duration(t.timeoutSec) * time.Second
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, cmdName, cmdArgs...)
	cmd.Dir = runDir
	out, err := cmd.CombinedOutput()

	output := string(out)
	if runCtx.Err() == context.DeadlineExceeded {
		return Result{
			Content: fmt.Sprintf("run_tests: timed out after %ds\n\nPartial output:\n%s", t.timeoutSec, truncateOutput(output, 4096)),
			IsError: true,
		}, nil
	}

	summary := buildTestSummary(kind, output, err == nil)
	return Result{Content: summary}, nil
}

// buildTestSummary formats a PASS/FAIL summary with relevant lines extracted from raw output.
func buildTestSummary(kind projectKind, output string, passed bool) string {
	var sb strings.Builder
	if passed {
		sb.WriteString("PASS\n\n")
	} else {
		sb.WriteString("FAIL\n\n")
	}

	lines := strings.Split(output, "\n")

	switch kind {
	case projectGo:
		var failLines []string
		var pkgPass, pkgFail, testPass, testFail int
		for _, line := range lines {
			switch {
			case strings.HasPrefix(line, "ok "):
				pkgPass++
			case strings.HasPrefix(line, "FAIL\t") || strings.HasPrefix(line, "FAIL "):
				pkgFail++
				failLines = append(failLines, line)
			case strings.HasPrefix(line, "--- PASS:"):
				testPass++
			case strings.HasPrefix(line, "--- FAIL:"):
				testFail++
				failLines = append(failLines, line)
			}
		}
		if testPass+testFail > 0 {
			sb.WriteString(fmt.Sprintf("Tests: %d passed, %d failed\n", testPass, testFail))
		}
		sb.WriteString(fmt.Sprintf("Packages: %d passed, %d failed\n", pkgPass, pkgFail))
		if len(failLines) > 0 {
			sb.WriteString("\nFailed:\n")
			for _, l := range failLines {
				sb.WriteString("  ")
				sb.WriteString(l)
				sb.WriteByte('\n')
			}
		}
	case projectNode:
		// Jest/Mocha output: look for summary lines like "X passed", "X failed", "X tests"
		var passCount, failCount int
		for _, line := range lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "passed") || strings.Contains(lower, "passing") {
				if n := extractFirstInt(line); n > 0 {
					passCount = n
				}
			}
			if strings.Contains(lower, "failed") || strings.Contains(lower, "failing") {
				if n := extractFirstInt(line); n > 0 {
					failCount = n
				}
			}
		}
		if passCount+failCount > 0 {
			sb.WriteString(fmt.Sprintf("Tests: %d passed, %d failed\n", passCount, failCount))
		}
		sb.WriteString(truncateOutput(output, 4096))
	case projectPython:
		// pytest output: look for "X passed", "X failed", "X error" in the summary line
		for _, line := range lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "passed") || strings.Contains(lower, "failed") || strings.Contains(lower, "error") {
				if strings.Contains(line, "=") { // pytest summary lines use "===" borders
					sb.WriteString(strings.TrimSpace(line))
					sb.WriteByte('\n')
				}
			}
		}
		sb.WriteString(truncateOutput(output, 4096))
	default:
		sb.WriteString(truncateOutput(output, 6144))
		return sb.String()
	}

	if !passed {
		// Include error context: lines around "Error" or "panic".
		sb.WriteString("\nOutput excerpt:\n")
		for _, line := range lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "error") || strings.Contains(lower, "panic") ||
				strings.Contains(lower, "expected") || strings.Contains(lower, "got ") {
				sb.WriteString("  ")
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

func truncateOutput(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n[output truncated]"
}

// extractFirstInt returns the first non-negative integer found in s, or 0 if none.
func extractFirstInt(s string) int {
	n := 0
	inNum := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
			inNum = true
		} else if inNum {
			return n
		}
	}
	if inNum {
		return n
	}
	return 0
}
