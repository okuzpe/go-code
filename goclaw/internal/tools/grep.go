package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GrepTool searches file contents under the workspace with a regular expression.
type GrepTool struct {
	root string
}

// NewGrep returns a grep tool scoped to root (directory).
func NewGrep(root string) *GrepTool {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &GrepTool{root: filepath.Clean(abs)}
}

var _ Tool = (*GrepTool)(nil)

func (GrepTool) Name() string { return "grep" }

func (GrepTool) Description() string {
	return "Search UTF-8 text files under the workspace for a regular expression (RE2 syntax). " +
		"Returns path:line:content lines, capped for size."
}

func (GrepTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression (Go regexp / RE2)",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Optional file or directory relative to workspace (default: entire workspace)",
			},
		},
		"required": []string{"pattern"},
	}
}

type grepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

// Execute implements Tool.
func (t *GrepTool) Execute(ctx context.Context, input string) (Result, error) {
	_ = ctx
	var in grepInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: "", IsError: true}, fmt.Errorf("invalid json input: %w", err)
	}
	pat := strings.TrimSpace(in.Pattern)
	if pat == "" {
		return Result{Content: "pattern is required", IsError: true}, nil
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return Result{Content: fmt.Sprintf("invalid regexp: %v", err), IsError: true}, nil
	}

	target := strings.TrimSpace(in.Path)
	if target == "" {
		target = "."
	}
	resolved, resErr := resolvePathUnderRoot(t.root, target)
	if resErr != nil {
		return Result{Content: resErr.Error(), IsError: true}, nil
	}

	var b strings.Builder
	matchCount := 0
	emit := func(relSlash string, lineNo int, line string) bool {
		if matchCount >= MaxGrepMatches {
			return false
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(relSlash)
		fmt.Fprintf(&b, ":%d:%s", lineNo, line)
		matchCount++
		return matchCount < MaxGrepMatches
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return Result{Content: fmt.Sprintf("stat: %v", err), IsError: true}, nil
	}
	if info.IsDir() {
		err = filepath.WalkDir(resolved, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if matchCount >= MaxGrepMatches {
				return fs.SkipAll
			}
			relFile, err := filepath.Rel(t.root, path)
			if err != nil {
				return err
			}
			grepOneFile(path, filepath.ToSlash(relFile), re, emit)
			if matchCount >= MaxGrepMatches {
				return fs.SkipAll
			}
			return nil
		})
	} else {
		relFile, err := filepath.Rel(t.root, resolved)
		if err != nil {
			return Result{Content: err.Error(), IsError: true}, nil
		}
		grepOneFile(resolved, filepath.ToSlash(relFile), re, emit)
	}
	if err != nil {
		return Result{Content: fmt.Sprintf("grep: %v", err), IsError: true}, nil
	}

	s := strings.TrimSpace(b.String())
	if s == "" {
		s = "(no matches)"
	} else if matchCount >= MaxGrepMatches {
		s += fmt.Sprintf("\n\n[truncated at %d matches]", MaxGrepMatches)
	}
	return Result{Content: s, IsError: false}, nil
}

func grepOneFile(absPath, relSlash string, re *regexp.Regexp, emit func(string, int, string) bool) {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return
	}
	if len(raw) > MaxGrepFileBytes {
		raw = raw[:MaxGrepFileBytes]
	}
	if bytes.IndexByte(raw, 0) >= 0 {
		return
	}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if !re.MatchString(line) {
			continue
		}
		if !emit(relSlash, lineNo, line) {
			break
		}
	}
}

// resolvePathUnderRoot resolves a user path to an absolute path under root (same rules as read_file).
func resolvePathUnderRoot(root, userPath string) (string, error) {
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
