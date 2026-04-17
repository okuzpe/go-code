package slashcmd

import "strings"

// MarkdownFencedPlain wraps arbitrary plain text in a markdown fenced code block,
// extending the fence if the body contains the closing sequence.
func MarkdownFencedPlain(plain string) string {
	plain = strings.TrimRight(plain, "\n")
	fence := "```"
	for strings.Contains(plain, fence) {
		fence += "`"
	}
	var b strings.Builder
	b.WriteString(fence)
	b.WriteByte('\n')
	b.WriteString(plain)
	b.WriteByte('\n')
	b.WriteString(fence)
	b.WriteByte('\n')
	return b.String()
}
