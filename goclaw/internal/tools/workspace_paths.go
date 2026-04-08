package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveExistingPathUnderRoot resolves userPath to an absolute path that exists under root,
// following symlinks. Used by read_file, grep, and edit_file for the same boundary checks.
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
