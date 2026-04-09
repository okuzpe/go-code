package slashcmd

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
	Workdir         string
	Profs           map[string]agents.Profile
	UserAgentsDir   string // for hot-reload of custom profiles on /profile
	ProjectAgentsDir string
	Doctor          func(ctx context.Context) (string, error)
}

// SlashContext carries dependencies for HandleSlash (memory, orchestrator, session pointer, disk store).
type SlashContext struct {
	SlashEnv
	Mem   *memory.Store
	Orch  *orchestrator.Orchestrator
	Sess  **session.Session
	Store *session.Store
}

// ErrReplQuit is returned by HandleSlash for /quit and /exit so the REPL can save and exit cleanly.
var ErrReplQuit = errors.New("repl quit")

// HandleSlash processes REPL slash commands. Returns handled=true if input was consumed.
// modelSubmit is non-empty when the caller should send that text to the model (e.g. /edit).
// quit with ErrReplQuit means the REPL should exit after printing out.
func HandleSlash(ctx context.Context, sc SlashContext, input string) (handled bool, out string, quit bool, modelSubmit string, err error) {
	mem := sc.Mem
	orch := sc.Orch
	sess := sc.Sess
	store := sc.Store
	env := sc.SlashEnv

	s := strings.TrimSpace(input)
	if s == "" {
		return false, "", false, "", nil
	}
	low := strings.ToLower(s)
	if low == "help" || low == "?" {
		return true, replHelpText(env, sess, orch), false, "", nil
	}
	if !strings.HasPrefix(s, "/") {
		return false, "", false, "", nil
	}

	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false, "", false, "", nil
	}
	cmd := strings.ToLower(strings.TrimPrefix(fields[0], "/"))

	switch cmd {
	case "help":
		return true, replHelpText(env, sess, orch), false, "", nil

	case "doctor":
		if env.Doctor == nil {
			return true, "", false, "", fmt.Errorf("/doctor: not available (missing wiring)")
		}
		out, derr := env.Doctor(ctx)
		if derr != nil {
			return true, "", false, "", derr
		}
		return true, out, false, "", nil

	case "quit", "exit":
		return true, "bye", true, "", ErrReplQuit

	case "sessions":
		if store == nil {
			return true, "", false, "", fmt.Errorf("/sessions: no session store configured")
		}
		ids, err := store.ListIDs()
		if err != nil {
			return true, "", false, "", err
		}
		if len(ids) == 0 {
			return true, "(no saved sessions on disk)", false, "", nil
		}
		return true, strings.Join(ids, "\n"), false, "", nil

	case "new":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/new requires a running agent (internal error)")
		}
		if sess == nil {
			return true, "", false, "", fmt.Errorf("/new: session pointer missing")
		}
		if store != nil && *sess != nil {
			if err := store.Save(*sess); err != nil {
				return true, "", false, "", fmt.Errorf("save current session before /new: %w (fix disk or permissions; session not reset)", err)
			}
		}
		next := session.New()
		*sess = next
		orch.ReplaceSession(next)
		return true, fmt.Sprintf("new empty session (previous transcript saved if a store is configured).\nnew session id: %s", next.ID), false, "", nil

	case "save":
		if store == nil {
			return true, "", false, "", fmt.Errorf("/save: no session store configured")
		}
		if sess == nil || *sess == nil {
			return true, "", false, "", fmt.Errorf("/save: no active session")
		}
		if err := store.Save(*sess); err != nil {
			return true, "", false, "", fmt.Errorf("save session: %w", err)
		}
		return true, fmt.Sprintf("(session saved: %s, %d messages)", (*sess).ID, (*sess).Len()), false, "", nil

	case "session":
		if sess == nil || *sess == nil {
			return true, "(no session)", false, "", nil
		}
		return true, fmt.Sprintf("session id: %s\nmessages: %d", (*sess).ID, (*sess).Len()), false, "", nil

	case "compact":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/compact requires a running agent")
		}
		orch.ForceCompact()
		return true, "(compaction applied: older turns summarized; tail preserved)", false, "", nil

	case "edit":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/edit requires a running agent")
		}
		body, eerr := openPromptEditor(ctx, env.Workdir)
		if eerr != nil {
			return true, "", false, "", eerr
		}
		if body == "" {
			return true, "(no message from editor — nothing sent)", false, "", nil
		}
		return true, "(sending message from editor…)", false, body, nil

	case "memory":
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

	case "profile":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/profile requires a running agent")
		}
		if len(fields) < 2 {
			return true, "", false, "", fmt.Errorf("usage: /profile <name>\nnames: %s", agents.ProfileListHint())
		}
		// Hot-reload: re-scan agent dirs so newly added *.md files are visible without restart.
		profs, _ := agents.AllWithCustom(env.UserAgentsDir, env.ProjectAgentsDir)
		name := strings.ToLower(strings.TrimSpace(fields[1]))
		p, ok := profs[name]
		if !ok {
			return true, "", false, "", fmt.Errorf("unknown profile %q; valid: %s", name, sortedProfileNames(profs))
		}
		orch.SetProfile(p)
		return true, fmt.Sprintf("active profile: %s", p.Name), false, "", nil

	case "plan":
		wd := strings.TrimSpace(env.Workdir)
		if wd == "" {
			return true, "", false, "", fmt.Errorf("/plan: workspace directory not set")
		}
		if len(fields) < 2 {
			return true, "", false, "", fmt.Errorf(`usage: /plan path | /plan init | /plan template
path  — show default plan file path
init  — create .goclaw/plan.md from template if missing
template — print the template to the terminal`)
		}
		sub := strings.ToLower(fields[1])
		switch sub {
		case "path":
			return true, planfile.Path(wd), false, "", nil
		case "init":
			created, ierr := planfile.Init(wd)
			if ierr != nil {
				return true, "", false, "", ierr
			}
			if created {
				return true, fmt.Sprintf("created %s", planfile.Path(wd)), false, "", nil
			}
			return true, fmt.Sprintf("already exists: %s", planfile.Path(wd)), false, "", nil
		case "template":
			return true, planfile.Template(), false, "", nil
		default:
			return true, "", false, "", fmt.Errorf("unknown /plan %q — use path, init, or template", fields[1])
		}

	case "apply-plan":
		if orch == nil {
			return true, "", false, "", fmt.Errorf("/apply-plan requires a running agent")
		}
		wd := strings.TrimSpace(env.Workdir)
		if wd == "" {
			return true, "", false, "", fmt.Errorf("/apply-plan: workspace directory not set")
		}
		if env.Profs == nil {
			return true, "", false, "", fmt.Errorf("/apply-plan: profile map not configured")
		}
		gp, ok := env.Profs["general-purpose"]
		if !ok {
			return true, "", false, "", fmt.Errorf("/apply-plan: general-purpose profile missing")
		}
		arg := ""
		if len(fields) >= 1 && len(s) > len(fields[0]) {
			arg = strings.TrimSpace(s[len(fields[0]):])
		}
		p := planfile.ResolvePlanArg(wd, arg)
		body, rerr := planfile.Read(p)
		if rerr != nil {
			return true, "", false, "", rerr
		}
		orch.SetProfile(gp)
		msg := planfile.HandoffUserMessage(p, body)
		resp, runErr := orch.Run(ctx, msg)
		if runErr != nil {
			return true, "", false, "", runErr
		}
		var b strings.Builder
		b.WriteString("switched to profile general-purpose; loaded plan:\n")
		b.WriteString(p)
		b.WriteString("\n\n")
		b.WriteString(resp)
		return true, b.String(), false, "", nil

	default:
		return true, "", false, "", fmt.Errorf("unknown command /%s — try /help", cmd)
	}
}

// PopularSlashHint is printed once after the startup banner on readline REPL (claw-style flow).
func PopularSlashHint(workdir string) string {
	var b strings.Builder
	b.WriteString("Popular slash commands (not sent to the model):\n")
	b.WriteString("  /help   /doctor   /plan   /apply-plan   /memory   /compact   /profile   /quit\n")
	if strings.TrimSpace(workdir) != "" {
		b.WriteString("Plan: ")
		b.WriteString(planfile.Path(workdir))
		b.WriteByte('\n')
	}
	return b.String()
}

// PreChatHelpSummary is a longer reference (e.g. after /help topics).
func PreChatHelpSummary(workdir string) string {
	var b strings.Builder
	b.WriteString("Slash commands (after chat starts; not sent to the model):\n")
	b.WriteString("  /help, help, ? — full list with session id and profile\n")
	b.WriteString("  /plan path|init|template — workspace plan under .goclaw/plan.md\n")
	b.WriteString("  /apply-plan [path] — run one execution turn from the plan\n")
	b.WriteString("  /memory list | add | delete — durable memory under ~/.goclaw/memory/\n")
	b.WriteString("  /compact, /edit, /profile, /new, /save, /session, /sessions, /quit\n")
	b.WriteString("Flags: --tui, --no-tools, --session <id>, --profile <name>\n")
	if strings.TrimSpace(workdir) != "" {
		b.WriteString("Plan file: ")
		b.WriteString(planfile.Path(workdir))
		b.WriteByte('\n')
	}
	return b.String()
}

func replHelpText(env SlashEnv, sess **session.Session, orch *orchestrator.Orchestrator) string {
	id := "(none)"
	if sess != nil && *sess != nil {
		id = (*sess).ID
	}
	var b strings.Builder
	b.WriteString("Slash commands (not sent to the model):\n")
	b.WriteString("  /help, help, ?   — this text\n")
	b.WriteString("  /doctor          — health check (config, provider reachability, session paths)\n")
	b.WriteString("  /session         — show full session id and message count\n")
	b.WriteString("  /sessions        — list saved session ids (same as --list-sessions, without restart)\n")
	b.WriteString("  /quit, /exit     — save session and exit (same shutdown path as Ctrl+C)\n")
	b.WriteString("  /new             — save current session to disk, start a fresh empty session\n")
	b.WriteString("  /save            — write current session JSONL without exiting\n")
	b.WriteString("  /compact         — force context compaction (keep recent tail)\n")
	b.WriteString("  /edit            — compose a multiline message in $EDITOR (vi, notepad, …)\n")
	b.WriteString("  /memory list     — list memory files under ~/.goclaw/memory/\n")
	b.WriteString("  /memory add <type> <name> <text...>  — types: user | feedback | project | reference\n")
	b.WriteString("  /memory delete <file.md> — remove one file (see list for basename)\n")
	b.WriteString("  /profile <name>  — switch agent profile (general-purpose, explore, plan, …)\n")
	b.WriteString("  /plan path|init|template — default plan path, create from template, or print template\n")
	b.WriteString("  /apply-plan [path] — load plan file and run with general-purpose profile\n")
	b.WriteString("  Ctrl+C           — exit (session is saved on shutdown)\n")
	b.WriteString("\nRestart CLI flags: --session <id>  --list-sessions  --no-tools  --tui  --profile <name>\n")
	b.WriteString("Env: GOCLAW_USE_TUI=1 enables fullscreen TUI; GOCLAW_USE_READLINE=1 forces readline if set.\n")
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

// sortedProfileNames returns a comma-separated sorted list of profile names for error messages.
func sortedProfileNames(profs map[string]agents.Profile) string {
	names := make([]string, 0, len(profs))
	for name := range profs {
		names = append(names, name)
	}
	// inline sort to avoid importing "sort" if it isn't already present
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return strings.Join(names, ", ")
}
