package slashcmd

import "github.com/chzyer/readline"

// readlineSlashPrefixes are top-level REPL slash commands for readline Tab completion.
// Keep sorted; add an entry when introducing a new /root command in HandleSlash.
var readlineSlashPrefixes = []string{
	"/apply-plan",
	"/compact",
	"/doctor",
	"/edit",
	"/exit",
	"/help",
	"/memory",
	"/new",
	"/plan",
	"/profile",
	"/quit",
	"/save",
	"/session",
	"/sessions",
}

// ReadlineSlashPrefixes returns a copy of slash prefixes shown in Tab completion.
func ReadlineSlashPrefixes() []string {
	out := make([]string, len(readlineSlashPrefixes))
	copy(out, readlineSlashPrefixes)
	return out
}

// ReadlinePrefixCompleter builds a readline.AutoCompleter for top-level /commands.
func ReadlinePrefixCompleter() *readline.PrefixCompleter {
	items := make([]readline.PrefixCompleterInterface, 0, len(readlineSlashPrefixes))
	for _, p := range readlineSlashPrefixes {
		items = append(items, readline.PcItem(p))
	}
	return readline.NewPrefixCompleter(items...)
}
