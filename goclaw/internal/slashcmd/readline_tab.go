package slashcmd

import "github.com/chzyer/readline"

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
