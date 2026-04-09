package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollect(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "b"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "SKILL.md"), []byte("# A\nbody a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b", "skill.md"), []byte("# B\nbody b"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := Collect([]string{root}, 500)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "body a") || !strings.Contains(out, "body b") {
		t.Fatalf("unexpected: %q", out)
	}
}
