package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.go")
	_ = os.WriteFile(p, []byte("alpha\nbeta foo\n"), 0o600)

	g := NewGrep(dir)
	res, err := g.Execute(context.Background(), `{"pattern":"foo","path":"f.go"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal(res.Content)
	}
	if !strings.Contains(res.Content, "f.go") || !strings.Contains(res.Content, "beta foo") {
		t.Fatalf("got %q", res.Content)
	}
}

func TestGrepNoMatch(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o600)
	g := NewGrep(dir)
	res, err := g.Execute(context.Background(), `{"pattern":"nope","path":"a.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "no matches") {
		t.Fatalf("got %q", res.Content)
	}
}
