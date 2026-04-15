package text

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// AtRefDisplayMaxRunes is the maximum rune width for @workspace path labels in the CLI
// (transcript chips, compose box, tool previews, path pickers). Long paths collapse to
// @<basename>… so rows stay readable; Tab completion and ExpandInlineAtRefs still use full paths.
const AtRefDisplayMaxRunes = 24

// AtRefDisplayLabel returns a short terminal label for a workspace-relative path or tool path.
// Input may be "@a/b/c.go", "a/b/c.go", a directory "@pkg/", "/abs/path.go", or "C:/x/y.go".
// Output always uses a leading @ for path-like inputs so the CLI reads consistently.
func AtRefDisplayLabel(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	s = filepath.ToSlash(s)
	isDir := strings.HasSuffix(s, "/")
	core := strings.TrimSuffix(s, "/")

	useBaseOnly := false
	switch {
	case strings.HasPrefix(core, "@"):
		core = strings.TrimPrefix(core, "@")
	case filepath.IsAbs(core):
		useBaseOnly = true
	case len(core) >= 2 && core[1] == ':': // Windows "C:/..."
		useBaseOnly = true
	default:
		// bare relative path (tool previews, etc.)
	}

	if useBaseOnly {
		core = filepath.Base(core)
	}

	display := "@" + core
	if isDir {
		display += "/"
	}
	if utf8.RuneCountInString(display) <= AtRefDisplayMaxRunes {
		return display
	}
	base := filepath.Base(strings.TrimSuffix(core, "/"))
	if base == "." || base == "" {
		return TruncateRunes(display, AtRefDisplayMaxRunes)
	}
	out := "@" + TruncateRunes(base, AtRefDisplayMaxRunes-1)
	if isDir {
		out += "/"
	}
	return out
}

// AtRefDisplayMaybePathLine abbreviates a grep-style "path:line:…" or plain path line for tool cards.
func AtRefDisplayMaybePathLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return line
	}
	slash := filepath.ToSlash(line)
	// Windows "C:/x/y.go:12:…" — first ':' is part of the drive letter.
	if len(slash) >= 2 && slash[1] == ':' {
		idx := strings.IndexByte(slash[2:], ':')
		if idx >= 0 {
			idx += 2
			return AtRefDisplayLabel(slash[:idx]) + slash[idx:]
		}
		return AtRefDisplayLabel(slash)
	}
	if i := strings.IndexByte(slash, ':'); i > 0 && strings.Contains(slash[:i], "/") {
		return AtRefDisplayLabel(slash[:i]) + slash[i:]
	}
	if strings.Contains(slash, "/") || filepath.IsAbs(slash) {
		return AtRefDisplayLabel(slash)
	}
	return line
}
