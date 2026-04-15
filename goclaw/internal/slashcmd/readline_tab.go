package slashcmd

import (
	"strings"

	"github.com/chzyer/readline"
	"github.com/chzyer/readline/runes"
	"github.com/okuzpe/goclaw/internal/inputprefix"
)

// ReadlineSlashPrefixes returns top-level REPL slash commands for readline Tab completion.
func ReadlineSlashPrefixes() []string {
	out := make([]string, len(slashCommandTable))
	for i, e := range slashCommandTable {
		out[i] = e.Name
	}
	return out
}

// ReadlinePrefixCompleter builds a readline.AutoCompleter for top-level /commands.
func ReadlinePrefixCompleter() *readline.PrefixCompleter {
	items := make([]readline.PrefixCompleterInterface, 0, len(slashCommandTable))
	for _, e := range slashCommandTable {
		items = append(items, readline.PcItem(e.Name))
	}
	return readline.NewPrefixCompleter(items...)
}

// readlineSlashLineCompleter completes top-level /commands and slash arguments when SlashContext is set.
type readlineSlashLineCompleter struct {
	prefix   *readline.PrefixCompleter
	slashCtx func() SlashContext
}

func (c *readlineSlashLineCompleter) Do(line []rune, pos int) ([][]rune, int) {
	if c.prefix == nil {
		return nil, 0
	}
	if pos < 0 {
		pos = 0
	}
	if pos > len(line) {
		pos = len(line)
	}
	if c.slashCtx == nil {
		return c.prefix.Do(line, pos)
	}
	s := string(line)
	parsed, ok := ParseSlashLineAtCursor(s, pos)
	if !ok {
		return c.prefix.Do(line, pos)
	}
	if parsed.FieldIndex == 0 {
		return c.prefix.Do(line, pos)
	}
	sc := c.slashCtx()
	sugs := slashArgSuggestionsParsed(sc, parsed, s, pos)
	if len(sugs) == 0 {
		return nil, 0
	}
	stem := line[parsed.ReplaceStartRune:pos]
	off := len(stem)
	lowStem := strings.ToLower(string(stem))
	var out [][]rune
	for _, sg := range sugs {
		name := []rune(sg.Name)
		nl := strings.ToLower(string(name))
		if !strings.HasPrefix(nl, lowStem) {
			continue
		}
		if len(name) < len(stem) {
			continue
		}
		out = append(out, name[len(stem):])
	}
	if len(out) == 0 {
		return nil, 0
	}
	if len(out) == 1 {
		return out, off
	}
	same, n := runes.Aggregate(out)
	if n > 0 {
		return [][]rune{same}, off
	}
	return out, off
}

// NewReadlineSlashAtCompleter returns an AutoCompleter that completes @workspace paths
// and delegates /slash commands to ReadlinePrefixCompleter (no argument completion).
func NewReadlineSlashAtCompleter(workdir string) readline.AutoCompleter {
	return NewReadlineSlashAtCompleterWithSlashContext(workdir, nil)
}

// NewReadlineSlashAtCompleterWithSlashContext is like NewReadlineSlashAtCompleter but completes
// slash command arguments when slashCtx returns a populated SlashContext (REPL wiring).
func NewReadlineSlashAtCompleterWithSlashContext(workdir string, slashCtx func() SlashContext) readline.AutoCompleter {
	return &inputprefix.ReadlineAtCompletions{
		Workdir: workdir,
		Slash: &readlineSlashLineCompleter{
			prefix:   ReadlinePrefixCompleter(),
			slashCtx: slashCtx,
		},
	}
}
