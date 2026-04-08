package planfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPath(t *testing.T) {
	wd := "/tmp/proj"
	got := Path(wd)
	want := filepath.Join(wd, Subdir, DefaultFilename)
	if got != want {
		t.Fatalf("Path: got %q want %q", got, want)
	}
}

func TestReadAndHandoff(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "p.md")
	body := "# Plan\n\n1. Do thing.\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Read(p)
	if err != nil || got != body {
		t.Fatalf("Read: err=%v got=%q", err, got)
	}
	h := HandoffUserMessage(p, body)
	if !strings.Contains(h, "step by step") || !strings.Contains(h, body) {
		t.Fatalf("handoff missing expected text: %q", h)
	}
}

func TestReadEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(p); err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestInitIdempotent(t *testing.T) {
	dir := t.TempDir()
	created, err := Init(dir)
	if err != nil || !created {
		t.Fatalf("first Init: created=%v err=%v", created, err)
	}
	created2, err := Init(dir)
	if err != nil || created2 {
		t.Fatalf("second Init: created=%v err=%v", created2, err)
	}
	b, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "## Steps") {
		t.Fatalf("template missing Steps: %s", b)
	}
}

func TestResolvePlanArg(t *testing.T) {
	wd := t.TempDir()
	if got := ResolvePlanArg(wd, ""); got != Path(wd) {
		t.Fatalf("empty arg: %q", got)
	}
	rel := ResolvePlanArg(wd, "notes/plan.md")
	wantSuffix := filepath.Join("notes", "plan.md")
	if !strings.HasSuffix(rel, wantSuffix) {
		t.Fatalf("relative join: %q want suffix %q", rel, wantSuffix)
	}
}
