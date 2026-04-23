// Package projectcontext builds the optional workspace summary injected into agent system prompts.
package projectcontext

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

	if layout := summarizeWorkspaceLayout(workdir, includeProjectConventions); layout != "" {
		parts = append(parts, layout)
	}

	if git := summarizeGitWorkspace(workdir); git != "" {
		parts = append(parts, git)
	}

	if repoMap := summarizeRepoMap(workdir); repoMap != "" {
		parts = append(parts, repoMap)
	}

	for _, c := range projectDocsCandidates() {
		if lines, ok := readProjectFileLines(workdir, c.file, c.maxLine); ok {
			parts = append(parts, c.label+":\n  "+strings.Join(lines, "\n  "))
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

func projectDocsCandidates() []struct {
	file    string
	maxLine int
	label   string
} {
	return []struct {
		file    string
		maxLine int
		label   string
	}{
		{"docs/docs-map.md", 24, "docs/docs-map.md"},
		{"docs/architecture.md", 20, "docs/architecture.md"},
		{"docs/README.md", 20, "docs/README.md"},
		{"docs/index.md", 20, "docs/index.md"},
	}
}

func summarizeWorkspaceLayout(workdir string, includeProjectConventions bool) string {
	entries, err := os.ReadDir(workdir)
	if err != nil {
		return ""
	}
	type item struct {
		name     string
		isDir    bool
		children []string
	}
	var items []item
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" || shouldSkipLayoutEntry(name, includeProjectConventions) {
			continue
		}
		current := item{name: name, isDir: entry.IsDir()}
		if entry.IsDir() {
			current.children = summarizeDirChildren(filepath.Join(workdir, name), includeProjectConventions)
		}
		items = append(items, current)
	}
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].isDir != items[j].isDir {
			return items[i].isDir
		}
		return items[i].name < items[j].name
	})
	if len(items) > 10 {
		items = items[:10]
	}
	lines := make([]string, 0, len(items))
	for _, entry := range items {
		if entry.isDir {
			line := entry.name + "/"
			if len(entry.children) > 0 {
				line += " -> " + strings.Join(entry.children, ", ")
			}
			lines = append(lines, line)
			continue
		}
		lines = append(lines, entry.name)
	}
	return "workspace_layout:\n  " + strings.Join(lines, "\n  ")
}

func shouldSkipLayoutEntry(name string, includeProjectConventions bool) bool {
	if !includeProjectConventions {
		if name == "CLAUDE.md" || name == "AGENTS.md" || name == "GEMINI.md" {
			return true
		}
		if name == ".goclaw" {
			return true
		}
	}
	switch name {
	case ".git", ".idea", ".vscode", "node_modules", "vendor", ".DS_Store":
		return true
	default:
		return strings.HasPrefix(name, ".") && name != ".goclaw"
	}
}

func summarizeDirChildren(dir string, includeProjectConventions bool) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	children := make([]string, 0, 4)
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" || shouldSkipLayoutEntry(name, includeProjectConventions) {
			continue
		}
		if entry.IsDir() {
			children = append(children, name+"/")
		} else {
			children = append(children, name)
		}
		if len(children) == 4 {
			break
		}
	}
	if len(children) == 0 {
		return nil
	}
	sort.Strings(children)
	return children
}

func summarizeGitWorkspace(workdir string) string {
	branch, dirtyCount, ok := gitWorkspaceSummary(workdir)
	if !ok {
		return ""
	}
	status := "clean"
	if dirtyCount > 0 {
		status = "dirty files: " + strconv.Itoa(dirtyCount)
	}
	if branch == "" {
		branch = "(detached)"
	}
	return fmt.Sprintf("git_workspace: branch=%s, %s", branch, status)
}

func gitWorkspaceSummary(workdir string) (branch string, dirtyCount int, ok bool) {
	check := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	check.Dir = workdir
	out, err := check.Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return "", 0, false
	}

	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = workdir
	if out, err := branchCmd.Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}

	statusCmd := exec.Command("git", "status", "--short", "--untracked-files=normal")
	statusCmd.Dir = workdir
	if out, err := statusCmd.Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				dirtyCount++
			}
		}
	}
	return branch, dirtyCount, true
}

func summarizeRepoMap(workdir string) string {
	const (
		maxFiles   = 18
		maxSymbols = 4
	)
	goFiles := collectRepoMapGoFiles(workdir, maxFiles)
	if len(goFiles) == 0 {
		return ""
	}
	lines := make([]string, 0, len(goFiles))
	for _, absPath := range goFiles {
		rel, err := filepath.Rel(workdir, absPath)
		if err != nil {
			continue
		}
		pkgName, symbols, ok := parseGoFileSummary(absPath, maxSymbols)
		if !ok {
			continue
		}
		line := filepath.ToSlash(rel)
		if pkgName != "" {
			line += " [" + pkgName + "]"
		}
		if len(symbols) > 0 {
			line += ": " + strings.Join(symbols, ", ")
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	return "repo_map:\n  " + strings.Join(lines, "\n  ")
}

func collectRepoMapGoFiles(workdir string, maxFiles int) []string {
	if maxFiles <= 0 {
		return nil
	}
	files := make([]string, 0, maxFiles)
	_ = filepath.WalkDir(workdir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != workdir && shouldSkipRepoMapDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		files = append(files, path)
		if len(files) >= maxFiles {
			return fs.SkipAll
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func shouldSkipRepoMapDir(name string) bool {
	switch name {
	case ".git", ".goclaw", ".idea", ".vscode", "node_modules", "vendor", "testdata":
		return true
	default:
		return strings.HasPrefix(name, ".")
	}
}

func parseGoFileSummary(absPath string, maxSymbols int) (pkgName string, symbols []string, ok bool) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, absPath, nil, parser.SkipObjectResolution)
	if err != nil {
		return "", nil, false
	}
	pkgName = strings.TrimSpace(parsed.Name.Name)
	symbols = topLevelGoSymbols(parsed, maxSymbols)
	return pkgName, symbols, true
}

func topLevelGoSymbols(file *ast.File, maxSymbols int) []string {
	if file == nil || maxSymbols <= 0 {
		return nil
	}
	exported := make([]string, 0, maxSymbols)
	local := make([]string, 0, maxSymbols)
	appendSymbol := func(target *[]string, symbol string) {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			return
		}
		*target = append(*target, symbol)
	}
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			name := typed.Name.Name
			if typed.Recv != nil && len(typed.Recv.List) > 0 {
				name = recvTypeName(typed.Recv.List[0].Type) + "." + name
			}
			if ast.IsExported(typed.Name.Name) {
				appendSymbol(&exported, "func "+name)
			} else {
				appendSymbol(&local, "func "+name)
			}
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				switch current := spec.(type) {
				case *ast.TypeSpec:
					name := "type " + current.Name.Name
					if ast.IsExported(current.Name.Name) {
						appendSymbol(&exported, name)
					} else {
						appendSymbol(&local, name)
					}
				case *ast.ValueSpec:
					for _, ident := range current.Names {
						prefix := "var "
						if typed.Tok == token.CONST {
							prefix = "const "
						}
						name := prefix + ident.Name
						if ast.IsExported(ident.Name) {
							appendSymbol(&exported, name)
						} else {
							appendSymbol(&local, name)
						}
					}
				}
			}
		}
	}
	out := make([]string, 0, maxSymbols)
	for _, group := range [][]string{exported, local} {
		for _, symbol := range group {
			out = append(out, symbol)
			if len(out) == maxSymbols {
				return out
			}
		}
	}
	return out
}

func recvTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return recvTypeName(typed.X)
	case *ast.IndexExpr:
		return recvTypeName(typed.X)
	case *ast.IndexListExpr:
		return recvTypeName(typed.X)
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return "recv"
	}
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
