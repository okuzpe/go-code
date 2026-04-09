package cli

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
)

func testRoot(t *testing.T) *cobra.Command {
	t.Helper()
	return NewRootCmd("dev",
		func(*cobra.Command, []string) error { return nil },
		func(*cobra.Command, []string) error { return nil },
		func() error { return nil },
		func(*cobra.Command, []string) error { return nil },
	)
}

func TestChatSubcommandResolved(t *testing.T) {
	root := testRoot(t)
	cmd, _, err := root.Find([]string{"chat"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name() != "chat" {
		t.Fatalf("expected chat subcommand, got %q", cmd.Name())
	}
}

func TestSessionsListSubcommandResolved(t *testing.T) {
	root := testRoot(t)
	cmd, _, err := root.Find([]string{"sessions", "list"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name() != "list" {
		t.Fatalf("expected list subcommand, got %q", cmd.Name())
	}
}

func TestSessionsListExecute(t *testing.T) {
	root := NewRootCmd("dev",
		func(*cobra.Command, []string) error { return nil },
		func(*cobra.Command, []string) error { return nil },
		func() error { return nil },
		func(*cobra.Command, []string) error { return nil },
	)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"sessions", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestDoctorSubcommandResolved(t *testing.T) {
	root := testRoot(t)
	cmd, _, err := root.Find([]string{"doctor"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name() != "doctor" {
		t.Fatalf("expected doctor subcommand, got %q", cmd.Name())
	}
}

func TestPromptSubcommandResolved(t *testing.T) {
	root := testRoot(t)
	cmd, _, err := root.Find([]string{"prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name() != "prompt" {
		t.Fatalf("expected prompt subcommand, got %q", cmd.Name())
	}
}
