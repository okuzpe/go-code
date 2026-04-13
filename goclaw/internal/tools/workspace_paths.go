package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathScope configures path resolution for file tools:
//   - Root: default directory tree for glob (no "under") and grep (no path, or path ".")
//   - RelativeBase: directory used to resolve relative paths in read/write/edit/patch and in
//     glob "under" / grep "path" when those are relative (typically goclaw's process cwd at start).
// Absolute paths are passed through and cleaned; the OS enforces existence and permissions.
type PathScope struct {
	Root         string
	RelativeBase string
}

// NormalizePathScope cleans Root and RelativeBase to absolute paths when possible.
func NormalizePathScope(s PathScope) PathScope {
	root := strings.TrimSpace(s.Root)
	if root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			root = filepath.Clean(abs)
		} else {
			root = filepath.Clean(root)
		}
	}
	rb := strings.TrimSpace(s.RelativeBase)
	if rb != "" {
		if abs, err := filepath.Abs(rb); err == nil {
			rb = filepath.Clean(abs)
		} else {
			rb = filepath.Clean(rb)
		}
	}
	return PathScope{Root: root, RelativeBase: rb}
}

// RelBase returns RelativeBase when set, otherwise Root.
func (s PathScope) RelBase() string {
	if strings.TrimSpace(s.RelativeBase) != "" {
		return s.RelativeBase
	}
	return s.Root
}

// ResolveReadExistingPath resolves an existing file or directory for read-only tools.
func ResolveReadExistingPath(scope PathScope, userPath string) (string, error) {
	scope = NormalizePathScope(scope)
	return resolveExistingPathUnrestricted(scope.RelBase(), userPath)
}

func resolveExistingPathUnrestricted(relBase, userPath string) (string, error) {
	userPath = strings.TrimSpace(userPath)
	if userPath == "" {
		return "", fmt.Errorf("path is required")
	}
	var candidate string
	if filepath.IsAbs(userPath) {
		candidate = filepath.Clean(userPath)
	} else {
		base, err := filepath.Abs(relBase)
		if err != nil {
			return "", fmt.Errorf("resolve base path: %w", err)
		}
		candidate = filepath.Clean(filepath.Join(base, userPath))
	}
	eval, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist")
		}
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}
	return eval, nil
}

// ResolveWriteTargetPath resolves the target path for write_file, edit_file, and patch.
func ResolveWriteTargetPath(scope PathScope, userPath string) (string, error) {
	scope = NormalizePathScope(scope)
	return resolveWriteTargetUnrestricted(scope.RelBase(), userPath)
}

func resolveWriteTargetUnrestricted(relBase, userPath string) (string, error) {
	userPath = strings.TrimSpace(userPath)
	if userPath == "" {
		return "", fmt.Errorf("path is required")
	}
	var candidate string
	if filepath.IsAbs(userPath) {
		candidate = filepath.Clean(userPath)
	} else {
		base, err := filepath.Abs(relBase)
		if err != nil {
			return "", fmt.Errorf("resolve base path: %w", err)
		}
		candidate = filepath.Clean(filepath.Join(base, userPath))
	}
	parentDir := filepath.Dir(candidate)
	evalParent, err := filepath.EvalSymlinks(parentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("parent directory does not exist: %s", parentDir)
		}
		return "", fmt.Errorf("resolve parent directory: %w", err)
	}
	return filepath.Join(evalParent, filepath.Base(candidate)), nil
}

// ResolveGlobWalkRoot returns the directory filepath.WalkDir should start from.
func ResolveGlobWalkRoot(scope PathScope, under string) (string, error) {
	scope = NormalizePathScope(scope)
	under = strings.TrimSpace(under)
	if under == "" {
		return filepath.Clean(scope.Root), nil
	}
	return ResolveReadExistingPath(scope, under)
}
