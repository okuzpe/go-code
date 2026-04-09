package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveExistingPathUnderRoot resolves userPath to an absolute path that exists under root,
// following symlinks.
//
// Path resolution strategy (Issue #11):
// - Accept either an absolute path or a path relative to the tool's workspace root.
// - Clean the candidate path (so `a/../b` normalizes).
// - Evaluate symlinks on the full path to obtain the real on-disk location.
// - Evaluate symlinks on root as well (best-effort; if it fails, fall back to root).
// - Enforce workspace scoping by checking `filepath.Rel(rootEval, eval)` does not escape.
//
// This prevents common symlink-escape tricks where a path appears to be inside the workspace
// before resolving symlinks but points outside after resolution.
//
// Used by read_file, grep, and edit_file for consistent boundary checks.
func resolveExistingPathUnderRoot(root, userPath string) (string, error) {
	candidate := userPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, userPath)
	}
	candidate = filepath.Clean(candidate)

	eval, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist")
		}
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}

	rootEval, err := filepath.EvalSymlinks(root)
	if err != nil {
		rootEval = root
	}

	rel, err := filepath.Rel(rootEval, eval)
	if err != nil {
		return "", fmt.Errorf("path escapes workspace")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return eval, nil
}
