package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/tools"
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}

func TestWriteFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFile(dir)

	input := mustJSON(t, map[string]any{"path": "hello.txt", "content": "hello world"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %s", res.Content)
	}
	if !strings.Contains(res.Content, "wrote 11 bytes") {
		t.Errorf("unexpected content: %q", res.Content)
	}

	got, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("file content = %q, want %q", string(got), "hello world")
	}
}

func TestWriteFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(target, []byte("old content"), 0o600); err != nil {
		t.Fatal(err)
	}

	tool := tools.NewWriteFile(dir)
	input := mustJSON(t, map[string]any{"path": "file.txt", "content": "new content"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil || res.IsError {
		t.Fatalf("unexpected error: err=%v isError=%v content=%s", err, res.IsError, res.Content)
	}

	got, _ := os.ReadFile(target)
	if string(got) != "new content" {
		t.Errorf("file content = %q, want %q", string(got), "new content")
	}
}

func TestWriteFileEmptyContent(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFile(dir)

	input := mustJSON(t, map[string]any{"path": "empty.txt", "content": ""})
	res, err := tool.Execute(context.Background(), input)
	if err != nil || res.IsError {
		t.Fatalf("unexpected error: err=%v isError=%v content=%s", err, res.IsError, res.Content)
	}

	got, err := os.ReadFile(filepath.Join(dir, "empty.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(got))
	}
}

func TestWriteFileRejectsEscapePath(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFile(dir)

	input := mustJSON(t, map[string]any{"path": "../../outside.txt", "content": "x"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError, got content: %q", res.Content)
	}
	if !strings.Contains(res.Content, "workspace") {
		t.Errorf("error message should mention workspace, got: %q", res.Content)
	}
}

func TestWriteFileRejectsAbsoluteOutsidePath(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFile(dir)

	// Use a different temp dir as the "outside" path
	outside := t.TempDir()
	input := mustJSON(t, map[string]any{"path": filepath.Join(outside, "evil.txt"), "content": "x"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError, got content: %q", res.Content)
	}
}

func TestWriteFileExceedsSizeCap(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFile(dir)

	big := strings.Repeat("a", 1*1024*1024+1) // MaxWriteFileBytes + 1
	input := mustJSON(t, map[string]any{"path": "big.txt", "content": big})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError for oversized content")
	}
	if !strings.Contains(res.Content, "too large") {
		t.Errorf("error message should mention size, got: %q", res.Content)
	}
}

func TestWriteFileAtExactSizeCap(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFile(dir)

	exact := strings.Repeat("a", 1*1024*1024) // exactly MaxWriteFileBytes
	input := mustJSON(t, map[string]any{"path": "exact.txt", "content": exact})
	res, err := tool.Execute(context.Background(), input)
	if err != nil || res.IsError {
		t.Fatalf("expected success at exact cap: err=%v isError=%v content=%s", err, res.IsError, res.Content)
	}
}

func TestWriteFileParentDirectoryMissing(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFile(dir)

	input := mustJSON(t, map[string]any{"path": "nonexistent-dir/file.txt", "content": "x"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError for missing parent directory")
	}
	if !strings.Contains(res.Content, "parent directory") {
		t.Errorf("error message should mention parent directory, got: %q", res.Content)
	}
}

func TestWriteFileRequiresPath(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFile(dir)

	input := mustJSON(t, map[string]any{"content": "x"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError for missing path")
	}
	if !strings.Contains(res.Content, "path is required") {
		t.Errorf("got: %q", res.Content)
	}
}

func TestWriteFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFile(dir)

	_, err := tool.Execute(context.Background(), "{bad json}")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
