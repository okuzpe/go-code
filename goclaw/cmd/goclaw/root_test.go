package main

import (
	"testing"
)

func TestSessionsListSubcommandResolved(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"sessions", "list"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name() != "list" {
		t.Fatalf("expected list subcommand, got %q", cmd.Name())
	}
}

func TestSessionsListExecute(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"sessions", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
}
