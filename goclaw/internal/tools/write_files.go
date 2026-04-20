package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// WriteFilesTool creates or overwrites multiple files in a single call.
// Files are written atomically per file (temp+rename), but the batch as a whole is not
// transactional — a failure partway through leaves previously written files on disk.
type WriteFilesTool struct {
	scope PathScope
}

// NewWriteFiles returns write_files with Root and RelativeBase both set to root.
func NewWriteFiles(root string) *WriteFilesTool {
	return NewWriteFilesScope(PathScope{Root: root, RelativeBase: root})
}

// NewWriteFilesScope returns write_files with explicit PathScope.
func NewWriteFilesScope(scope PathScope) *WriteFilesTool {
	s := NormalizePathScope(scope)
	if strings.TrimSpace(s.RelativeBase) == "" {
		s.RelativeBase = s.Root
	}
	return &WriteFilesTool{scope: s}
}

var _ Tool = (*WriteFilesTool)(nil)

func (WriteFilesTool) Name() string { return "write_files" }

func (WriteFilesTool) Description() string {
	return "Write multiple files in one call. Each entry specifies a path and content; " +
		"files are created or overwritten. Missing parent directories are created automatically. " +
		"Use for scaffolding or batch changes where issuing separate write_file calls would consume " +
		"too many iterations. Maximum 20 files per call."
}

func (WriteFilesTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"files": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "Path relative to launch cwd, or absolute. Parent directories are created automatically.",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "Full UTF-8 content to write.",
						},
					},
					"required": []string{"path", "content"},
				},
				"description": "List of files to write (max 20).",
			},
		},
		"required": []string{"files"},
	}
}

const maxWriteFilesCount = 20

type writeFilesInput struct {
	Files []writeFileInput `json:"files"`
}

// Execute implements Tool.
func (t *WriteFilesTool) Execute(_ context.Context, input string) (Result, error) {
	var in writeFilesInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid json input: %v", err), IsError: true}, nil
	}
	if len(in.Files) == 0 {
		return Result{Content: "files list is empty", IsError: true}, nil
	}
	if len(in.Files) > maxWriteFilesCount {
		return Result{
			Content: fmt.Sprintf("too many files: %d exceeds limit of %d", len(in.Files), maxWriteFilesCount),
			IsError: true,
		}, nil
	}

	results := make([]string, 0, len(in.Files))
	anyError := false

	for _, entry := range in.Files {
		entry.Path = strings.TrimSpace(entry.Path)
		if entry.Path == "" {
			results = append(results, "ERROR: empty path in file entry")
			anyError = true
			continue
		}
		if len(entry.Content) > MaxWriteFileBytes {
			results = append(results, fmt.Sprintf("ERROR %s: content too large (%d bytes exceeds %d-byte limit)", entry.Path, len(entry.Content), MaxWriteFileBytes))
			anyError = true
			continue
		}
		resolved, err := ResolveWriteTargetPath(t.scope, entry.Path)
		if err != nil {
			results = append(results, fmt.Sprintf("ERROR %s: %v", entry.Path, err))
			anyError = true
			continue
		}
		if err := atomicWriteFile(resolved, []byte(entry.Content), 0o600); err != nil {
			results = append(results, fmt.Sprintf("ERROR %s: %v", entry.Path, err))
			anyError = true
			continue
		}
		results = append(results, fmt.Sprintf("wrote %d bytes to %s", len(entry.Content), entry.Path))
	}

	return Result{Content: strings.Join(results, "\n"), IsError: anyError}, nil
}
