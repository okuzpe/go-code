package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	pathpkg "path"
	"path/filepath"
	"strings"
)

// GlobTool lists file paths under a workspace matching a glob pattern.
// If the pattern contains no path separator, it matches basenames in any subdirectory.
// Otherwise filepath.Match is applied to paths relative to the workspace.
type GlobTool struct {
	root string
}

// NewGlob returns a glob tool scoped to root (directory).
func NewGlob(root string) *GlobTool {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &GlobTool{root: filepath.Clean(abs)}
}

var _ Tool = (*GlobTool)(nil)

func (GlobTool) Name() string { return "glob" }

func (GlobTool) Description() string {
	return "List files under the workspace matching a glob. Use a basename pattern (e.g. *.go) to match any depth; " +
		"use path/globs (e.g. internal/*.go) for a path relative to the workspace root."
}

func (GlobTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": `Glob pattern relative to workspace. Examples: "*.go" matches any Go file at any depth; "internal/*.go" matches only paths under internal/; path-aware patterns use / as separator (see tool description).`,
			},
		},
		"required": []string{"pattern"},
	}
}

type globInput struct {
	Pattern string `json:"pattern"`
}

// Execute implements Tool.
func (t *GlobTool) Execute(ctx context.Context, input string) (Result, error) {
	_ = ctx
	var in globInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: "", IsError: true}, fmt.Errorf("invalid json input: %w", err)
	}
	pat := strings.TrimSpace(in.Pattern)
	if pat == "" {
		return Result{Content: "pattern is required", IsError: true}, nil
	}
	if strings.Contains(pat, "..") {
		return Result{Content: "pattern must not contain ..", IsError: true}, nil
	}

	pathAware := strings.ContainsRune(pat, '/') || strings.ContainsRune(pat, '\\')

	var matches []string
	err := filepath.WalkDir(t.root, func(fullPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(t.root, fullPath)
		if err != nil {
			return err
		}
		var ok bool
		var mErr error
		if pathAware {
			ok, mErr = pathpkg.Match(filepath.ToSlash(pat), filepath.ToSlash(rel))
		} else {
			ok, mErr = filepath.Match(pat, filepath.Base(rel))
		}
		if mErr != nil {
			return mErr
		}
		if !ok {
			return nil
		}
		matches = append(matches, filepath.ToSlash(rel))
		if len(matches) >= MaxGlobMatches {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return Result{Content: fmt.Sprintf("glob walk: %v", err), IsError: true}, nil
	}

	if len(matches) == 0 {
		return Result{Content: "(no matches)", IsError: false}, nil
	}
	out := strings.Join(matches, "\n")
	if len(matches) >= MaxGlobMatches {
		out += fmt.Sprintf("\n\n[truncated at %d matches]", MaxGlobMatches)
	}
	return Result{Content: out, IsError: false}, nil
}
