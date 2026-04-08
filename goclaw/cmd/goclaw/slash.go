package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/planfile"
	"github.com/okuzpe/goclaw/internal/session"
)

// SlashEnv carries workspace paths and profile lookup for slash commands.
type SlashEnv struct {
	Workdir string
	Profs   map[string]agents.Profile
}

// ErrReplQuit is returned by handleSlash for /quit and /exit so main can save and exit cleanly.
var ErrReplQuit = errors.New("repl quit")

// handleSlash processes REPL slash commands. Returns handled=true if input was consumed.
// sess must point to the current session pointer in main; store is used for /new and /save.
func handleSlash(ctx context.Context, env SlashEnv, input string, mem *memory.Store, orch *orchestrator.Orchestrator, sess **session.Session, store *session.Store) (handled bool, out string, err error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return false, "", nil
	}
	low := strings.ToLower(s)
	if low == "help" || low == "?" {
		return true, replHelpText(env, sess, orch), nil
	}
	if !strings.HasPrefix(s, "/") {
		return false, "", nil
	}

	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false, "", nil
	}
	cmd := strings.ToLower(strings.TrimPrefix(fields[0], "/"))

	switch cmd {
	case "help":
		return true, replHelpText(env, sess, orch), nil

	case "quit", "exit":
		return true, "bye", ErrReplQuit

	case "sessions":
		if store == nil {
			return true, "", fmt.Errorf("/sessions: no session store configured")
		}
		ids, err := store.ListIDs()
		if err != nil {
			return true, "", err
		}
		if len(ids) == 0 {
			return true, "(no saved sessions on disk)", nil
		}
		return true, strings.Join(ids, "\n"), nil

	case "new":
		if orch == nil {
			return true, "", fmt.Errorf("/new requires a running agent (internal error)")
		}
		if sess == nil {
			return true, "", fmt.Errorf("/new: session pointer missing")
		}
		if store != nil && *sess != nil {
			if err := store.Save(*sess); err != nil {
				return true, "", fmt.Errorf("save current session before /new: %w (fix disk or permissions; session not reset)", err)
			}
		}
		next := session.New()
		*sess = next
		orch.ReplaceSession(next)
		return true, fmt.Sprintf("new empty session (previous transcript saved if a store is configured).\nnew session id: %s", next.ID), nil

	case "save":
		if store == nil {
			return true, "", fmt.Errorf("/save: no session store configured")
		}
		if sess == nil || *sess == nil {
			return true, "", fmt.Errorf("/save: no active session")
		}
		if err := store.Save(*sess); err != nil {
			return true, "", fmt.Errorf("save session: %w", err)
		}
		return true, fmt.Sprintf("(session saved: %s, %d messages)", (*sess).ID, (*sess).Len()), nil

	case "session":
		if sess == nil || *sess == nil {
			return true, "(no session)", nil
		}
		return true, fmt.Sprintf("session id: %s\nmessages: %d", (*sess).ID, (*sess).Len()), nil

	case "compact":
		if orch == nil {
			return true, "", fmt.Errorf("/compact requires a running agent")
		}
		orch.ForceCompact()
		return true, "(compaction applied: older turns summarized; tail preserved)", nil

	case "memory":
		if len(fields) < 2 {
			return true, "", fmt.Errorf(`usage: /memory list | /memory add <user|feedback|project|reference> <name> <words...>
example: /memory add project style Prefer tabs over spaces for Go imports`)
		}
		sub := strings.ToLower(fields[1])
		switch sub {
		case "list":
			list, lerr := mem.List()
			if lerr != nil {
				return true, "", lerr
			}
			if len(list) == 0 {
				return true, "(no memory entries)", nil
			}
			var b strings.Builder
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
				b.WriteByte('\n')
			}
			return true, strings.TrimSuffix(b.String(), "\n"), nil

		case "add":
			if len(fields) < 5 {
				return true, "", fmt.Errorf(`usage: /memory add <user|feedback|project|reference> <one-word-name> <text...>
example: /memory add user prefs Use British spelling in docs`)
			}
			typ := memory.Type(fields[2])
			switch typ {
			case memory.TypeUser, memory.TypeFeedback, memory.TypeProject, memory.TypeReference:
			default:
				return true, "", fmt.Errorf("invalid type %q — use user, feedback, project, or reference", fields[2])
			}
			name := fields[3]
			body := strings.Join(fields[4:], " ")
			if body == "" {
				return true, "", fmt.Errorf("memory text cannot be empty (add words after the name)")
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
				return true, "", serr
			}
			return true, fmt.Sprintf("saved memory entry %q (%s)", base, typ), nil

		case "delete":
			if len(fields) < 3 {
				return true, "", fmt.Errorf(`usage: /memory delete <filename.md>
use /memory list to see basenames (e.g. mynote_a1b2c3d4.md)`)
			}
			base := fields[2]
			if err := mem.Delete(base); err != nil {
				return true, "", err
			}
			return true, fmt.Sprintf("deleted memory file %q", base), nil

		default:
			return true, "", fmt.Errorf("unknown /memory %q — use list, add, or delete", fields[1])
		}

	case "profile":
		if orch == nil {
			return true, "", fmt.Errorf("/profile requires a running agent")
		}
		if env.Profs == nil {
			return true, "", fmt.Errorf("/profile: profile map not configured")
		}
		if len(fields) < 2 {
			return true, "", fmt.Errorf(`usage: /profile <name>
names: general-purpose, explore, plan, verification, guide, statusline`)
		}
		name := strings.ToLower(strings.TrimSpace(fields[1]))
		p, ok := env.Profs[name]
		if !ok {
			return true, "", fmt.Errorf("unknown profile %q", name)
		}
		orch.SetProfile(p)
		return true, fmt.Sprintf("active profile: %s", p.Name), nil

	case "plan":
		wd := strings.TrimSpace(env.Workdir)
		if wd == "" {
			return true, "", fmt.Errorf("/plan: workspace directory not set")
		}
		if len(fields) < 2 {
			return true, "", fmt.Errorf(`usage: /plan path | /plan init | /plan template
path  — show default plan file path
init  — create .goclaw/plan.md from template if missing
template — print the template to the terminal`)
		}
		sub := strings.ToLower(fields[1])
		switch sub {
		case "path":
			return true, planfile.Path(wd), nil
		case "init":
			created, ierr := planfile.Init(wd)
			if ierr != nil {
				return true, "", ierr
			}
			if created {
				return true, fmt.Sprintf("created %s", planfile.Path(wd)), nil
			}
			return true, fmt.Sprintf("already exists: %s", planfile.Path(wd)), nil
		case "template":
			return true, planfile.Template(), nil
		default:
			return true, "", fmt.Errorf("unknown /plan %q — use path, init, or template", fields[1])
		}

	case "apply-plan":
		if orch == nil {
			return true, "", fmt.Errorf("/apply-plan requires a running agent")
		}
		wd := strings.TrimSpace(env.Workdir)
		if wd == "" {
			return true, "", fmt.Errorf("/apply-plan: workspace directory not set")
		}
		if env.Profs == nil {
			return true, "", fmt.Errorf("/apply-plan: profile map not configured")
		}
		gp, ok := env.Profs["general-purpose"]
		if !ok {
			return true, "", fmt.Errorf("/apply-plan: general-purpose profile missing")
		}
		arg := ""
		if len(fields) >= 1 && len(s) > len(fields[0]) {
			arg = strings.TrimSpace(s[len(fields[0]):])
		}
		p := planfile.ResolvePlanArg(wd, arg)
		body, rerr := planfile.Read(p)
		if rerr != nil {
			return true, "", rerr
		}
		orch.SetProfile(gp)
		msg := planfile.HandoffUserMessage(p, body)
		resp, runErr := orch.Run(ctx, msg)
		if runErr != nil {
			return true, "", runErr
		}
		var b strings.Builder
		b.WriteString("switched to profile general-purpose; loaded plan:\n")
		b.WriteString(p)
		b.WriteString("\n\n")
		b.WriteString(resp)
		return true, b.String(), nil

	default:
		return true, "", fmt.Errorf("unknown command /%s — try /help", cmd)
	}
}

func replHelpText(env SlashEnv, sess **session.Session, orch *orchestrator.Orchestrator) string {
	id := "(none)"
	if sess != nil && *sess != nil {
		id = (*sess).ID
	}
	var b strings.Builder
	b.WriteString("Slash commands (not sent to the model):\n")
	b.WriteString("  /help, help, ?   — this text\n")
	b.WriteString("  /session         — show full session id and message count\n")
	b.WriteString("  /sessions        — list saved session ids (same as --list-sessions, without restart)\n")
	b.WriteString("  /quit, /exit     — save session and exit (same shutdown path as Ctrl+C)\n")
	b.WriteString("  /new             — save current session to disk, start a fresh empty session\n")
	b.WriteString("  /save            — write current session JSONL without exiting\n")
	b.WriteString("  /compact         — force context compaction (keep recent tail)\n")
	b.WriteString("  /memory list     — list memory files under ~/.goclaw/memory/\n")
	b.WriteString("  /memory add <type> <name> <text...>  — types: user | feedback | project | reference\n")
	b.WriteString("  /memory delete <file.md> — remove one file (see list for basename)\n")
	b.WriteString("  /profile <name>  — switch agent profile (general-purpose, explore, plan, …)\n")
	b.WriteString("  /plan path|init|template — default plan path, create from template, or print template\n")
	b.WriteString("  /apply-plan [path] — load plan file and run with general-purpose profile\n")
	b.WriteString("  Ctrl+C           — exit (session is saved on shutdown)\n")
	b.WriteString("\nRestart CLI flags: --session <id>  --list-sessions  --no-tools  --profile <name>\n")
	b.WriteString("CLI subcommand: goclaw sessions list (same as --list-sessions)\n")
	b.WriteString("Current session id: ")
	b.WriteString(id)
	b.WriteByte('\n')
	if orch != nil {
		b.WriteString("Active profile (this session): ")
		b.WriteString(orch.ProfileName())
		b.WriteByte('\n')
	}
	if strings.TrimSpace(env.Workdir) != "" {
		b.WriteString("Workspace plan file: ")
		b.WriteString(planfile.Path(env.Workdir))
		b.WriteByte('\n')
	}
	return b.String()
}
