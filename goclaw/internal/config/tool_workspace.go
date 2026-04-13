package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveToolWorkspace returns the default project directory: system prompt hints, optional
// project manifest snippet (go.mod, etc.), default walk root for glob (no "under") and grep
// (no path / "."), and extra skill search paths when it differs from LaunchDir.
// Relative file paths in read/write/edit/patch still resolve from LaunchDir, not from here.
// LaunchDir is the process cwd used when merging project settings (typically os.Getwd()).
// Override empty uses LaunchDir; otherwise absolute or relative to LaunchDir; must exist as a directory.
func ResolveToolWorkspace(launchDir, override string) (string, error) {
	launchAbs, err := filepath.Abs(launchDir)
	if err != nil {
		return "", fmt.Errorf("resolve launch directory: %w", err)
	}
	override = strings.TrimSpace(override)
	if override == "" {
		return filepath.Clean(launchAbs), nil
	}

	var candidate string
	if filepath.IsAbs(override) {
		candidate = filepath.Clean(override)
	} else {
		candidate = filepath.Clean(filepath.Join(launchAbs, override))
	}

	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve tool workspace: %w", err)
	}

	fi, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("tool_workspace_root %q: %w", abs, err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("tool_workspace_root is not a directory: %s", abs)
	}
	return abs, nil
}
