package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool reads text files with size/line caps using PathScope for path resolution.
type ReadFileTool struct {
	scope PathScope
}

// NewReadFile returns read_file with Root and RelativeBase both set to root (typical tests).
func NewReadFile(root string) *ReadFileTool {
	return NewReadFileScope(PathScope{Root: root, RelativeBase: root})
}

// NewReadFileScope returns read_file with an explicit PathScope (see package comment).
func NewReadFileScope(scope PathScope) *ReadFileTool {
	s := NormalizePathScope(scope)
	if strings.TrimSpace(s.RelativeBase) == "" {
		s.RelativeBase = s.Root
	}
	return &ReadFileTool{scope: s}
}

var _ Tool = (*ReadFileTool)(nil)

func (ReadFileTool) Name() string { return "read_file" }

func (ReadFileTool) Description() string {
	return "Read a UTF-8 text file, or list the contents of a directory. " +
		"Output includes line numbers (N⇥content) for reference — the numbers are not part of the file content. " +
		"Symlinks are resolved. Relative paths resolve from the process working directory at agent start; absolute paths are used as-is."
}

func (ReadFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File or directory path: relative to launch cwd, or absolute (any location the OS allows)",
			},
			"offset_lines": map[string]any{
				"type":        "integer",
				"description": "Skip this many lines before reading (optional)",
			},
			"limit_lines": map[string]any{
				"type":        "integer",
				"description": "Maximum lines to return (optional, capped at 400)",
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
//
// Path resolution: ResolveReadExistingPath — the target must already exist.
// EvalSymlinks is applied to the resolved path. For writes to paths that may not exist yet,
// use ResolveWriteTargetPath (write_file.go), which resolves only the parent directory.
func (t *ReadFileTool) Execute(ctx context.Context, input string) (Result, error) {
	_ = ctx
	var in readFileInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid json input: %v", err), IsError: true}, nil
	}
	in.Path = strings.TrimSpace(in.Path)
	if in.Path == "" {
		return Result{Content: "path is required", IsError: true}, nil
	}

	resolved, err := ResolveReadExistingPath(t.scope, in.Path)
	if err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}

	f, err := os.Open(resolved)
	if err != nil {
		return Result{Content: fmt.Sprintf("open file: %v", err), IsError: true}, nil
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Result{Content: fmt.Sprintf("stat file: %v", err), IsError: true}, nil
	}
	if st.IsDir() {
		return listDirectory(resolved, in.Path), nil
	}

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

	startLine := in.OffsetLines + 1
	if startLine < 1 {
		startLine = 1
	}
	numbered := make([]string, len(lines))
	for i, l := range lines {
		numbered[i] = fmt.Sprintf("%4d\t%s", startLine+i, l)
	}
	out := strings.Join(numbered, "\n")
	if truncated {
		out += fmt.Sprintf("\n\n[output truncated: max %d lines or %d bytes]", limitLines, MaxReadFileBytes)
	}
	return Result{Content: out, IsError: false}, nil
}

// dirSkip lists subdirectory names that are never walked when listing a directory reference.
var dirSkip = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
}

// listDirectory walks resolved (an absolute directory path) and returns a Result with one
// path per line relative to that directory (slash-separated). Directories end with "/".
// Capped at MaxReadFileLines entries.
func listDirectory(resolved, displayPath string) Result {
	var entries []string
	truncated := false
	_ = filepath.WalkDir(resolved, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p == resolved {
			return nil
		}
		if d.IsDir() && dirSkip[d.Name()] {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(resolved, p)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			relSlash += "/"
		}
		entries = append(entries, relSlash)
		if len(entries) >= MaxReadFileLines {
			truncated = true
			return fs.SkipAll
		}
		return nil
	})
	header := fmt.Sprintf("(directory: %s, %d entries)\n", displayPath, len(entries))
	body := strings.Join(entries, "\n")
	if truncated {
		body += fmt.Sprintf("\n[truncated at %d entries]", MaxReadFileLines)
	}
	return Result{Content: header + body}
}
