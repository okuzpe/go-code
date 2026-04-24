package slashcmd

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/inputprefix"
	"github.com/okuzpe/goclaw/internal/memory"
)

// SlashInlineSuggestions returns slash command or argument rows for the TUI picker at runeCol.
func SlashInlineSuggestions(ctx context.Context, sc SlashContext, line string, runeCol int) []SlashCommandSuggest {
	_ = ctx
	parsed, ok := ParseSlashLineAtCursor(line, runeCol)
	if !ok {
		return nil
	}
	if parsed.FieldIndex == 0 {
		return TUISlashSuggestions(line)
	}
	return slashArgSuggestionsParsed(sc, parsed, line, runeCol)
}

// SlashArgTabExpand replaces the active argument token (field index >= 1). When ok is false, the caller
// should try SlashTabExpand for the command token or other handlers.
func SlashArgTabExpand(ctx context.Context, sc SlashContext, line string, runeCol int) (newLine string, newCursorRune int, ok bool) {
	_ = ctx
	parsed, okParse := ParseSlashLineAtCursor(line, runeCol)
	if !okParse || parsed.FieldIndex == 0 {
		return "", 0, false
	}
	sugs := slashArgSuggestionsParsed(sc, parsed, line, runeCol)
	if len(sugs) == 0 {
		return "", 0, false
	}
	runes := []rune(line)
	if runeCol > len(runes) {
		runeCol = len(runes)
	}
	if runeCol < parsed.ReplaceStartRune {
		runeCol = parsed.ReplaceStartRune
	}
	endStem := runeCol
	if endStem > parsed.ReplaceEndRune {
		endStem = parsed.ReplaceEndRune
	}
	stem := string(runes[parsed.ReplaceStartRune:endStem])

	names := make([]string, 0, len(sugs))
	for _, s := range sugs {
		names = append(names, s.Name)
	}
	lowStem := strings.ToLower(stem)
	var matches []string
	for _, n := range names {
		if strings.HasPrefix(strings.ToLower(n), lowStem) {
			matches = append(matches, n)
		}
	}
	if len(matches) == 0 {
		return "", 0, false
	}
	lcp := longestCommonPrefix(matches)
	if len(lcp) > len(stem) {
		out := replaceRunes(runes, parsed.ReplaceStartRune, parsed.ReplaceEndRune, lcp)
		return out, parsed.ReplaceStartRune + utf8.RuneCountInString(lcp), true
	}
	pick := matches[0]
	if strings.EqualFold(stem, pick) {
		if argAppendSpace(parsed.Cmd, parsed.FieldIndex, pick) {
			out := replaceRunes(runes, parsed.ReplaceStartRune, parsed.ReplaceEndRune, pick+" ")
			return out, parsed.ReplaceStartRune + utf8.RuneCountInString(pick) + 1, true
		}
		return "", 0, false
	}
	out := replaceRunes(runes, parsed.ReplaceStartRune, parsed.ReplaceEndRune, pick)
	newCol := parsed.ReplaceStartRune + utf8.RuneCountInString(pick)
	if argAppendSpace(parsed.Cmd, parsed.FieldIndex, pick) {
		out = replaceRunes([]rune(out), newCol, newCol, " ")
		newCol++
	}
	return out, newCol, true
}

func replaceRunes(line []rune, start, end int, insert string) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start > len(line) {
		start = len(line)
	}
	if end > len(line) {
		end = len(line)
	}
	ins := []rune(insert)
	out := make([]rune, 0, len(line)+len(ins))
	out = append(out, line[:start]...)
	out = append(out, ins...)
	out = append(out, line[end:]...)
	return string(out)
}

func argAppendSpace(cmd string, field int, picked string) bool {
	switch cmd {
	case "memory":
		if field == 1 && (picked == "list" || picked == "add" || picked == "delete") {
			return true
		}
	case "plan":
		if field == 1 {
			return true
		}
	case "mode":
		if field == 1 && (picked == "build" || picked == "plan") {
			return true
		}
	case "profile", "agents", "resume", "model", "theme", "focus", "in", "export":
		if field == 1 {
			return true
		}
	}
	return false
}

func slashArgSuggestionsParsed(sc SlashContext, parsed ParsedSlashLine, line string, runeCol int) []SlashCommandSuggest {
	runes := []rune(line)
	if runeCol > len(runes) {
		runeCol = len(runes)
	}
	endAt := parsed.ReplaceEndRune
	if runeCol < endAt {
		endAt = runeCol
	}
	partial := ""
	if parsed.ReplaceStartRune <= endAt {
		partial = string(runes[parsed.ReplaceStartRune:endAt])
	}
	lowPartial := strings.ToLower(partial)

	switch parsed.Cmd {
	case "mode":
		if parsed.FieldIndex != 1 {
			return nil
		}
		var out []SlashCommandSuggest
		for _, mode := range agents.PublicModeNames() {
			if lowPartial != "" && !strings.HasPrefix(strings.ToLower(mode), lowPartial) {
				continue
			}
			summary := "primary mode"
			if mode == "plan" {
				summary = "read-only planning mode"
			} else {
				summary = "direct coding mode"
			}
			out = append(out, SlashCommandSuggest{Name: mode, Summary: summary})
		}
		return out

	case "profile", "agents":
		if parsed.FieldIndex != 1 || sc.Orch == nil {
			return nil
		}
		profs, _ := agents.AllWithCustom(sc.UserAgentsDir, sc.ProjectAgentsDir)
		keys := agents.UserFacingSortedKeys(profs)
		var out []SlashCommandSuggest
		for _, k := range keys {
			if lowPartial != "" && !strings.HasPrefix(strings.ToLower(k), lowPartial) {
				continue
			}
			pr := profs[k]
			name := agents.DisplayProfileName(k)
			sum := name + " profile"
			if strings.TrimSpace(pr.Description) != "" {
				sum = pr.Description
			}
			out = append(out, SlashCommandSuggest{Name: name, Summary: sum})
		}
		return out

	case "resume":
		if parsed.FieldIndex != 1 || sc.Store == nil {
			return nil
		}
		entries, err := sc.Store.ListSessionEntries()
		if err != nil {
			return nil
		}
		var out []SlashCommandSuggest
		for _, e := range entries {
			if lowPartial != "" && !strings.HasPrefix(strings.ToLower(e.ID), lowPartial) {
				continue
			}
			out = append(out, SlashCommandSuggest{Name: e.ID, Summary: "saved session"})
		}
		return out

	case "memory":
		switch parsed.FieldIndex {
		case 1:
			return suggestSubcommands(memorySubcommandSpecs, lowPartial)
		case 2:
			fields := strings.Fields(line)
			if len(fields) < 2 || !strings.EqualFold(strings.TrimPrefix(fields[0], "/"), "memory") {
				return nil
			}
			sub := strings.ToLower(fields[1])
			if sub == "delete" {
				if sc.Mem == nil {
					return nil
				}
				list, err := sc.Mem.List()
				if err != nil {
					return nil
				}
				var out []SlashCommandSuggest
				for _, e := range list {
					base := e.Filename
					if lowPartial != "" && !strings.HasPrefix(strings.ToLower(base), lowPartial) {
						continue
					}
					out = append(out, SlashCommandSuggest{Name: base, Summary: fmt.Sprintf("%s — %s", e.Type, e.Name)})
				}
				return out
			}
			if sub == "add" {
				var out []SlashCommandSuggest
				for _, typ := range []memory.Type{
					memory.TypeUser, memory.TypeFeedback, memory.TypeProject, memory.TypeReference,
				} {
					t := string(typ)
					if lowPartial != "" && !strings.HasPrefix(strings.ToLower(t), lowPartial) {
						continue
					}
					out = append(out, SlashCommandSuggest{Name: t, Summary: "memory type"})
				}
				return out
			}
			return nil
		default:
			return nil
		}

	case "plan":
		if parsed.FieldIndex != 1 || strings.TrimSpace(sc.Workdir) == "" {
			return nil
		}
		return suggestSubcommands(planSubcommandSpecs, lowPartial)

	case "focus", "in":
		if parsed.FieldIndex != 1 || sc.Focus == nil {
			return nil
		}
		var out []SlashCommandSuggest
		for _, alias := range []string{"parent", "..", "coordinator"} {
			if lowPartial != "" && !strings.HasPrefix(strings.ToLower(alias), lowPartial) {
				continue
			}
			out = append(out, SlashCommandSuggest{Name: alias, Summary: "detach worker focus"})
		}
		for _, w := range coordinator.ListInteractiveWorkers() {
			id := w.TaskID
			if lowPartial != "" && !strings.HasPrefix(strings.ToLower(id), lowPartial) {
				continue
			}
			sum := w.Profile + " · " + w.Status
			if strings.TrimSpace(w.Summary) != "" {
				sum += " — " + w.Summary
			}
			out = append(out, SlashCommandSuggest{Name: id, Summary: sum})
		}
		return out

	case "theme":
		if parsed.FieldIndex != 1 || strings.TrimSpace(sc.UserConfigDir) == "" {
			return nil
		}
		var out []SlashCommandSuggest
		add := func(name, sum string) {
			if lowPartial != "" && !strings.HasPrefix(strings.ToLower(name), lowPartial) {
				return
			}
			out = append(out, SlashCommandSuggest{Name: name, Summary: sum})
		}
		add(config.UIAppearanceAuto, "follow terminal palette")
		for _, c := range config.UIAppearanceChoices {
			add(c, "TUI appearance preset")
		}
		return out

	case "export":
		if parsed.FieldIndex != 1 || strings.TrimSpace(sc.Workdir) == "" {
			return nil
		}
		atSugs := inputprefix.TUIAtPathSuggestions(sc.Workdir, "@"+partial)
		var out []SlashCommandSuggest
		for _, s := range atSugs {
			name := s.RelPath
			if s.IsDir {
				name += "/"
			}
			out = append(out, SlashCommandSuggest{Name: name, Summary: "relative to workspace"})
		}
		return out
	default:
		return nil
	}
}
