package main

import (
	"log/slog"
	"os"

	"github.com/okuzpe/goclaw/internal/app"
	"github.com/okuzpe/goclaw/internal/cli"
	"github.com/okuzpe/goclaw/internal/loglevel"
	"github.com/spf13/cobra"
)

// Version is the build version (same ldflags pattern as cmd/goclaw):
//
//	go build -ldflags "-X main.Version=v1.2.3" ./cmd/localagent
var Version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: loglevel.FromEnv(),
	})))
	root := cli.NewRootCmd(Version,
		func(cmd *cobra.Command, args []string) error {
			return app.RunChat(cmd, Version, args, app.FullscreenChat{Version: Version})
		},
		func(cmd *cobra.Command, args []string) error {
			return app.RunPrompt(cmd, args)
		},
		app.RunListSessions,
		app.RunDoctor,
		&cli.TelegramCommands{
			Start:     app.RunTelegramStart,
			Bridge:    app.RunTelegramBridge,
			Configure: app.RunTelegramConfigure,
			UserAdd:   app.RunTelegramUserAdd,
		},
	)
	if err := root.Execute(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
