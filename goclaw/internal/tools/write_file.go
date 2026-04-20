package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFileTool creates or overwrites a UTF-8 text file using PathScope for path resolution.
type WriteFileTool struct {
	scope PathScope
}

// NewWriteFile returns write_file with Root and RelativeBase both set to root.
func NewWriteFile(root string) *WriteFileTool {
	return NewWriteFileScope(PathScope{Root: root, RelativeBase: root})
}

// NewWriteFileScope returns write_file with explicit PathScope.
func NewWriteFileScope(scope PathScope) *WriteFileTool {
	s := NormalizePathScope(scope)
	if strings.TrimSpace(s.RelativeBase) == "" {
		s.RelativeBase = s.Root
	}
	return &WriteFileTool{scope: s}
}

var _ Tool = (*WriteFileTool)(nil)

func (WriteFileTool) Name() string { return "write_file" }

func (WriteFileTool) Description() string {
	return "Create or overwrite a UTF-8 text file. " +
		"Missing parent directories are created automatically. Content is written atomically. " +
		"Symlinks in the parent path are resolved to the real directory; the final path component is not followed, so replacing a symlink file overwrites that path per the OS. " +
		"Use edit_file for targeted line replacements; use write_file for new files or full rewrites."
}

func (WriteFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path relative to launch cwd, or absolute (missing parent directories are created). Parent symlinks are resolved; the basename is not symlink-expanded.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Full UTF-8 content to write (may be empty to create an empty file)",
			},
			"append": map[string]any{
				"type":        "boolean",
				"description": "If true, append content to the file instead of overwriting. Creates the file if it does not exist. Default: false (overwrite).",
			},
		},
		"required": []string{"path", "content"},
	}
}

type writeFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append"`
}

// Execute implements Tool.
//
// Path resolution: uses resolveWriteTarget — the target file may not exist yet.
// EvalSymlinks is called only on the parent directory (created with MkdirAll if needed), and the
// filename is appended afterward. This differs from ResolveReadExistingPath,
// which requires the full path to already exist. Use resolveWriteTarget for any tool
// that creates or overwrites a file; use ResolveReadExistingPath for read-only tools.
func (t *WriteFileTool) Execute(_ context.Context, input string) (Result, error) {
	var in writeFileInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid json input: %v", err), IsError: true}, nil
	}

	in.Path = strings.TrimSpace(in.Path)
	if in.Path == "" {
		return Result{Content: "path is required", IsError: true}, nil
	}

	if len(in.Content) > MaxWriteFileBytes {
		return Result{
			Content: fmt.Sprintf("content too large: %d bytes exceeds %d-byte limit", len(in.Content), MaxWriteFileBytes),
			IsError: true,
		}, nil
	}

	resolved, err := t.resolveWriteTarget(in.Path)
	if err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}

	if in.Append {
		if err := appendToFile(resolved, []byte(in.Content)); err != nil {
			return Result{Content: fmt.Sprintf("append file: %v", err), IsError: true}, nil
		}
		return Result{Content: fmt.Sprintf("appended %d bytes to %s", len(in.Content), in.Path)}, nil
	}

	if err := atomicWriteFile(resolved, []byte(in.Content), 0o600); err != nil {
		return Result{Content: fmt.Sprintf("write file: %v", err), IsError: true}, nil
	}

	return Result{Content: fmt.Sprintf("wrote %d bytes to %s", len(in.Content), in.Path)}, nil
}

// resolveWriteTarget validates and resolves the target path for writing.
// EvalSymlinks runs on the parent directory only because the target file may not exist yet.
func (t *WriteFileTool) resolveWriteTarget(userPath string) (string, error) {
	return ResolveWriteTargetPath(t.scope, userPath)
}

// appendToFile opens targetPath for appending (creating it if needed) and writes data.
// Not atomic — suitable for log-style appends where atomicity is not required.
func appendToFile(targetPath string, data []byte) error {
	f, err := os.OpenFile(targetPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open for append: %w", err)
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("write: %w", writeErr)
	}
	return closeErr
}

// atomicWriteFile writes data to targetPath atomically via a temp file in the same directory.
// perm is applied to the temp file before rename so the final file has the desired mode.
func atomicWriteFile(targetPath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(dir, ".goclaw-write-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("rename to target: %w", err)
	}

	ok = true
	return nil
}
