package inputprefix

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// AtPathSuggest is one workspace-relative path for TUI @ picker rows.
type AtPathSuggest struct {
	RelPath string // forward slashes, relative to workspace
	IsDir   bool
}

const (
	atSuggestMaxPick    = 20
	atSuggestMaxCollect = 2500
	atSuggestMaxDepth   = 12
)

var atWalkSkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
}

// ParseAtPathBuffer returns the path query after @ for a single-line buffer, or ok=false.
func ParseAtPathBuffer(line string) (query string, ok bool) {
	if strings.Contains(line, "\n") {
		return "", false
	}
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "@") {
		return "", false
	}
	q := strings.TrimSpace(s[1:])
	if strings.Contains(q, "..") {
		return "", false
	}
	q = filepath.ToSlash(q)
	return q, true
}

// ExtractAtTokens returns all distinct @path tokens found in s where @ is preceded
// by start-of-string or whitespace. Tokens with ".." or absolute paths are skipped.
// Used by ExpandInlineAtRefs to find files to pre-load as context.
func ExtractAtTokens(s string) []string {
	runes := []rune(s)
	tokens := make([]string, 0, 8)
	seen := map[string]bool{}
	i := 0
	for i < len(runes) {
		if runes[i] == '@' && (i == 0 || runes[i-1] == ' ' || runes[i-1] == '\t' || runes[i-1] == '\n') {
			j := i + 1
			for j < len(runes) && runes[j] != ' ' && runes[j] != '\t' && runes[j] != '\n' {
				j++
			}
			if j > i+1 {
				tok := string(runes[i:j])
				path := strings.TrimPrefix(tok, "@")
				path = strings.TrimSuffix(path, "/")
				if !strings.Contains(path, "..") && !filepath.IsAbs(path) &&
					!strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "\\") && !seen[tok] {
					tokens = append(tokens, tok)
					seen[tok] = true
				}
			}
			i = j
			continue
		}
		i++
	}
	return tokens
}

// AtFragmentAtCursor returns the @token the user is currently typing at runeCol in lineText.
// runeCol is the 0-indexed rune column (as returned by textarea.Column()).
// Scans backward from runeCol; any whitespace before @ means we're not inside an @-token.
// Returns ("@query", startRuneCol, true) or ("", 0, false).
func AtFragmentAtCursor(lineText string, runeCol int) (fragment string, startCol int, ok bool) {
	runes := []rune(lineText)
	if runeCol > len(runes) {
		runeCol = len(runes)
	}
	for i := runeCol - 1; i >= 0; i-- {
		r := runes[i]
		if r == ' ' || r == '\t' {
			return "", 0, false
		}
		if r == '@' {
			q := filepath.ToSlash(string(runes[i+1 : runeCol]))
			if strings.Contains(q, "..") {
				return "", 0, false
			}
			return "@" + q, i, true
		}
	}
	return "", 0, false
}

// TUIAtPathSuggestions returns up to maxPick file/dir paths under workdir whose
// relative path (slash form) prefix-matches query (ASCII case-insensitive).
func TUIAtPathSuggestions(workdir, line string) []AtPathSuggest {
	query, ok := ParseAtPathBuffer(line)
	if !ok || strings.TrimSpace(workdir) == "" {
		return nil
	}
	abs, err := filepath.Abs(workdir)
	if err != nil {
		abs = workdir
	}
	abs = filepath.Clean(abs)
	qLower := strings.ToLower(query)

	var raw []AtPathSuggest
	_ = filepath.WalkDir(abs, func(full string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Name() == "." || d.Name() == ".." {
			return nil
		}
		rel, err := filepath.Rel(abs, full)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		depth := strings.Count(relSlash, "/")
		if depth > atSuggestMaxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() && atWalkSkipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if !atMatchFold(relSlash, qLower) {
			return nil
		}
		raw = append(raw, AtPathSuggest{RelPath: relSlash, IsDir: d.IsDir()})
		if len(raw) >= atSuggestMaxCollect {
			return fs.SkipAll
		}
		return nil
	})
	if len(raw) == 0 {
		return nil
	}
	sortAtSuggests(raw)
	if len(raw) > atSuggestMaxPick {
		raw = raw[:atSuggestMaxPick]
	}
	return raw
}

// atMatchFold reports whether relSlash matches the query (already lowercased).
// When the query contains "/" the user is navigating a path — strict prefix match is used.
// Otherwise: match if the full path starts with the query OR if any path component contains it.
// This lets "@plan" find ".goclaw/plan.md" and "@goclaw" find both "goclaw/" and ".goclaw/".
func atMatchFold(relSlash, qLower string) bool {
	if qLower == "" {
		return true
	}
	low := strings.ToLower(relSlash)
	if strings.HasPrefix(low, qLower) {
		return true
	}
	// Path-navigating query: don't do component search, prefix only.
	if strings.Contains(qLower, "/") {
		return false
	}
	// Match if any path component contains the query string.
	for _, part := range strings.Split(low, "/") {
		if strings.Contains(part, qLower) {
			return true
		}
	}
	return false
}

func sortAtSuggests(s []AtPathSuggest) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && atLess(s[j], s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func atLess(a, b AtPathSuggest) bool {
	da := strings.Count(a.RelPath, "/")
	db := strings.Count(b.RelPath, "/")
	if da != db {
		return da < db
	}
	// Within the same depth: non-hidden entries sort before hidden ones (first component starts with ".").
	aHid := isHiddenRelPath(a.RelPath)
	bHid := isHiddenRelPath(b.RelPath)
	if aHid != bHid {
		return !aHid
	}
	return strings.ToLower(a.RelPath) < strings.ToLower(b.RelPath)
}

// isHiddenRelPath reports whether the top-level path component starts with a dot.
func isHiddenRelPath(relSlash string) bool {
	first := relSlash
	if i := strings.IndexByte(relSlash, '/'); i >= 0 {
		first = relSlash[:i]
	}
	return strings.HasPrefix(first, ".")
}

// atDisplayNames returns @rel paths for suggestions (dirs end with /).
func atDisplayNames(sugs []AtPathSuggest) []string {
	out := make([]string, 0, len(sugs))
	for _, s := range sugs {
		n := "@" + s.RelPath
		if s.IsDir {
			n += "/"
		}
		out = append(out, n)
	}
	return out
}

// longestCommonPrefixASCII returns the longest shared byte prefix (paths are ASCII).
func longestCommonPrefixASCII(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	ref := strs[0]
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		for j := 1; j < len(strs); j++ {
			s := strs[j]
			if i >= len(s) || s[i] != c {
				return ref[:i]
			}
		}
	}
	return ref
}

// AtTabExpand completes @path on a single line.
// It uses the longest common prefix of all matches when that extends what was typed,
// otherwise falls back to the first suggestion — this covers both prefix matches
// and component/basename matches (e.g. "@plan" → "@.goclaw/plan.md").
func AtTabExpand(workdir, line string) (replacement string, ok bool) {
	if strings.Contains(line, "\n") {
		return "", false
	}
	raw := strings.TrimSpace(line)
	if !strings.HasPrefix(raw, "@") || strings.TrimSpace(workdir) == "" {
		return "", false
	}
	sugs := TUIAtPathSuggestions(workdir, raw)
	if len(sugs) == 0 {
		return "", false
	}
	names := atDisplayNames(sugs)
	lcp := longestCommonPrefixASCII(names)
	lowLCP := strings.ToLower(lcp)
	lowRaw := strings.ToLower(raw)
	// If the LCP is a genuine extension of what was typed, use it.
	if strings.HasPrefix(lowLCP, lowRaw) && len(lcp) > len(raw) {
		return lcp, true
	}
	// Otherwise (LCP is shorter, or doesn't share a prefix — component match case):
	// pick the first suggestion so the user always gets a completion on Tab.
	if strings.HasSuffix(names[0], "/") {
		return names[0], true // directory: no trailing space, user continues typing inside
	}
	return names[0] + " ", true // file: trailing space marks completion
}
