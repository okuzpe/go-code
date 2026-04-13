package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

// GlobTool lists file paths under a directory matching a glob pattern.
// If the pattern contains no path separator, it matches basenames in any subdirectory.
// Otherwise filepath.Match is applied to paths relative to the walk root.
type GlobTool struct {
	scope PathScope
}

// NewGlob returns glob with Root and RelativeBase both set to root.
func NewGlob(root string) *GlobTool {
	return NewGlobScope(PathScope{Root: root, RelativeBase: root})
}

// NewGlobScope returns glob with explicit PathScope.
func NewGlobScope(scope PathScope) *GlobTool {
	s := NormalizePathScope(scope)
	if strings.TrimSpace(s.RelativeBase) == "" {
		s.RelativeBase = s.Root
	}
	return &GlobTool{scope: s}
}

var _ Tool = (*GlobTool)(nil)

func (GlobTool) Name() string { return "glob" }

func (GlobTool) Description() string {
	return "List files under a directory matching a glob. Use a basename pattern (e.g. *.go) to match any depth; " +
		"**/*.go is accepted as an alias for *.go (any depth). " +
		"use path/globs (e.g. internal/*.go) relative to the walk root. " +
		"Optional under: directory to walk (default: configured project / tool path root; same default tree as grep when path is omitted)."
}

func (GlobTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": `Glob pattern relative to the walk root. Examples: "*.go" or "**/*.go" matches any Go file at any depth; "internal/*.go" matches only paths under internal/; path-aware patterns use / as separator (see tool description).`,
			},
			"under": map[string]any{
				"type":        "string",
				"description": "Optional directory to walk (absolute or relative to launch cwd). When empty, walks the default project tree (tool path root in system prompt).",
			},
		},
		"required": []string{"pattern"},
	}
}

type globInput struct {
	Pattern string `json:"pattern"`
	Under   string `json:"under,omitempty"`
}

// normalizeGlobDoubleStar maps shell-style "**/<basename-glob>" to basename-only matching.
// Go's path.Match does not treat ** as recursive; patterns like "**/*.go" would otherwise
// match nothing in a normal tree. When the suffix has no further '/', we match filepath.Base(rel).
func normalizeGlobDoubleStar(pat string) (newPat string, basenameOnly bool) {
	pat = strings.TrimSpace(pat)
	if pat == "" {
		return pat, false
	}
	s := filepath.ToSlash(pat)
	s = strings.TrimPrefix(s, "./")
	if !strings.HasPrefix(s, "**/") {
		return pat, false
	}
	suffix := strings.TrimPrefix(s, "**/")
	if suffix == "" || strings.Contains(suffix, "/") {
		return pat, false
	}
	return suffix, true
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

	basenameOnly := false
	if np, bio := normalizeGlobDoubleStar(pat); bio {
		pat = np
		basenameOnly = true
	}
	pathAware := !basenameOnly && (strings.ContainsRune(pat, '/') || strings.ContainsRune(pat, '\\'))

	walkRoot, wErr := ResolveGlobWalkRoot(t.scope, in.Under)
	if wErr != nil {
		return Result{Content: wErr.Error(), IsError: true}, nil
	}
	if st, statErr := os.Stat(walkRoot); statErr != nil || !st.IsDir() {
		if statErr != nil {
			return Result{Content: fmt.Sprintf("glob root: %v", statErr), IsError: true}, nil
		}
		return Result{Content: "glob under: not a directory", IsError: true}, nil
	}

	var matches []string
	err := filepath.WalkDir(walkRoot, func(fullPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(walkRoot, fullPath)
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
