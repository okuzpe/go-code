package app

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/text"
	"golang.org/x/term"
)

const chatTitleModelMaxRunes = 40

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := int(f.Fd())
	return term.IsTerminal(fd)
}

// wrapPlain breaks text at spaces to fit width (runes); newlines become spaces first.
func wrapPlain(text string, width int) string {
	if width < 12 {
		width = 76
	}
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	var b strings.Builder
	lineLen := 0
	for _, word := range words {
		wordLen := utf8.RuneCountInString(word)
		if lineLen == 0 {
			b.WriteString(word)
			lineLen = wordLen
			continue
		}
		if lineLen+1+wordLen > width {
			lines = append(lines, b.String())
			b.Reset()
			b.WriteString(word)
			lineLen = wordLen
		} else {
			b.WriteByte(' ')
			b.WriteString(word)
			lineLen += 1 + wordLen
		}
	}
	if b.Len() > 0 {
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, max int) string {
	return text.TruncateRunes(s, max)
}

// FormatChatWindowTitle keeps the TUI header on one line for typical terminal widths.
func FormatChatWindowTitle(provider, model, profile string) string {
	model = truncate(model, chatTitleModelMaxRunes)
	return fmt.Sprintf("goclaw  %s/%s  %s", provider, model, agents.DisplayProfileName(profile))
}
