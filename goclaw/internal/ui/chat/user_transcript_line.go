package chat

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/text"
)

const (
	pathChipMaxRunes = 40
	pathChipMinBare = 10
)

var bareAbsPathPrefixes = []string{
	"/Users/", "/home/", "/private/", "/var/", "/tmp/", "/etc/", "/opt/", "/usr/", "/Volumes/",
}

// renderUserTranscriptLine styles @workspace refs, quoted absolute paths, and common bare
// Unix absolute paths in user transcript lines (drag/drop often pastes quoted paths).
// workdir is the tool workspace root: paths under it render as @rel (e.g. @goclaw for the project root)
// while OSC-8 hyperlinks still target the absolute file:// URL for click-to-open in capable terminals.
func renderUserTranscriptLine(s string, th *Theme, workdir string) string {
	if strings.TrimSpace(s) == "" {
		return s
	}
	return renderUserRefsLine(s, th, workdir, nil)
}

// renderUserRefsLine is the shared tokenizer for @refs and path chips. When renderPlain is nil,
// non-chip text is copied verbatim (transcript caller applies row-level styling). When renderPlain
// is set, every plain run is passed through it so the compose box can match cursor-line / text styles.
func renderUserRefsLine(s string, th *Theme, workdir string, renderPlain func(string) string) string {
	if th == nil {
		th = DefaultTheme()
	}
	rs := []rune(s)
	var b strings.Builder
	b.Grow(len(s) + 64)
	var plain strings.Builder
	flushPlain := func() {
		if plain.Len() == 0 {
			return
		}
		frag := plain.String()
		plain.Reset()
		if renderPlain != nil {
			b.WriteString(renderPlain(frag))
		} else {
			b.WriteString(frag)
		}
	}
	for i := 0; i < len(rs); {
		r := rs[i]
		// @path (start or after whitespace only — avoids emails).
		if r == '@' && (i == 0 || rs[i-1] == ' ' || rs[i-1] == '\t') {
			j := i + 1
			for j < len(rs) && rs[j] != ' ' && rs[j] != '\t' && rs[j] != '\n' {
				j++
			}
			if j > i+1 {
				flushPlain()
				token := string(rs[i:j])
				b.WriteString(th.AtRefChip.Render(text.AtRefDisplayLabel(token)))
				i = j
				continue
			}
		}
		// '…' or "…" with absolute path inside.
		if (r == '\'' || r == '"') && i+2 < len(rs) {
			q := r
			j := i + 1
			for j < len(rs) && rs[j] != q {
				j++
			}
			if j < len(rs) && rs[j] == q {
				inner := strings.TrimSpace(string(rs[i+1 : j]))
				if isAbsPathString(inner) {
					flushPlain()
					b.WriteString(th.Dim.Render(string(q)))
					b.WriteString(pathChip(th, inner, workdir))
					b.WriteString(th.Dim.Render(string(q)))
					i = j + 1
					continue
				}
			}
		}
		// Bare Windows "C:\…" / "C:/…" (after non-alphanumeric boundary).
		if looksLikeWindowsBarePathAt(rs, i) && barePathBoundaryOK(rs, i) {
			if end, ok := scanBareWindowsAbsPath(rs, i); ok {
				flushPlain()
				raw := string(rs[i:end])
				b.WriteString(pathChip(th, raw, workdir))
				i = end
				continue
			}
		}
		// Bare /Users/… /home/… (after non-alphanumeric boundary).
		if r == '/' && barePathBoundaryOK(rs, i) {
			if end, ok := scanBareAbsPath(rs, i); ok {
				flushPlain()
				raw := string(rs[i:end])
				b.WriteString(pathChip(th, raw, workdir))
				i = end
				continue
			}
		}
		plain.WriteRune(r)
		i++
	}
	flushPlain()
	return b.String()
}

func barePathBoundaryOK(rs []rune, i int) bool {
	if i > 0 {
		switch rs[i-1] {
		case ' ', '\t', '\n', '(', '[', '{', '<', ':', '=', ',', ';', '!', '?':
			return true
		}
		prev := rs[i-1]
		if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') ||
			(prev >= '0' && prev <= '9') || prev == '_' || prev == '.' {
			return false
		}
	}
	return true
}

func scanBareAbsPath(rs []rune, i int) (end int, ok bool) {
	raw := string(rs[i:])
	if !hasBareAbsPathPrefix(raw) {
		return 0, false
	}
	j := i
	for j < len(rs) {
		c := rs[j]
		switch c {
		case ' ', '\t', '\n', '\'', '"', ')', ']', '>', '`':
			goto done
		default:
			if pathRuneOK(c) {
				j++
				continue
			}
			goto done
		}
	}
done:
	if j-i < pathChipMinBare {
		return 0, false
	}
	return j, true
}

func pathRuneOK(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '/' || c == '\\' || c == '.' || c == '_' || c == '-' || c == '+' || c == '@' || c == '~':
		return true
	}
	return false
}

func hasBareAbsPathPrefix(s string) bool {
	for _, p := range bareAbsPathPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func isWindowsDriveLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// looksLikeWindowsAbsPath reports drive-letter absolute paths (e.g. C:\… or c:/…).
func looksLikeWindowsAbsPath(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 3 {
		return false
	}
	if !isWindowsDriveLetter(s[0]) || s[1] != ':' {
		return false
	}
	// Require "\" or "/" after ":" so tokens like "n:label" are not treated as paths.
	if s[2] != '\\' && s[2] != '/' {
		return false
	}
	return true
}

func looksLikeWindowsBarePathAt(rs []rune, i int) bool {
	if i+2 >= len(rs) {
		return false
	}
	r0 := rs[i]
	if !((r0 >= 'A' && r0 <= 'Z') || (r0 >= 'a' && r0 <= 'z')) {
		return false
	}
	if rs[i+1] != ':' {
		return false
	}
	if rs[i+2] != '\\' && rs[i+2] != '/' {
		return false
	}
	return true
}

func scanBareWindowsAbsPath(rs []rune, i int) (end int, ok bool) {
	if !looksLikeWindowsBarePathAt(rs, i) {
		return 0, false
	}
	j := i
	for j < len(rs) {
		c := rs[j]
		switch c {
		case ' ', '\t', '\n', '\'', '"', ')', ']', '>', '`':
			goto done
		default:
			if pathRuneOK(c) {
				j++
				continue
			}
			goto done
		}
	}
done:
	if j-i < pathChipMinBare {
		return 0, false
	}
	return j, true
}

func isAbsPathString(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, "://") {
		return false
	}
	if looksLikeWindowsAbsPath(s) {
		return true
	}
	if filepath.IsAbs(s) {
		return true
	}
	// Unix-style absolute paths (also matches some cross-platform strings IsAbs misses).
	return len(s) >= 4 && strings.HasPrefix(s, "/")
}

// workspaceAtChipLabel returns a short @token when abs is inside workdir (after symlink resolution);
// otherwise falls back to tail abbreviation of the absolute path.
func workspaceAtChipLabel(abs, workdir string) string {
	abs = filepath.Clean(strings.TrimSpace(abs))
	workdir = strings.TrimSpace(workdir)
	if workdir == "" || abs == "" {
		return abbreviatePathForTranscript(abs)
	}
	wd := filepath.Clean(workdir)
	absEv, errA := filepath.EvalSymlinks(abs)
	if errA != nil {
		absEv = abs
	}
	wdEv, errW := filepath.EvalSymlinks(wd)
	if errW != nil {
		wdEv = wd
	}
	rel, err := filepath.Rel(wdEv, absEv)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return abbreviatePathForTranscript(abs)
	}
	relSlash := filepath.ToSlash(rel)
	if relSlash == "." {
		base := filepath.Base(wdEv)
		if base != "" && base != "." && base != "/" {
			return "@" + base
		}
		return "@."
	}
	return "@" + relSlash
}

func abbreviatePathForTranscript(abs string) string {
	abs = filepath.Clean(abs)
	if n := utf8.RuneCountInString(abs); n <= pathChipMaxRunes {
		return abs
	}
	rs := []rune(abs)
	tail := pathChipMaxRunes - 1 // "…"
	if len(rs) <= tail {
		return abs
	}
	return "…" + string(rs[len(rs)-tail:])
}

// pathOSC8Hyperlink wraps terminal hyperlink sequences (iTerm2, WezTerm, recent VTE, etc.).
// Full path is available on hover/open where supported; otherwise the abbreviated label still shows.
func pathOSC8Hyperlink(abs string) (open, close string) {
	abs = filepath.Clean(abs)
	if abs == "" || abs == "." {
		return "", ""
	}
	slash := filepath.ToSlash(abs)
	var u string
	switch {
	case len(slash) >= 3 && isWindowsDriveLetter(slash[0]) && slash[1] == ':' &&
		(slash[2] == '/' || slash[2] == '\\'):
		// RFC 8089 style: file:///C:/path
		u = "file:///" + strings.TrimPrefix(slash, "/")
	case strings.HasPrefix(slash, "/"):
		u = "file://" + slash
	default:
		return "", ""
	}
	u = strings.ReplaceAll(u, " ", "%20")
	open = "\x1b]8;;" + u + "\x1b\\"
	close = "\x1b]8;;\x1b\\"
	return open, close
}

func pathChip(th *Theme, abs, workdir string) string {
	label := workspaceAtChipLabel(abs, workdir)
	// Outside-workspace chips use abbreviatePathForTranscript ("…tail"); do not re-alias those.
	pathLike := strings.Contains(label, "/") || strings.Contains(label, `\`)
	if strings.HasPrefix(label, "@") || (pathLike && !strings.HasPrefix(label, "…")) {
		label = text.AtRefDisplayLabel(label)
	}
	oscOpen, oscClose := pathOSC8Hyperlink(abs)
	chip := th.AtRefChip.Render(label)
	if oscOpen == "" {
		return chip
	}
	return oscOpen + chip + oscClose
}
