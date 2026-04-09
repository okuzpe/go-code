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

// NewRootCmd builds the Cobra command tree. runChat and listSessions are injected so tests
// do not link the full UI stack.
func NewRootCmd(version string, runChat RunChatFunc, listSessions RunListSessionsFunc, runDoctor RunDoctorFunc) *cobra.Command {
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
	root.PersistentFlags().Bool("readline", false, "force readline REPL; disables TUI even if GOCLAW_USE_TUI=1")
	root.PersistentFlags().Bool("tui", false, "use fullscreen Bubble Tea TUI instead of readline (also GOCLAW_USE_TUI=1)")
	root.PersistentFlags().Bool("mock", false, "stream a canned assistant reply without calling the model (UI demo; TUI and readline)")

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
		Short: "Start interactive chat (fullscreen TUI or readline with --readline)",
		Long: `Opens the coding-agent chat: streaming assistant, tool loop, slash commands, and session persistence.

Default is readline (claw-style). Use --tui or GOCLAW_USE_TUI=1 for fullscreen TUI.
Use --mock to stream a canned reply without calling the model (UI / wiring check).`,
		RunE: runChat,
	}
}
