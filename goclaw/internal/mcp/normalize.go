package mcp

import (
	"strings"
	"unicode"
)

const maxNormalizedToolNameLen = 64

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
	if len(out) > maxNormalizedToolNameLen {
		out = out[:maxNormalizedToolNameLen]
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
