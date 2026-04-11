package inputprefix

import (
	"os"
	"path/filepath"
	"strings"
)

// TryPasteAsAtPaths tries to interpret pasted text as one or more file/directory
// paths (e.g. from a drag-and-drop into the terminal). If the entire paste content
// consists of absolute paths that exist under workdir, it returns them as
// space-separated "@relpath" tokens suitable for insertion into the chat input.
//
// Returns ("", false) when the paste looks like regular text rather than file paths.
func TryPasteAsAtPaths(workdir, content string) (string, bool) {
	content = strings.TrimSpace(content)
	if content == "" || strings.TrimSpace(workdir) == "" {
		return "", false
	}
	absRoot, err := filepath.Abs(workdir)
	if err != nil {
		absRoot = workdir
	}
	absRoot = filepath.Clean(absRoot)

	candidates := splitPastedPaths(content)
	if len(candidates) == 0 {
		return "", false
	}
	var tokens []string
	for _, raw := range candidates {
		raw = strings.TrimSpace(raw)
		raw = strings.Trim(raw, `"'`) // strip shell-style quoting
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !filepath.IsAbs(raw) {
			return "", false // not an absolute path → regular text
		}
		info, statErr := os.Stat(raw)
		if statErr != nil {
			return "", false // path doesn't exist → regular text
		}
		rel, relErr := filepath.Rel(absRoot, raw)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			return "", false // outside workspace
		}
		relSlash := filepath.ToSlash(rel)
		var tok string
		if relSlash == "." {
			tok = "@./" // workspace root → read_file lists the root directory
		} else {
			tok = "@" + relSlash
			if info.IsDir() {
				tok += "/"
			}
		}
		tokens = append(tokens, tok)
	}
	if len(tokens) == 0 {
		return "", false
	}
	return strings.Join(tokens, " "), true
}

// splitPastedPaths splits a drag-drop paste into individual path candidates.
// Handles: single path, quoted paths ("p1" "p2"), newline-separated paths.
func splitPastedPaths(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Newline-separated (some terminals/OS drag multiple files this way).
	if strings.Contains(s, "\n") {
		var parts []string
		for _, line := range strings.Split(s, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				parts = append(parts, line)
			}
		}
		return parts
	}
	// Quoted sequence: "path1" "path2" or 'path1' 'path2' (Windows shell / macOS).
	if len(s) > 0 && (s[0] == '"' || s[0] == '\'') {
		return parseQuotedPaths(s)
	}
	// Single unquoted path.
	return []string{s}
}

// parseQuotedPaths parses a sequence of shell-quoted path tokens.
func parseQuotedPaths(s string) []string {
	var paths []string
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}
		var quote byte
		if s[0] == '"' {
			quote = '"'
		} else if s[0] == '\'' {
			quote = '\''
		}
		if quote != 0 {
			end := strings.IndexByte(s[1:], quote)
			if end < 0 {
				paths = append(paths, s[1:]) // unclosed quote
				break
			}
			paths = append(paths, s[1:end+1])
			s = s[end+2:]
		} else {
			// Unquoted segment until next whitespace or quote.
			sp := strings.IndexAny(s, " \t\"'")
			if sp < 0 {
				paths = append(paths, s)
				break
			}
			paths = append(paths, s[:sp])
			s = s[sp:]
		}
	}
	return paths
}
