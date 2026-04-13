package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EditFileTool replaces an exact string occurrence in a file (str_replace style).
type EditFileTool struct {
	scope PathScope
}

// NewEditFile returns edit_file with Root and RelativeBase both set to root.
func NewEditFile(root string) *EditFileTool {
	return NewEditFileScope(PathScope{Root: root, RelativeBase: root})
}

// NewEditFileScope returns edit_file with explicit PathScope.
func NewEditFileScope(scope PathScope) *EditFileTool {
	s := NormalizePathScope(scope)
	if strings.TrimSpace(s.RelativeBase) == "" {
		s.RelativeBase = s.Root
	}
	return &EditFileTool{scope: s}
}

var _ Tool = (*EditFileTool)(nil)

func (EditFileTool) Name() string { return "edit_file" }

func (EditFileTool) Description() string {
	return "Replace an exact string in a file (str_replace). " +
		"old_string and new_string must match the file's raw on-disk text. " +
		"If read_file showed lines as N<TAB>content, do not include the line number or that tab in old_string — only the real file lines. " +
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
				"description": "Path relative to launch cwd, or absolute path to an existing file",
			},
			"old_string": map[string]any{
				"type": "string",
				"description": "Exact substring to find in the file as stored on disk (must be non-empty). " +
					"Do not paste read_file line-number prefixes (leading digits + tab); copy only the file text.",
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

// editNotFoundError builds an actionable error message when old_string has zero matches.
// It looks for the first line of oldString in the file and, when found, returns a snippet
// of surrounding context so the model can correct its old_string without another read_file call.
func editNotFoundError(path, oldString, fileContent string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "old_string not found in %s (0 matches)\n", path)
	builder.WriteString("Hint: do not include read_file line-number prefixes (e.g. \"   1\\t\") in old_string — copy only the raw file text.\n")

	// Try to locate the first non-empty line of oldString in the file.
	firstLine := ""
	for _, line := range strings.Split(oldString, "\n") {
		if strings.TrimSpace(line) != "" {
			firstLine = line
			break
		}
	}

	if firstLine == "" {
		return builder.String()
	}

	fileLines := strings.Split(fileContent, "\n")
	foundIndex := -1
	for i, line := range fileLines {
		if strings.Contains(line, strings.TrimSpace(firstLine)) {
			foundIndex = i
			break
		}
	}

	if foundIndex < 0 {
		fmt.Fprintf(&builder, "(first line of old_string %q not found in file either — verify the exact text)", strings.TrimSpace(firstLine))
		return builder.String()
	}

	const contextLines = 4
	start := max(0, foundIndex-contextLines)
	end := min(foundIndex+contextLines+1, len(fileLines))

	fmt.Fprintf(&builder, "Nearest match found near line %d. Surrounding context:\n", foundIndex+1)
	for i := start; i < end; i++ {
		marker := "  "
		if i == foundIndex {
			marker = "> "
		}
		fmt.Fprintf(&builder, "%s%4d\t%s\n", marker, i+1, fileLines[i])
	}
	builder.WriteString("Copy the exact lines above (without the line-number prefix) into old_string.")
	return builder.String()
}

// Execute implements Tool.
//
// Path resolution: uses ResolveReadExistingPath — edit_file requires the target
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

	resolved, err := ResolveReadExistingPath(t.scope, in.Path)
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
		return Result{Content: editNotFoundError(in.Path, in.OldString, content), IsError: true}, nil
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
