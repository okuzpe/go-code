package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/okuzpe/goclaw/internal/tools"
)

func TestReadFileHappyPath(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("hello\nworld"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := tools.NewReadFile(dir)
	res, err := tool.Execute(ctx, `{"path":"a.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", res.Content)
	}
	if res.Content != "hello\nworld" {
		t.Fatalf("got %q", res.Content)
	}
}

func TestReadFileRejectsEscape(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	outside := t.TempDir()
	tool := tools.NewReadFile(dir)
	target := filepath.Join(outside, "secret.txt")
	_ = os.WriteFile(target, []byte("x"), 0o600)

	payload, _ := json.Marshal(map[string]string{"path": filepath.Join(outside, "secret.txt")})
	res, err := tool.Execute(ctx, string(payload))
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || res.Content == "" {
		t.Fatalf("expected error content, got %#v", res)
	}
}
