package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/okuzpe/goclaw/internal/app"
	"github.com/okuzpe/goclaw/internal/cli"
	"github.com/spf13/cobra"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel(),
	})))
	root := cli.NewRootCmd(Version,
		func(cmd *cobra.Command, args []string) error {
			return app.RunChat(cmd, Version, args, fullscreenChat{})
		},
		app.RunListSessions,
		app.RunDoctor,
	)
	if err := root.Execute(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func logLevel() slog.Level {
	switch strings.ToLower(os.Getenv("GOCLAW_LOG")) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
