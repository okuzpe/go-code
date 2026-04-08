package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool reads text files within a workspace root with size/line caps.
type ReadFileTool struct {
	root string // absolute, clean workspace root
}

// NewReadFile returns a read_file tool scoped to root (directory).
func NewReadFile(root string) *ReadFileTool {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &ReadFileTool{root: filepath.Clean(abs)}
}

var _ Tool = (*ReadFileTool)(nil)

func (ReadFileTool) Name() string { return "read_file" }

func (ReadFileTool) Description() string {
	return "Read a UTF-8 text file from the workspace. Symlinks are resolved; paths outside the workspace are rejected."
}

func (ReadFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path relative to workspace, or absolute path inside the workspace",
			},
			"offset_lines": map[string]any{
				"type":        "integer",
				"description": "Skip this many lines before reading (optional)",
			},
			"limit_lines": map[string]any{
				"type":        "integer",
				"description": "Maximum lines to return (optional, capped at 200)",
			},
		},
		"required": []string{"path"},
	}
}

type readFileInput struct {
	Path        string `json:"path"`
	OffsetLines int    `json:"offset_lines"`
	LimitLines  int    `json:"limit_lines"`
}

// Execute implements Tool.
func (t *ReadFileTool) Execute(ctx context.Context, input string) (Result, error) {
	_ = ctx
	var in readFileInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: "", IsError: true}, fmt.Errorf("invalid json input: %w", err)
	}
	in.Path = strings.TrimSpace(in.Path)
	if in.Path == "" {
		return Result{Content: "path is required", IsError: true}, nil
	}

	resolved, err := t.resolveUnderRoot(in.Path)
	if err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}

	f, err := os.Open(resolved)
	if err != nil {
		return Result{Content: fmt.Sprintf("open file: %v", err), IsError: true}, nil
	}
	defer f.Close()

	limitLines := in.LimitLines
	if limitLines <= 0 || limitLines > MaxReadFileLines {
		limitLines = MaxReadFileLines
	}

	sc := bufio.NewScanner(f)
	const maxToken = 1024 * 1024 // 1 MiB lines still capped by byte budget below
	buf := make([]byte, 64*1024)
	sc.Buffer(buf, maxToken)

	skipLeft := in.OffsetLines
	if skipLeft < 0 {
		skipLeft = 0
	}

	var lines []string
	bytesOut := 0
	truncated := false

	for sc.Scan() {
		if skipLeft > 0 {
			skipLeft--
			continue
		}
		line := sc.Text()
		add := len(line) + 1
		if len(lines) > 0 && bytesOut+add > MaxReadFileBytes {
			truncated = true
			break
		}
		if len(lines) == 0 && len(line) > MaxReadFileBytes {
			lines = append(lines, line[:MaxReadFileBytes])
			truncated = true
			break
		}
		lines = append(lines, line)
		bytesOut += add
		if len(lines) >= limitLines {
			if sc.Scan() {
				truncated = true
			}
			break
		}
	}
	if err := sc.Err(); err != nil {
		return Result{Content: fmt.Sprintf("read file: %v", err), IsError: true}, nil
	}

	out := strings.Join(lines, "\n")
	if truncated {
		out += fmt.Sprintf("\n\n[output truncated: max %d lines or %d bytes]", limitLines, MaxReadFileBytes)
	}
	return Result{Content: out, IsError: false}, nil
}

func (t *ReadFileTool) resolveUnderRoot(userPath string) (string, error) {
	candidate := userPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(t.root, userPath)
	}
	candidate = filepath.Clean(candidate)

	eval, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist")
		}
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}

	rootEval, err := filepath.EvalSymlinks(t.root)
	if err != nil {
		rootEval = t.root
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
