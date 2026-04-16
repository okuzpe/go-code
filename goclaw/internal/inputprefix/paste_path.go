package inputprefix

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// NormalizePasteNewlines converts CRLF and lone CR to LF before paste is passed
// to the textarea or path heuristics. The bubbles textarea sanitizer replaces
// carriage return and newline independently with "\n", so raw "\r\n" from the
// Windows clipboard becomes two newlines and inserts blank lines between pasted rows.
func NormalizePasteNewlines(s string) string {
	if !strings.Contains(s, "\r") {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// pathFromFileURL converts a file: URI (common when terminals/OS inject drag-drop)
// into an absolute host path. Returns ("", false) if s is not a usable file URL.
func pathFromFileURL(s string) (string, bool) {
	s = strings.TrimSpace(s)
	u, err := url.Parse(s)
	if err != nil || !strings.EqualFold(u.Scheme, "file") {
		return "", false
	}
	p := u.Path
	if p == "" && u.Opaque != "" {
		p = u.Opaque
	}
	if p == "" {
		return "", false
	}
	// Windows: file:///C:/Users/... → Path is "/C:/Users/..."
	if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	p = filepath.Clean(filepath.FromSlash(p))
	if p == "" || p == "." {
		return "", false
	}
	ap, err := filepath.Abs(p)
	if err != nil {
		return "", false
	}
	return ap, true
}

// resolvePastedPathCandidate turns one drag-drop / paste segment into an absolute path
// under workdir when possible: absolute paths, file: URLs, or paths relative to workdir.
func resolvePastedPathCandidate(raw, absRoot string) (abs string, ok bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if fp, ok := pathFromFileURL(raw); ok {
		raw = fp
	}
	if filepath.IsAbs(raw) {
		a, err := filepath.Abs(filepath.Clean(raw))
		return a, err == nil
	}
	joined := filepath.Join(absRoot, filepath.Clean(filepath.FromSlash(raw)))
	joinedAbs, err := filepath.Abs(joined)
	if err != nil {
		return "", false
	}
	rel, relErr := filepath.Rel(absRoot, joinedAbs)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return joinedAbs, true
}

// TryPasteAsAtPaths tries to interpret pasted text as one or more file/directory
// paths (e.g. from a drag-and-drop into the terminal). If the entire paste content
// consists of paths that exist under workdir, it returns them as
// space-separated "@relpath" tokens suitable for insertion into the chat input.
//
// Supports: absolute paths, file:// URLs (incl. Windows file:///C:/...), and paths
// relative to workdir (e.g. internal/foo.go).
//
// Returns ("", false) when the paste looks like regular text rather than file paths.
func TryPasteAsAtPaths(workdir, content string) (string, bool) {
	content = strings.TrimSpace(NormalizePasteNewlines(content))
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
	tokens := make([]string, 0, len(candidates))
	for _, raw := range candidates {
		raw = strings.TrimSpace(raw)
		raw = strings.Trim(raw, `"'`) // strip shell-style quoting
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		resolved, resOK := resolvePastedPathCandidate(raw, absRoot)
		if !resOK {
			return "", false
		}
		info, statErr := os.Stat(resolved)
		if statErr != nil {
			return "", false // path doesn't exist → regular text
		}
		rel, relErr := filepath.Rel(absRoot, resolved)
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
		lines := strings.Split(s, "\n")
		parts := make([]string, 0, len(lines))
		for _, line := range lines {
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
	paths := make([]string, 0, 8)
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
