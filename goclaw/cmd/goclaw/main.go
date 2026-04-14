package main

import (
	"log/slog"
	"os"

	"github.com/okuzpe/goclaw/internal/app"
	"github.com/okuzpe/goclaw/internal/cli"
	"github.com/okuzpe/goclaw/internal/loglevel"
	"github.com/spf13/cobra"
)

func main() {
	// Add your initialization code here

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: loglevel.FromEnv(),
	})))
	root := cli.NewRootCmd(Version,
		func(cmd *cobra.Command, args []string) error {
			return app.RunChat(cmd, Version, args, fullscreenChat{})
		},
		func(cmd *cobra.Command, args []string) error {
			return app.RunPrompt(cmd, args)
		},
		app.RunListSessions,
		app.RunDoctor,
	)
	if err := root.Execute(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
