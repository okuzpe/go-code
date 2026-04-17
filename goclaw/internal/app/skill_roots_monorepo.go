package app

import (
	"os"
	"path/filepath"
)

// appendMonorepoClaudeSkillRoot appends ../.claude/skills when wd is the goclaw module
// inside a monorepo (e.g. go-code/goclaw with skills at go-code/.claude/skills).
func appendMonorepoClaudeSkillRoot(wd string, roots []string) []string {
	wd = filepath.Clean(wd)
	if filepath.Base(wd) != "goclaw" {
		return roots
	}
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); err != nil {
		return roots
	}
	parent := filepath.Dir(wd)
	if parent == wd {
		return roots
	}
	mono := filepath.Join(parent, ".claude", "skills")
	local := filepath.Join(wd, ".claude", "skills")
	if mono == local {
		return roots
	}
	if st, err := os.Stat(mono); err == nil && st.IsDir() {
		return append(roots, mono)
	}
	return roots
}
