package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditFileTool replaces an exact string occurrence in a file (str_replace style).
type EditFileTool struct {
	root string // absolute, clean workspace root
}

// NewEditFile returns an edit_file tool scoped to root.
func NewEditFile(root string) *EditFileTool {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &EditFileTool{root: filepath.Clean(abs)}
}

var _ Tool = (*EditFileTool)(nil)

func (EditFileTool) Name() string { return "edit_file" }

func (EditFileTool) Description() string {
	return "Replace an exact string in a file (str_replace). " +
		"By default old_string must appear exactly once; set replace_all:true to replace every occurrence. " +
		"The edit is written atomically and the file's original permissions are preserved. " +
		"Use write_file for new files or full rewrites."
}

func (EditFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path relative to workspace root, or absolute path inside the workspace",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "Exact string to find in the file (must be non-empty)",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "Replacement string; may be empty to delete the matched text",
			},
			"replace_all": map[string]any{
				"type":        "boolean",
				"description": "Replace all occurrences (default false — requires exactly one occurrence)",
			},
		},
		"required": []string{"path", "old_string", "new_string"},
	}
}

type editFileInput struct {
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

// Execute implements Tool.
//
// Path resolution: uses resolveExistingPathUnderRoot — edit_file requires the target
// file to already exist (it reads, replaces, and atomically rewrites it). EvalSymlinks
// is called on the full path before reading; the atomic rewrite writes to the resolved
// real path (not the symlink), preserving the original file permissions.
// For creating new files that do not yet exist, use write_file (resolveWriteTarget).
func (t *EditFileTool) Execute(_ context.Context, input string) (Result, error) {
	var in editFileInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid json input: %v", err), IsError: true}, nil
	}

	in.Path = strings.TrimSpace(in.Path)
	if in.Path == "" {
		return Result{Content: "path is required", IsError: true}, nil
	}
	// old_string must be non-empty: strings.Count("x", "") == len("x")+1 which is meaningless.
	if in.OldString == "" {
		return Result{Content: "old_string is required", IsError: true}, nil
	}

	resolved, err := resolveExistingPathUnderRoot(t.root, in.Path)
	if err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}

	raw, err := os.ReadFile(resolved)
	if err != nil {
		return Result{Content: fmt.Sprintf("read file: %v", err), IsError: true}, nil
	}
	content := string(raw)

	// strings.Count counts non-overlapping occurrences — consistent with strings.Replace behaviour.
	count := strings.Count(content, in.OldString)
	if count == 0 {
		return Result{Content: fmt.Sprintf("old_string not found in %s (0 matches)", in.Path), IsError: true}, nil
	}
	if !in.ReplaceAll && count > 1 {
		return Result{
			Content: fmt.Sprintf(
				"old_string found %d times in %s; use replace_all:true or provide a more specific string",
				count, in.Path,
			),
			IsError: true,
		}, nil
	}

	var result string
	var n int
	if in.ReplaceAll {
		result = strings.ReplaceAll(content, in.OldString, in.NewString)
		n = count
	} else {
		result = strings.Replace(content, in.OldString, in.NewString, 1)
		n = 1
	}

	if len(result) > MaxWriteFileBytes {
		return Result{
			Content: fmt.Sprintf("result too large: %d bytes exceeds %d-byte limit", len(result), MaxWriteFileBytes),
			IsError: true,
		}, nil
	}

	// Preserve the existing file's permission bits.
	perm := os.FileMode(0o600)
	if info, statErr := os.Stat(resolved); statErr == nil {
		perm = info.Mode().Perm()
	}

	if err := atomicWriteFile(resolved, []byte(result), perm); err != nil {
		return Result{Content: fmt.Sprintf("write file: %v", err), IsError: true}, nil
	}

	return Result{Content: fmt.Sprintf("replaced %d occurrence(s) in %s", n, in.Path)}, nil
}
