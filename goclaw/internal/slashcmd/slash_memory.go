package slashcmd

import (
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/memory"
)

func handleSlashMemory(mem *memory.Store, fields []string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	if len(fields) < 2 {
		return true, "", false, "", fmt.Errorf(`usage: /memory list | /memory add <user|feedback|project|reference> <name> <words...>
example: /memory add project style Prefer tabs over spaces for Go imports`)
	}
	sub := strings.ToLower(fields[1])
	switch sub {
	case "list":
		list, lerr := mem.List()
		if lerr != nil {
			return true, "", false, "", lerr
		}
		if len(list) == 0 {
			return true, "(no memory entries)", false, "", nil
		}
		setTUIDocOverlay(hintsOut, "Memory")
		var b strings.Builder
		b.WriteString("## Memory entries\n\n")
		for _, e := range list {
			b.WriteString("- ")
			b.WriteString(e.Filename)
			b.WriteString(" [")
			b.WriteString(string(e.Type))
			b.WriteString("] ")
			b.WriteString(e.Name)
			if e.Description != "" {
				b.WriteString(" — ")
				b.WriteString(e.Description)
			}
			if preview := previewRunes(e.Body, 80); preview != "" {
				b.WriteString(" | ")
				b.WriteString(preview)
			}
			b.WriteByte('\n')
		}
		return true, strings.TrimSuffix(b.String(), "\n"), false, "", nil

	case "add":
		if len(fields) < 5 {
			return true, "", false, "", fmt.Errorf(`usage: /memory add <user|feedback|project|reference> <one-word-name> <text...>
example: /memory add user prefs Use British spelling in docs`)
		}
		typ := memory.Type(fields[2])
		switch typ {
		case memory.TypeUser, memory.TypeFeedback, memory.TypeProject, memory.TypeReference:
		default:
			return true, "", false, "", fmt.Errorf("invalid type %q — use user, feedback, project, or reference", fields[2])
		}
		name := fields[3]
		body := strings.Join(fields[4:], " ")
		if body == "" {
			return true, "", false, "", fmt.Errorf("memory text cannot be empty (add words after the name)")
		}
		desc := body
		if len(desc) > 160 {
			desc = desc[:160] + "…"
		}
		base, serr := mem.Save(memory.Entry{
			Type:        typ,
			Name:        name,
			Description: desc,
			Body:        body,
		})
		if serr != nil {
			return true, "", false, "", serr
		}
		return true, fmt.Sprintf("saved memory entry %q (%s)", base, typ), false, "", nil

	case "delete":
		if len(fields) < 3 {
			return true, "", false, "", fmt.Errorf(`usage: /memory delete <filename.md>
use /memory list to see basenames (e.g. mynote_a1b2c3d4.md)`)
		}
		base := fields[2]
		if err := mem.Delete(base); err != nil {
			return true, "", false, "", err
		}
		return true, fmt.Sprintf("deleted memory file %q", base), false, "", nil

	default:
		return true, "", false, "", fmt.Errorf("unknown /memory %q — use list, add, or delete", fields[1])
	}
}
