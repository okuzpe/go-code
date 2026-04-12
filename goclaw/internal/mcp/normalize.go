package mcp

import (
	"strings"
	"unicode"
)

// maxNormalizedToolNameRunes caps the full "mcp__server__tool" string (rune count, UTF-8 safe).
const maxNormalizedToolNameRunes = 64

// NormalizeMCPToolName builds the LLM-facing tool name: mcp__<server>__<tool>.
// Characters outside [a-zA-Z0-9_-] become underscores; result is trimmed and capped.
func NormalizeMCPToolName(server, tool string) string {
	s := sanitizeSegment(server)
	t := sanitizeSegment(tool)
	if s == "" {
		s = "server"
	}
	if t == "" {
		t = "tool"
	}
	out := "mcp__" + s + "__" + t
	rs := []rune(out)
	if len(rs) > maxNormalizedToolNameRunes {
		out = string(rs[:maxNormalizedToolNameRunes])
	}
	return out
}

func sanitizeSegment(s string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if !prevUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_-")
	return out
}
