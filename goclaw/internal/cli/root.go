package cli

import (
	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/spf13/cobra"
)

// RunChatFunc starts the interactive REPL (same for default root and `chat` subcommand).
type RunChatFunc func(cmd *cobra.Command, args []string) error

// RunListSessionsFunc prints saved session ids and exits (see sessions list --long for TSV rows).
type RunListSessionsFunc func(cmd *cobra.Command) error

// RunDoctorFunc prints a preflight health check and exits.
type RunDoctorFunc func(cmd *cobra.Command, args []string) error

// RunPromptFunc runs one agent turn from argv text (`goclaw prompt ...`).
type RunPromptFunc func(cmd *cobra.Command, args []string) error

// TelegramSubcommandFunc runs one `goclaw telegram …` handler (injected from cmd/goclaw to avoid an app↔cli import cycle in tests).
type TelegramSubcommandFunc func(cmd *cobra.Command, args []string) error

// TelegramCommands wires optional telegram subcommands. Nil receiver or all nil fields omits the `telegram` command group.
type TelegramCommands struct {
	Start     TelegramSubcommandFunc // guided setup (TTY) then bridge; used by make telegram
	Bridge    TelegramSubcommandFunc // strict: exit if settings incomplete
	Configure TelegramSubcommandFunc // merge settings.local.json only
	UserAdd   TelegramSubcommandFunc // append resolved ids to telegram_allowed_user_ids in ~/.goclaw/settings.local.json
}

func telegramWired(tg *TelegramCommands) bool {
	return tg != nil && (tg.Start != nil || tg.Bridge != nil || tg.Configure != nil || tg.UserAdd != nil)
}

// NewRootCmd builds the Cobra command tree. runChat and listSessions are injected so tests
// do not link the full UI stack.
func NewRootCmd(version string, runChat RunChatFunc, runPrompt RunPromptFunc, listSessions RunListSessionsFunc, runDoctor RunDoctorFunc, tg *TelegramCommands) *cobra.Command {
	root := &cobra.Command{
		Use:     "goclaw",
		Short:   "Go CLI coding agent — coordinator hub by default; workers run tools (local Ollama)",
		Long:    "Local-first coding agent: default profile is coordinator (hub); delegate work with spawn_agent to workers that use file and shell tools. Sessions and REPL slash commands. Run with no arguments to start the chat REPL. Override with --profile or agent_profile in settings.",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			listSessionsFlag, err := cmd.Flags().GetBool("list-sessions")
			if err != nil {
				return err
			}
			if listSessionsFlag {
				return listSessions(cmd)
			}
			return runChat(cmd, args)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().String("profile", "", "agent profile ("+agents.ProfileListHint()+")")
	root.PersistentFlags().String("session", "", "resume an existing session id (JSONL in ~/.goclaw/sessions)")
	root.PersistentFlags().Bool("list-sessions", false, "list saved session ids and exit")
	root.PersistentFlags().Bool("no-tools", false, "run without registering tools (chat-only; also GOCLAW_DISABLE_TOOLS=1)")
	root.PersistentFlags().Bool("tui", false, "fullscreen Bubble Tea TUI (default on a TTY; redundant with GOCLAW_USE_TUI=1)")
	root.PersistentFlags().Bool("mock", false, "stream a canned assistant reply without calling the model (UI demo; TUI and JSON stdin modes)")
	root.PersistentFlags().String("output-format", "text", `stdout for one-shot modes: "text" (final assistant only) or "json" (object with response and toolCalls); use with stdin automation or goclaw prompt`)
	root.PersistentFlags().Bool("json-output", false, `shorthand for --output-format json with stdin automation (echo "hi" | goclaw --json-output); same JSON shape as --output-format json`)
	root.PersistentFlags().StringSlice("plugin-dir", nil, `plugin root directories (each contains goclaw-plugin.json); repeat flag or comma-separated; merges with settings "plugin_dirs"`)
	root.PersistentFlags().String("task-model-router", "", `per-turn model selection: "off", "rules" (heuristics), or "llm" (extra classifier call); requires task_models in settings`)
	root.PersistentFlags().String("workspace", "", `default project directory for prompt/glob/grep defaults (absolute or relative to cwd); does not block absolute paths in file tools; overrides tool_workspace_root and GOCLAW_TOOL_WORKSPACE`)

	sessionsCmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage saved conversation sessions on disk",
	}
	sessionsListCmd := &cobra.Command{
		Use:   "list",
		Short: "Print saved session ids (default: one id per line; use --long for mod time + preview TSV)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listSessions(cmd)
		},
	}
	sessionsListCmd.Flags().Bool("long", false, "tab-separated rows: id, RFC3339 mod time (UTC), first user message preview (stable for scripts)")
	sessionsCmd.AddCommand(sessionsListCmd)
	root.AddCommand(sessionsCmd)
	root.AddCommand(newDoctorCmd(runDoctor))
	root.AddCommand(newChatCmd(runChat))
	root.AddCommand(newPromptCmd(runPrompt))
	if telegramWired(tg) {
		root.AddCommand(newTelegramCmd(tg))
	}

	return root
}

func newTelegramCmd(tg *TelegramCommands) *cobra.Command {
	telegram := &cobra.Command{
		Use:   "telegram",
		Short: "Optional Telegram Bot API bridge (allowlisted users only)",
	}
	if tg.Configure != nil {
		configure := &cobra.Command{
			Use:   "configure",
			Short: "Interactive wizard: merge Telegram bot token and allowlist into ~/.goclaw/settings.local.json",
			RunE:  tg.Configure,
		}
		telegram.AddCommand(configure)
	}
	if tg.Start != nil {
		start := &cobra.Command{
			Use:   "start",
			Short: "Run the Telegram bridge (prompts for missing settings in a TTY, then long-polls)",
			Long: `Same bridge as telegram bridge after settings exist. When bot token or telegram_allowed_user_ids
is missing, runs an interactive prompt (stdin/stdout must be TTYs) and merges into ~/.goclaw/settings.local.json.

For scripts or CI, use telegram bridge with env vars or a pre-filled settings.local.json.`,
			RunE: tg.Start,
		}
		telegram.AddCommand(start)
	}
	if tg.Bridge != nil {
		bridge := &cobra.Command{
			Use:   "bridge",
			Short: "Long-poll Telegram and run one agent turn per allowlisted text message",
			Long: `Runs until interrupted. Requires telegram_bot_token (or telegram_bot_token_file / GOCLAW_TELEGRAM_BOT_TOKEN)
and a non-empty telegram_allowed_user_ids list (or GOCLAW_TELEGRAM_ALLOWED_USER_IDS) in settings — prefer ~/.goclaw/settings.local.json.

Tool runs that would prompt in Ask mode fail in this path; set tool_permissions to "allow" for tools you need, use a read-only profile, or --no-tools.`,
			RunE: tg.Bridge,
		}
		telegram.AddCommand(bridge)
	}
	if tg.UserAdd != nil {
		user := &cobra.Command{
			Use:   "user",
			Short: "Manage allowlisted Telegram user ids (stored in ~/.goclaw/settings.local.json)",
		}
		add := &cobra.Command{
			Use:   "add [entries...]",
			Short: "Append resolved user id(s) to telegram_allowed_user_ids (deduplicated)",
			Long: `Each entry is a numeric Telegram user id (optional leading #), or @PublicUsername (resolved via Bot API using your bot token).

Existing ids in ~/.goclaw/settings.local.json are kept; new ones are merged and the list is sorted.

Examples:
  goclaw telegram user add 123456789
  goclaw telegram user add @alice,987654321

Requires telegram_bot_token in settings (or GOCLAW_TELEGRAM_BOT_TOKEN) for @username resolution.`,
			Args: cobra.MinimumNArgs(1),
			RunE: tg.UserAdd,
		}
		user.AddCommand(add)
		telegram.AddCommand(user)
	}
	return telegram
}

func newDoctorCmd(runDoctor RunDoctorFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Preflight health check: Bubble Tea pager on a TTY, plain text when stdout is not a terminal",
		RunE:  runDoctor,
	}
}

func newChatCmd(runChat RunChatFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "chat",
		Short: "Start interactive chat (fullscreen Bubble Tea TUI on a TTY)",
		Long: `Opens the coding-agent chat: streaming assistant, tool loop, slash commands, and session persistence.
Default agent profile is coordinator (hub); use spawn_agent in-session or switch profile with /profile.

On an interactive terminal the UI is fullscreen Bubble Tea (Bubbles textarea + transcript). GOCLAW_USE_TUI=0 on a TTY is unsupported — use a real terminal or --output-format json for pipes.
Use --mock to stream a canned reply without calling the model (UI / wiring check).
Use --output-format json or --json-output to read one stdin line and print JSON (automation; incompatible with explicit --tui).
Use goclaw prompt "message" for a one-shot turn without piping stdin.`,
		RunE: runChat,
	}
}

func newPromptCmd(runPrompt RunPromptFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "prompt [message]...",
		Short: "Run one agent turn from argument text and exit",
		Long: `Joins all arguments into a single user message and runs the same one-turn loop as the REPL (no interactive session).

Default prints the final assistant text to stdout. Use --output-format json (or --json-output) for machine-readable JSON.
Non-interactive tool runs need tool_permissions "allow" for tools that would otherwise prompt (or use --no-tools).

Incompatible with --tui; use the chat subcommand for fullscreen UI.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrompt(cmd, args)
		},
	}
}
