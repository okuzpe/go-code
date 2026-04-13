package cli

import (
	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/spf13/cobra"
)

// RunChatFunc starts the interactive REPL (same for default root and `chat` subcommand).
type RunChatFunc func(cmd *cobra.Command, args []string) error

// RunListSessionsFunc prints saved session ids and exits.
type RunListSessionsFunc func() error

// RunDoctorFunc prints a preflight health check and exits.
type RunDoctorFunc func(cmd *cobra.Command, args []string) error

// RunPromptFunc runs one agent turn from argv text (`goclaw prompt ...`).
type RunPromptFunc func(cmd *cobra.Command, args []string) error

// NewRootCmd builds the Cobra command tree. runChat and listSessions are injected so tests
// do not link the full UI stack.
func NewRootCmd(version string, runChat RunChatFunc, runPrompt RunPromptFunc, listSessions RunListSessionsFunc, runDoctor RunDoctorFunc) *cobra.Command {
	root := &cobra.Command{
		Use:     "goclaw",
		Short:   "Go CLI coding agent (Ollama or Anthropic)",
		Long:    "Local-first coding agent with tools, sessions, and REPL slash commands. Run with no arguments to start the chat REPL.",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			listSessionsFlag, err := cmd.Flags().GetBool("list-sessions")
			if err != nil {
				return err
			}
			if listSessionsFlag {
				return listSessions()
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
	root.PersistentFlags().Bool("readline", false, "force line-at-a-time readline REPL (disables default fullscreen TUI)")
	root.PersistentFlags().Bool("tui", false, "fullscreen Bubble Tea TUI (default on a TTY; redundant with GOCLAW_USE_TUI=1)")
	root.PersistentFlags().Bool("mock", false, "stream a canned assistant reply without calling the model (UI demo; TUI and readline)")
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
		Short: "Print saved session ids (same as -list-sessions)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listSessions()
		},
	}
	sessionsCmd.AddCommand(sessionsListCmd)
	root.AddCommand(sessionsCmd)
	root.AddCommand(newDoctorCmd(runDoctor))
	root.AddCommand(newChatCmd(runChat))
	root.AddCommand(newPromptCmd(runPrompt))

	return root
}

func newDoctorCmd(runDoctor RunDoctorFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Print a short preflight health check and exit",
		RunE:  runDoctor,
	}
}

func newChatCmd(runChat RunChatFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "chat",
		Short: "Start interactive chat (default fullscreen TUI on a TTY; use --readline for line REPL)",
		Long: `Opens the coding-agent chat: streaming assistant, tool loop, slash commands, and session persistence.

On an interactive terminal the default UI is fullscreen Bubble Tea (better tool approval and transcript).
Use --readline or GOCLAW_USE_READLINE=1 for claw-style line REPL, or GOCLAW_USE_TUI=0 to opt out of TUI.
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
