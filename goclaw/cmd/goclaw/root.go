package main

import (
	"github.com/spf13/cobra"
)

// newRootCmd builds the Cobra command tree. Call from main (never init).
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "goclaw",
		Short: "Go CLI coding agent (Ollama or Anthropic)",
		Long:  "Local-first coding agent with tools, sessions, and REPL slash commands. Run with no arguments to start the chat REPL.",
		RunE: func(cmd *cobra.Command, args []string) error {
			listSessions, err := cmd.Flags().GetBool("list-sessions")
			if err != nil {
				return err
			}
			if listSessions {
				return runListSessions()
			}
			return runChat(cmd, args)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().String("profile", "", "agent profile (general-purpose, explore, plan, verification, guide, statusline)")
	root.PersistentFlags().String("session", "", "resume an existing session id (JSONL in ~/.goclaw/sessions)")
	root.PersistentFlags().Bool("list-sessions", false, "list saved session ids and exit")
	root.PersistentFlags().Bool("no-tools", false, "run without registering tools (chat-only; also GOCLAW_DISABLE_TOOLS=1)")

	sessionsCmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage saved conversation sessions on disk",
	}
	sessionsListCmd := &cobra.Command{
		Use:   "list",
		Short: "Print saved session ids (same as -list-sessions)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListSessions()
		},
	}
	sessionsCmd.AddCommand(sessionsListCmd)
	root.AddCommand(sessionsCmd)

	return root
}
