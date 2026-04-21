package slashcmd

import (
	"strings"
	"unicode"
)

// ParsedSlashLine describes the slash command token the user is editing at a rune cursor.
// FieldIndex 0 is the command token (including a leading /). FieldIndex 1 is the first argument, etc.
// When the cursor is in whitespace after a token, FieldIndex refers to the next argument (possibly empty).
type ParsedSlashLine struct {
	OK bool

	// Cmd is the normalized first-token command without a leading slash (e.g. "profile").
	Cmd string
	// CmdToken is the raw first token text including slash (e.g. "/profile").
	CmdToken string

	FieldIndex int

	// ReplaceStartRune and ReplaceEndRune bound the active argument token in the original line (half-open in runes).
	// For an empty next argument, ReplaceStartRune == ReplaceEndRune == cursor (insert point).
	ReplaceStartRune int
	ReplaceEndRune   int
}

type runeSpan struct {
	start int
	end   int
}

// ParseSlashLineAtCursor parses a single logical line for slash completion at runeCol (0-based rune index).
// Quoted arguments are not supported: tokens are split on unicode.IsSpace only.
// ok is false when the line does not start (after optional leading ASCII spaces/tabs) with '/'.
func ParseSlashLineAtCursor(line string, runeCol int) (p ParsedSlashLine, ok bool) {
	runes := []rune(line)
	if len(runes) == 0 {
		return
	}
	if runeCol < 0 {
		runeCol = 0
	}
	if runeCol > len(runes) {
		runeCol = len(runes)
	}

	lead := 0
	for lead < len(runes) && (runes[lead] == ' ' || runes[lead] == '\t') {
		lead++
	}
	if lead >= len(runes) || (runes[lead] != '/' && runes[lead] != ':') {
		return
	}
	// Normalize ':' prefix to '/' so the rest of the parser and all callers see a uniform slash command.
	if runes[lead] == ':' {
		runes[lead] = '/'
	}

	var tokens []runeSpan
	i := lead
	for i < len(runes) {
		for i < len(runes) && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= len(runes) {
			break
		}
		start := i
		for i < len(runes) && !unicode.IsSpace(runes[i]) {
			i++
		}
		tokens = append(tokens, runeSpan{start, i})
	}
	if len(tokens) == 0 {
		return
	}

	first := string(runes[tokens[0].start:tokens[0].end])
	if !strings.HasPrefix(first, "/") {
		return
	}
	cmd := strings.ToLower(strings.TrimPrefix(first, "/"))

	var field int
	var repStart, repEnd int

	if runeCol < tokens[0].start {
		field = 0
		repStart, repEnd = tokens[0].start, tokens[0].end
	} else if runeCol < tokens[0].end {
		field = 0
		repStart, repEnd = tokens[0].start, tokens[0].end
	} else if runeCol == tokens[0].end && (runeCol >= len(runes) || !unicode.IsSpace(runes[runeCol])) {
		// Cursor immediately after the command token (end is exclusive): still editing the
		// command name for completion — e.g. "/" or "/help" with cursor at end-of-line. If the
		// cursor sits on whitespace before the next token, that is the first-argument field.
		field = 0
		repStart, repEnd = tokens[0].start, tokens[0].end
	} else {
		for k := 1; k < len(tokens); k++ {
			tok := tokens[k]
			if runeCol < tok.start {
				field = k
				repStart, repEnd = runeCol, runeCol
				p.OK = true
				p.Cmd = cmd
				p.CmdToken = first
				p.FieldIndex = field
				p.ReplaceStartRune = repStart
				p.ReplaceEndRune = repEnd
				return p, true
			}
			if runeCol < tok.end {
				field = k
				repStart, repEnd = tok.start, tok.end
				p.OK = true
				p.Cmd = cmd
				p.CmdToken = first
				p.FieldIndex = field
				p.ReplaceStartRune = repStart
				p.ReplaceEndRune = repEnd
				return p, true
			}
			if runeCol == tok.end {
				field = k + 1
				repStart, repEnd = runeCol, runeCol
				p.OK = true
				p.Cmd = cmd
				p.CmdToken = first
				p.FieldIndex = field
				p.ReplaceStartRune = repStart
				p.ReplaceEndRune = repEnd
				return p, true
			}
		}
		last := tokens[len(tokens)-1]
		field = len(tokens)
		if runeCol > last.end {
			repStart, repEnd = runeCol, runeCol
		} else {
			repStart, repEnd = runeCol, runeCol
		}
	}

	p.OK = true
	p.Cmd = cmd
	p.CmdToken = first
	p.FieldIndex = field
	p.ReplaceStartRune = repStart
	p.ReplaceEndRune = repEnd
	return p, true
}
