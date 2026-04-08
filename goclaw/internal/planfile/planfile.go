// Package planfile defines the workspace plan artifact path, size limits, and handoff text
// for the plan → execute workflow (see README and AGENT_PROFILES.md).
package planfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Subdir is the project config directory under the workspace root (relative name only).
const Subdir = ".goclaw"

// DefaultFilename is the default plan file name inside Subdir.
const DefaultFilename = "plan.md"

// MaxBytes caps plan file size to keep REPL handoff payloads bounded.
const MaxBytes = 256 * 1024

// Path returns the default plan file path: <workdir>/.goclaw/plan.md
func Path(workdir string) string {
	return filepath.Join(workdir, Subdir, DefaultFilename)
}

// Read loads plan content from path. The file must be regular, non-empty, and at most MaxBytes.
func Read(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("plan file: %w", err)
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("plan file: not a regular file: %s", path)
	}
	if st.Size() == 0 {
		return "", fmt.Errorf("plan file is empty: %s", path)
	}
	if st.Size() > int64(MaxBytes) {
		return "", fmt.Errorf("plan file exceeds %d bytes: %s", MaxBytes, path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read plan: %w", err)
	}
	return string(b), nil
}

// Template returns the recommended Markdown skeleton for a new plan file.
func Template() string {
	return strings.TrimSpace(`
# Implementation plan

Brief goal (one sentence).

## Steps

1. First step — concrete, verifiable.
2. Second step — files or commands involved.
3. Third step — tests or manual checks.

## Notes

Optional constraints, APIs, or risks.
`) + "\n"
}

// Init writes the default plan file if it does not exist. It creates the .goclaw directory.
// If the file already exists, it returns (false, nil).
func Init(workdir string) (created bool, err error) {
	dir := filepath.Join(workdir, Subdir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, fmt.Errorf("mkdir plan dir: %w", err)
	}
	p := Path(workdir)
	if _, statErr := os.Stat(p); statErr == nil {
		return false, nil
	} else if !os.IsNotExist(statErr) {
		return false, statErr
	}
	if err := os.WriteFile(p, []byte(Template()), 0o600); err != nil {
		return false, fmt.Errorf("write plan template: %w", err)
	}
	return true, nil
}

// HandoffUserMessage builds the user message that starts execution against a saved plan.
func HandoffUserMessage(planPath, planContent string) string {
	// Use forward slashes in the message for cross-platform readability.
	display := filepath.ToSlash(planPath)
	var b strings.Builder
	b.WriteString("The following is an implementation plan saved at ")
	b.WriteString(display)
	b.WriteString(". Execute it step by step using the available tools. ")
	b.WriteString("Update the session task list with todo_write when it helps track progress. ")
	b.WriteString("Do not skip verification or test steps described in the plan.\n\n---\n\n")
	b.WriteString(planContent)
	return b.String()
}

// ResolvePlanArg turns an optional CLI/REPL path argument into an absolute path.
// Empty arg selects the default Path(workdir). Relative paths are joined with workdir.
func ResolvePlanArg(workdir, arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return Path(workdir)
	}
	arg = filepath.FromSlash(arg)
	if filepath.IsAbs(arg) {
		return filepath.Clean(arg)
	}
	return filepath.Join(workdir, arg)
}
