package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/okuzpe/goclaw/internal/text"
)

// GrepTool searches file contents with a regular expression (RE2).
type GrepTool struct {
	scope PathScope
}

// NewGrep returns grep with Root and RelativeBase both set to root.
func NewGrep(root string) *GrepTool {
	return NewGrepScope(PathScope{Root: root, RelativeBase: root})
}

// NewGrepScope returns grep with explicit PathScope.
func NewGrepScope(scope PathScope) *GrepTool {
	s := NormalizePathScope(scope)
	if strings.TrimSpace(s.RelativeBase) == "" {
		s.RelativeBase = s.Root
	}
	return &GrepTool{scope: s}
}

var _ Tool = (*GrepTool)(nil)

func (GrepTool) Name() string { return "grep" }

func (GrepTool) Description() string {
	return "Search UTF-8 text files for a regular expression (RE2 syntax). " +
		"Returns path:line:content lines, capped for size. " +
		"When path is omitted or \".\", searches the same default project tree as glob with no \"under\"."
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
				"description": "Optional file or directory to search. Omit or use \".\" for the default project tree (same as glob with no \"under\"). Otherwise relative to launch cwd or absolute.",
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
	var in grepInput
	if err := UnmarshalToolInputJSON(input, &in); err != nil {
		return Result{Content: "", IsError: true}, err
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
	var resolved string
	var resErr error
	if target == "" || target == "." {
		resolved, resErr = ResolveGlobWalkRoot(t.scope, "")
	} else {
		resolved, resErr = ResolveReadExistingPath(t.scope, target)
	}
	if resErr != nil {
		return Result{Content: resErr.Error(), IsError: true}, nil
	}

	prog := newPathListProgress(ctx, ProgressReporterFromContext(ctx))
	if prog != nil {
		defer prog.flush()
	}

	var b strings.Builder
	matchCount := 0
	emit := func(relSlash string, lineNo int, line string) bool {
		if matchCount >= MaxGrepMatches {
			return false
		}
		row := relSlash + fmt.Sprintf(":%d:%s", lineNo, line)
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(row)
		if prog != nil {
			_ = prog.addLine(text.TruncateRunes(row, 160))
		}
		matchCount++
		return matchCount < MaxGrepMatches
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return Result{Content: fmt.Sprintf("stat: %v", err), IsError: true}, nil
	}
	var searchRoot string
	if info.IsDir() {
		searchRoot = resolved
	} else {
		searchRoot = filepath.Dir(resolved)
	}
	if info.IsDir() {
		err = filepath.WalkDir(resolved, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if d.IsDir() {
				return nil
			}
			if matchCount >= MaxGrepMatches {
				return fs.SkipAll
			}
			relFile := grepRelPath(searchRoot, path)
			grepOneFile(path, relFile, re, emit)
			if matchCount >= MaxGrepMatches {
				return fs.SkipAll
			}
			return nil
		})
	} else {
		relFile := grepRelPath(searchRoot, resolved)
		grepOneFile(resolved, relFile, re, emit)
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

// grepRelPath returns a slash path for grep output: relative to searchRoot when possible,
// otherwise the absolute path.
func grepRelPath(searchRoot, absPath string) string {
	searchRoot = filepath.Clean(searchRoot)
	absPath = filepath.Clean(absPath)
	rel, err := filepath.Rel(searchRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(absPath)
	}
	return filepath.ToSlash(rel)
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
