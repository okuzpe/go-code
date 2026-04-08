package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobBasenamePattern(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o600)
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0o700)
	_ = os.WriteFile(filepath.Join(dir, "sub", "b.go"), []byte("y"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "sub", "c.txt"), []byte("z"), 0o600)

	g := NewGlob(dir)
	res, err := g.Execute(context.Background(), `{"pattern":"*.go"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal(res.Content)
	}
	lines := strings.Split(strings.TrimSpace(res.Content), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 matches, got %q", res.Content)
	}
}

func TestGlobPathAware(t *testing.T) {
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, "pkg"), 0o700)
	_ = os.WriteFile(filepath.Join(dir, "pkg", "x.go"), []byte("1"), 0o600)

	g := NewGlob(dir)
	res, err := g.Execute(context.Background(), `{"pattern":"pkg/*.go"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "pkg") || !strings.Contains(res.Content, "x.go") {
		t.Fatalf("got %q", res.Content)
	}
}
