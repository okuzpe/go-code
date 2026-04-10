package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFileTool creates or overwrites a UTF-8 text file inside the workspace.
type WriteFileTool struct {
	root string // absolute, clean workspace root
}

// NewWriteFile returns a write_file tool scoped to root.
func NewWriteFile(root string) *WriteFileTool {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &WriteFileTool{root: filepath.Clean(abs)}
}

var _ Tool = (*WriteFileTool)(nil)

func (WriteFileTool) Name() string { return "write_file" }

func (WriteFileTool) Description() string {
	return "Create or overwrite a UTF-8 text file inside the workspace. " +
		"The parent directory must already exist. Content is written atomically. " +
		"Use edit_file for targeted line replacements; use write_file for new files or full rewrites."
}

func (WriteFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path relative to workspace root, or absolute path inside the workspace",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Full UTF-8 content to write (may be empty to create an empty file)",
			},
		},
		"required": []string{"path", "content"},
	}
}

type writeFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Execute implements Tool.
//
// Path resolution: uses resolveWriteTarget — the target file may not exist yet.
// EvalSymlinks is called only on the parent directory (which must exist), and the
// filename is appended afterward. This differs from resolveExistingPathUnderRoot,
// which requires the full path to already exist. Use resolveWriteTarget for any tool
// that creates or overwrites a file; use resolveExistingPathUnderRoot for read-only tools.
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

	if err := atomicWriteFile(resolved, []byte(in.Content), 0o600); err != nil {
		return Result{Content: fmt.Sprintf("write file: %v", err), IsError: true}, nil
	}

	return Result{Content: fmt.Sprintf("wrote %d bytes to %s", len(in.Content), in.Path)}, nil
}

// resolveWriteTarget validates and resolves the target path for writing.
// Unlike resolveExistingPathUnderRoot, it evaluates symlinks on the parent directory
// because the target file may not exist yet.
func (t *WriteFileTool) resolveWriteTarget(userPath string) (string, error) {
	return resolveWriteTargetUnderRoot(t.root, userPath)
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
