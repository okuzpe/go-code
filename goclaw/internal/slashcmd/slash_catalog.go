package slashcmd

import "strings"

type slashSubcommandSpec struct {
	Name    string
	Summary string
}

var memorySubcommandSpecs = []slashSubcommandSpec{
	{Name: "list", Summary: "list memory entries"},
	{Name: "add", Summary: "add memory entry"},
	{Name: "delete", Summary: "delete by filename"},
}

var planSubcommandSpecs = []slashSubcommandSpec{
	{Name: "path", Summary: "print default plan file path"},
	{Name: "init", Summary: "create plan file from template"},
	{Name: "new", Summary: "create .goclaw/plans/<name>.md mini template"},
	{Name: "save", Summary: "save last assistant to plan (optional path)"},
	{Name: "run", Summary: "save then execute plan (one model turn)"},
	{Name: "apply", Summary: "same as run"},
	{Name: "template", Summary: "print plan template"},
	{Name: "review", Summary: "preview plan + approval + parsed steps"},
	{Name: "approve", Summary: "record plan approval for gate"},
	{Name: "revoke", Summary: "clear plan approval"},
	{Name: "steps", Summary: "list parsed ## Steps lines"},
}

func usageList(specs []slashSubcommandSpec) string {
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	return strings.Join(names, " | ")
}

func suggestSubcommands(specs []slashSubcommandSpec, lowPartial string) []SlashCommandSuggest {
	var out []SlashCommandSuggest
	for _, spec := range specs {
		if lowPartial != "" && !strings.HasPrefix(strings.ToLower(spec.Name), lowPartial) {
			continue
		}
		out = append(out, SlashCommandSuggest{Name: spec.Name, Summary: spec.Summary})
	}
	return out
}
