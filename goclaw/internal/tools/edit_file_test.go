package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/tools"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("writeTestFile: %v", err)
	}
	return p
}

func TestEditFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "alpha\nbeta\ngamma\n")
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "beta", "new_string": "BETA"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil || res.IsError {
		t.Fatalf("unexpected: err=%v isError=%v content=%s", err, res.IsError, res.Content)
	}
	if !strings.Contains(res.Content, "replaced 1 occurrence") {
		t.Errorf("unexpected content: %q", res.Content)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if !strings.Contains(string(got), "BETA") || strings.Contains(string(got), "beta") {
		t.Errorf("file content wrong: %q", string(got))
	}
}

func TestEditFileDeleteOccurrence(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "foo bar baz")
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "bar ", "new_string": ""})
	res, err := tool.Execute(context.Background(), input)
	if err != nil || res.IsError {
		t.Fatalf("unexpected: err=%v isError=%v content=%s", err, res.IsError, res.Content)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if string(got) != "foo baz" {
		t.Errorf("file content = %q, want %q", string(got), "foo baz")
	}
}

func TestEditFileOldStringNotFound(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "hello")
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "world", "new_string": "x"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError, got: %q", res.Content)
	}
	if !strings.Contains(res.Content, "not found") {
		t.Errorf("error message should say not found, got: %q", res.Content)
	}
	if !strings.Contains(res.Content, "0 matches") {
		t.Errorf("error message should report match count, got: %q", res.Content)
	}
}

func TestEditFileMultipleOccurrencesNoReplaceAll(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "aa bb aa")
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "aa", "new_string": "XX"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError for multiple occurrences without replace_all")
	}
	if !strings.Contains(res.Content, "found 2 times") {
		t.Errorf("error should report count, got: %q", res.Content)
	}
	if !strings.Contains(res.Content, "replace_all") {
		t.Errorf("error should suggest replace_all, got: %q", res.Content)
	}
}

func TestEditFileReplaceAll(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "aa bb aa")
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "aa", "new_string": "XX", "replace_all": true})
	res, err := tool.Execute(context.Background(), input)
	if err != nil || res.IsError {
		t.Fatalf("unexpected: err=%v isError=%v content=%s", err, res.IsError, res.Content)
	}
	if !strings.Contains(res.Content, "replaced 2 occurrence") {
		t.Errorf("unexpected content: %q", res.Content)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if string(got) != "XX bb XX" {
		t.Errorf("file content = %q, want %q", string(got), "XX bb XX")
	}
}

func TestEditFileMultilineOldString(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "line1\nline2\nline3\n")
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "line1\nline2", "new_string": "merged"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil || res.IsError {
		t.Fatalf("unexpected: err=%v isError=%v content=%s", err, res.IsError, res.Content)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if string(got) != "merged\nline3\n" {
		t.Errorf("file content = %q, want %q", string(got), "merged\nline3\n")
	}
}

func TestEditFileRejectsEscapePath(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "x")
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "../../outside.txt", "old_string": "x", "new_string": "y"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError for path escape")
	}
}

func TestEditFileFileNotFound(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "nonexistent.txt", "old_string": "x", "new_string": "y"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError for missing file")
	}
	if !strings.Contains(res.Content, "does not exist") {
		t.Errorf("error should mention file not found, got: %q", res.Content)
	}
}

func TestEditFileRequiresOldString(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "x")
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "", "new_string": "y"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError for empty old_string")
	}
	if !strings.Contains(res.Content, "old_string is required") {
		t.Errorf("got: %q", res.Content)
	}
}

func TestEditFileRequiresPath(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"old_string": "x", "new_string": "y"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError for missing path")
	}
}

func TestEditFileResultExceedsSizeCap(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "f.txt", "x")
	tool := tools.NewEditFile(dir)

	// Replace "x" with a string that exceeds the size cap
	big := strings.Repeat("y", 1*1024*1024+1)
	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "x", "new_string": big})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError for oversized result")
	}
	if !strings.Contains(res.Content, "too large") {
		t.Errorf("error should mention size, got: %q", res.Content)
	}

	// File must be unchanged
	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if string(got) != "x" {
		t.Errorf("file should be unchanged, got: %q", string(got))
	}
}

func TestEditFilePreservesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits not supported on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(target, []byte("hello world"), 0o640); err != nil {
		t.Fatal(err)
	}
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "hello", "new_string": "goodbye"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil || res.IsError {
		t.Fatalf("unexpected: err=%v isError=%v content=%s", err, res.IsError, res.Content)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("permissions = %o, want %o", info.Mode().Perm(), 0o640)
	}
}

func TestEditFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewEditFile(dir)

	_, err := tool.Execute(context.Background(), "{bad json}")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestEditFileWindowsLineEndingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// CRLF throughout — replace must not normalize line endings unexpectedly.
	writeTestFile(t, dir, "f.txt", "first line\r\nsecond line\r\nthird\r\n")
	tool := tools.NewEditFile(dir)

	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "second line", "new_string": "SECOND"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil || res.IsError {
		t.Fatalf("unexpected: err=%v isError=%v content=%s", err, res.IsError, res.Content)
	}

	got, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "first line\r\nSECOND\r\nthird\r\n"
	if string(got) != want {
		t.Errorf("file content = %q, want %q", string(got), want)
	}
}

func TestEditFileReadOnlyTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only file behaviour is platform-specific on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(target, []byte("immutable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0o644) })

	tool := tools.NewEditFile(dir)
	input := mustJSON(t, map[string]any{"path": "f.txt", "old_string": "immutable", "new_string": "changed"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError when file is read-only, got success: %q", res.Content)
	}
	low := strings.ToLower(res.Content)
	if !strings.Contains(low, "permission denied") && !strings.Contains(low, "operation not permitted") {
		t.Fatalf("expected permission error in message, got: %q", res.Content)
	}

	got, _ := os.ReadFile(target)
	if string(got) != "immutable" {
		t.Errorf("file should be unchanged, got %q", string(got))
	}
}
