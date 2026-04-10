package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	raw := `---
name: demo
description: A demo skill
allowed-tools:
  - Read
  - Bash
---
# Instructions

Use Read first.
`
	meta, body, ok := ParseFrontmatter([]byte(raw))
	if !ok {
		t.Fatal("expected frontmatter")
	}
	if meta.Name != "demo" || meta.Description != "A demo skill" {
		t.Fatalf("meta: %+v", meta)
	}
	if len(meta.EffectiveAllowedTools()) != 2 {
		t.Fatalf("tools: %v", meta.EffectiveAllowedTools())
	}
	if !strings.Contains(body, "Use Read first") {
		t.Fatalf("body: %q", body)
	}
}

func TestParseFrontmatterPlainNoFence(t *testing.T) {
	raw := "# Just markdown\nno yaml"
	_, body, ok := ParseFrontmatter([]byte(raw))
	if ok {
		t.Fatal("expected no frontmatter")
	}
	if body != strings.TrimSpace(raw) {
		t.Fatalf("body: %q", body)
	}
}

func TestParseFrontmatterAllowedUnderscore(t *testing.T) {
	raw := `---
name: x
allowed_tools: [Glob]
---
body`
	meta, _, ok := ParseFrontmatter([]byte(raw))
	if !ok {
		t.Fatal("expected frontmatter")
	}
	if got := meta.EffectiveAllowedTools(); len(got) != 1 || got[0] != "Glob" {
		t.Fatalf("tools: %v", meta)
	}
}

func TestCollectWithFrontmatter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := `---
name: search-helper
description: Help with grep-style tasks
allowed-tools:
  - Grep
---
## When to use
Prefer Grep over Bash for code search.
`
	path := filepath.Join(root, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := Collect([]string{root}, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "search-helper") || !strings.Contains(out, "Grep") {
		t.Fatalf("missing meta in output: %q", out)
	}
	if !strings.Contains(out, "Prefer Grep") {
		t.Fatalf("missing body: %q", out)
	}
}
