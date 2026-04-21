// Package projectcontext builds the optional workspace summary injected into agent system prompts.
package projectcontext

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/config"
)

// Build reads key project files from workdir and returns a compact summary for injection
// into the system prompt. When includeProjectConventions is false, CLAUDE.md and standing
// orders are omitted (explore/plan sessions and workers).
// Returns "" when nothing useful is found, except thin mode may return a one-line hint
// when the workspace has no manifest or README so explore/plan never fall back to full
// convention text.
func Build(workdir string, cfg config.Config, includeProjectConventions bool) string {
	var parts []string

	if pt := detectProjectType(workdir); pt != "" {
		parts = append(parts, "project_type: "+pt)
	}

	type candidate struct {
		file    string
		maxLine int
		label   string
	}
	manifests := []candidate{
		{"go.mod", 8, "go.mod"},
		{"package.json", 6, "package.json"},
		{"Cargo.toml", 8, "Cargo.toml"},
		{"pyproject.toml", 8, "pyproject.toml"},
	}
	for _, c := range manifests {
		if lines, ok := readProjectFileLines(workdir, c.file, c.maxLine); ok {
			parts = append(parts, c.label+":\n  "+strings.Join(lines, "\n  "))
			break
		}
	}

	for _, name := range []string{"README.md", "README.txt", "README"} {
		if lines, ok := readProjectFileLines(workdir, name, 20); ok {
			parts = append(parts, name+":\n  "+strings.Join(lines, "\n  "))
			break
		}
	}

	if includeProjectConventions {
		claudeLines := cfg.ClaudeProjectContextLineLimit()
		if lines, ok := readProjectFileLines(workdir, "CLAUDE.md", claudeLines); ok {
			parts = append(parts, fmt.Sprintf("CLAUDE.md (project rules, first %d lines):\n  ", claudeLines)+strings.Join(lines, "\n  "))
		}

		if soPath, ok := resolveStandingOrdersPath(workdir, cfg); ok {
			maxL := cfg.StandingOrdersProjectContextLineLimit()
			if lines, ok2 := readProjectFileLinesAt(soPath, maxL); ok2 {
				joined := strings.Join(lines, "\n  ")
				maxBytes := config.StandingOrdersInjectMaxBytes()
				if len(joined) > maxBytes {
					joined = joined[:maxBytes] + "\n  ... (truncated)"
				}
				relShow, err := filepath.Rel(workdir, soPath)
				if err != nil {
					relShow = filepath.Base(soPath)
				}
				parts = append(parts, filepath.ToSlash(relShow)+" (standing orders):\n  "+joined)
			}
		}
	}

	out := strings.Join(parts, "\n\n")
	if !includeProjectConventions && strings.TrimSpace(out) == "" {
		return "project_workspace_hint: no stack manifest, requirements.txt, or README found under the tool workspace root; use glob or grep to discover layout if needed."
	}
	return out
}

func readProjectFileLines(workdir, name string, maxLines int) ([]string, bool) {
	return readProjectFileLinesAt(filepath.Join(workdir, name), maxLines)
}

func readProjectFileLinesAt(absPath string, maxLines int) ([]string, bool) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, false
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines, true
}

// FileUnderRoot returns the absolute path for userRel joined under workdir when userRel is
// strictly inside workdir (no absolute path, no ".." escape).
func FileUnderRoot(workdir, userRel string) (string, bool) {
	workdir = filepath.Clean(workdir)
	userRel = strings.TrimSpace(userRel)
	if userRel == "" {
		return "", false
	}
	userRel = filepath.Clean(userRel)
	if userRel == "." || userRel == ".." || strings.HasPrefix(userRel, ".."+string(filepath.Separator)) {
		return "", false
	}
	if filepath.IsAbs(userRel) {
		return "", false
	}
	full := filepath.Join(workdir, userRel)
	absRoot, err := filepath.Abs(workdir)
	if err != nil {
		return "", false
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absRoot, absFull)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return absFull, true
}

func resolveStandingOrdersPath(workdir string, cfg config.Config) (string, bool) {
	workdir = filepath.Clean(workdir)
	if p := strings.TrimSpace(cfg.ProjectContextStandingOrdersPath); p != "" {
		full, ok := FileUnderRoot(workdir, p)
		if !ok {
			return "", false
		}
		st, err := os.Stat(full)
		if err != nil || st.IsDir() {
			return "", false
		}
		return full, true
	}
	defaultPath := filepath.Join(workdir, ".goclaw", "STANDING_ORDERS.md")
	if st, err := os.Stat(defaultPath); err == nil && !st.IsDir() {
		return defaultPath, true
	}
	return "", false
}

func detectProjectType(workdir string) string {
	checks := []struct {
		file string
		tag  string
	}{
		{"go.mod", "go"},
		{"package.json", "nodejs"},
		{"Cargo.toml", "rust"},
		{"pyproject.toml", "python"},
		{"requirements.txt", "python"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(workdir, c.file)); err == nil {
			return c.tag
		}
	}
	return ""
}
