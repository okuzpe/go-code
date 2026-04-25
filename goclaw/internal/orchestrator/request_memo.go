package orchestrator

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
)

func (o *Orchestrator) environmentSystemMemoKey(model string) string {
	p := o.profile
	return strings.Join([]string{
		strings.TrimSpace(p.Name),
		strings.TrimSpace(p.SystemPrompt),
		fmtBool(p.ReadOnly),
		strings.TrimSpace(o.workdir),
		strings.TrimSpace(o.launchDir),
		strings.TrimSpace(o.scratchDir),
		strings.TrimSpace(o.projectContext),
		strings.TrimSpace(o.projectContextThin),
		strings.TrimSpace(o.projectContextLite),
		hashString32(strings.TrimSpace(o.skillsPrompt)),
		strings.TrimSpace(model),
		strings.TrimSpace(o.cfg.Provider),
	}, "\x1e")
}

func fmtBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func hashString32(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%08x", h.Sum32())
}

// buildStaticSystemThroughQwen returns the system prompt fragment from base through the Qwen suffix
// (excludes per-iteration budget text, memory, todos, task-role exploration hint, language, and TUI hints).
func (o *Orchestrator) buildStaticSystemThroughQwen(model string) string {
	sys := baseSystemPrompt + o.profile.SystemPrompt
	if o.workdir != "" {
		sys = sys + "\n\n## Workspace (tool path root)\n" + o.workdir
		if ld := strings.TrimSpace(o.launchDir); ld != "" {
			if filepath.Clean(ld) != filepath.Clean(o.workdir) {
				sys = sys + "\nProcess started in: " + ld + " — relative paths in file tools resolve from this directory."
			} else {
				sys = sys + "\nRelative paths in file tools resolve from this directory unless you pass an absolute path."
			}
		}
		sys = sys + "\n\n### Paths in tool arguments (read_file, glob, grep, write_file, edit_file, patch)\n" +
			"- If the user pasted or named a **full absolute path** (Windows: `C:\\...` or `c:/...`; Unix: starts with `/`), put that **exact** string in the tool JSON. Do not replace it with only the last segment (e.g. do not use `docs` alone when they meant `c:/project/docs`).\n" +
			"- **Relative** paths in file tools resolve from the **process / launch directory** (named above when it differs from the tool path root). They are not limited to the tool path root.\n" +
			"- `glob` without `under` and `grep` without `path` (or with path `.`) search the **tool path root** tree above.\n" +
			"- For `glob`, use optional `under` to walk a different directory (absolute or relative to launch cwd)."
	}
	if sd := strings.TrimSpace(o.scratchDir); sd != "" {
		sys = sys + "\n\n## Session scratch (ephemeral)\n" +
			"Directory: " + sd + "\n" +
			"This folder is deleted when the session ends. Use it for drafts, logs, or scratch files you need without asking for write approval, " +
			"by passing this absolute path (or paths under it) to write_file, edit_file, patch, or write_files. " +
			"Do not rely on it for durable project state."
	}
	if ctx := o.selectedProjectContext(); ctx != "" {
		sys = sys + "\n\n## Project context\n" + ctx
	}
	if o.skillsPrompt != "" && !o.usesBuildLiteRuntime() {
		sys = sys + "\n\n## Loaded skills (SKILL.md)\n" + o.skillsPrompt
	}
	if qwenFamily(model) {
		sys = sys + qwenSystemSuffix
	}
	return sys
}

func (o *Orchestrator) usesBuildLiteRuntime() bool {
	if o == nil {
		return false
	}
	return agents.IsBuildLiteProfileName(o.profile.Name)
}

func (o *Orchestrator) selectedProjectContext() string {
	if o == nil {
		return ""
	}
	switch {
	case o.usesBuildLiteRuntime():
		if s := strings.TrimSpace(o.projectContextLite); s != "" {
			return s
		}
		if s := strings.TrimSpace(o.projectContextThin); s != "" {
			return s
		}
		return strings.TrimSpace(o.projectContext)
	case strings.EqualFold(strings.TrimSpace(o.profile.Name), agents.Plan.Name),
		strings.EqualFold(strings.TrimSpace(o.profile.Name), agents.Explore.Name):
		if s := strings.TrimSpace(o.projectContextThin); s != "" {
			return s
		}
		if s := strings.TrimSpace(o.projectContextLite); s != "" {
			return s
		}
		return strings.TrimSpace(o.projectContext)
	default:
		if s := strings.TrimSpace(o.projectContext); s != "" {
			return s
		}
		if s := strings.TrimSpace(o.projectContextThin); s != "" {
			return s
		}
		return strings.TrimSpace(o.projectContextLite)
	}
}
