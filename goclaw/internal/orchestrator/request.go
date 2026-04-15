package orchestrator

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/text"
	"github.com/okuzpe/goclaw/internal/tools"
)

//go:embed base_system_prompt.md
var baseSystemPrompt string

const (
	memorySnippetEntries = 4    // max memory entries injected per store
	memoryMaxBytes       = 4096 // max bytes per memory block to prevent silent context bloat
)

// effectiveToolSpecs returns registry specs after profile allowlist, disallowed, and read-only stripping.
func (o *Orchestrator) effectiveToolSpecs() []tools.ToolSpec {
	specs := o.tools.Specs()
	if o.profile.ToolAllowlist != nil {
		if len(o.profile.ToolAllowlist) == 0 {
			specs = nil
		} else {
			allow := make(map[string]struct{}, len(o.profile.ToolAllowlist))
			for _, n := range o.profile.ToolAllowlist {
				allow[n] = struct{}{}
			}
			filtered := make([]tools.ToolSpec, 0, len(specs))
			for _, s := range specs {
				if toolMatchesAllowlist(s.Name, allow) {
					filtered = append(filtered, s)
				}
			}
			specs = filtered
		}
	}
	if len(o.profile.DisallowedTools) > 0 {
		denied := make(map[string]struct{}, len(o.profile.DisallowedTools))
		for _, n := range o.profile.DisallowedTools {
			denied[n] = struct{}{}
		}
		filtered := make([]tools.ToolSpec, 0, len(specs))
		for _, s := range specs {
			if _, blocked := denied[s.Name]; !blocked {
				filtered = append(filtered, s)
			}
		}
		specs = filtered
	}
	if o.profile.ReadOnly {
		specs = stripToolName(specs, "bash")
		specs = stripToolName(specs, "write_file")
		specs = stripToolName(specs, "edit_file")
		specs = stripToolName(specs, "patch")
		specs = stripMCPNames(specs)
	}
	return specs
}

func (o *Orchestrator) buildRequest() llm.Request {
	model := o.cfg.Model()
	if o.profile.ModelOverride != "" {
		model = o.profile.ModelOverride
	} else if o.turnModel != "" {
		model = o.turnModel
	}

	specs := o.effectiveToolSpecs()

	llmTools := make([]llm.ToolSpec, 0, len(specs))
	for _, s := range specs {
		llmTools = append(llmTools, llm.ToolSpec{
			Name:        s.Name,
			Description: s.Description,
			InputSchema: s.InputSchema,
		})
	}

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
	if o.projectContext != "" {
		sys = sys + "\n\n## Project context\n" + o.projectContext
	}
	if o.skillsPrompt != "" {
		sys = sys + "\n\n## Loaded skills (SKILL.md)\n" + o.skillsPrompt
	}
	if o.mem != nil {
		if block, err := o.mem.RecentContext(memorySnippetEntries); err == nil && block != "" {
			block = truncateMemoryBlock(block, memoryMaxBytes)
			sys = sys + "\n\n## Persistent memory (recent)\n" + block
		}
	}
	if o.projectMem != nil {
		if block, err := o.projectMem.RecentContext(memorySnippetEntries); err == nil && block != "" {
			block = truncateMemoryBlock(block, memoryMaxBytes)
			sys = sys + "\n\n## Project memory (.goclaw/memory)\n" + block
		}
	}
	if o.todoStore != nil {
		if block := o.todoStore.FormatForPrompt(); block != "" {
			sys = sys + "\n\n## Session task list (todo_write)\n" + block
		}
	}

	if hint := taskExplorationHint(o.taskRole); hint != "" {
		sys = sys + hint
	}

	if hint := userLanguageSystemSuffix(lastUserNaturalText(o.session.Messages), o.cfg); hint != "" {
		sys = sys + hint
	}

	// Budget reminder: inject once we are past the halfway point of available iterations,
	// so the model knows it is approaching its ceiling and should wrap up or report.
	if o.budgetIter > 0 && o.budgetLimit > 0 && o.budgetIter > o.budgetLimit/2 {
		remaining := o.budgetLimit - o.budgetIter
		toolsLeft := o.effectiveMaxToolCalls() - o.budgetToolCalls
		softenBudget := !o.profile.ReadOnly &&
			toolSpecsAllowWorkspaceWrite(specs) &&
			userMessageWantsWorkspaceWrites(o.turnUserMessage) &&
			o.turnHadToolRound &&
			!o.turnWorkspaceWriteOK
		var body string
		if softenBudget {
			body = "You are running low on iteration budget. If the user asked for concrete code changes, prioritize completing in-flight edits (write_file, edit_file, patch) or clearly stating blockers — avoid stopping with analysis-only while edits are still expected."
		} else {
			body = "If you are close to finishing, wrap up and report rather than starting new work."
		}
		sys = sys + fmt.Sprintf(
			"\n\n<system-reminder>Budget: iteration %d/%d (%d remaining). Tool calls used: %d/%d (%d remaining). %s</system-reminder>",
			o.budgetIter, o.budgetLimit, remaining,
			o.budgetToolCalls, o.effectiveMaxToolCalls(), toolsLeft,
			body,
		)
	}

	maxTokens := 4096
	if o.cfg.MaxResponseTokens > 0 {
		maxTokens = o.cfg.MaxResponseTokens
	}
	req := llm.Request{
		Model:     model,
		System:    sys,
		Messages:  o.session.Messages,
		Tools:     llmTools,
		MaxTokens: maxTokens,
	}
	if o.cfg.Provider == "ollama" && o.cfg.OllamaNumCtx > 0 {
		req.NumCtx = o.cfg.OllamaNumCtx
	}
	return req
}

func stripToolName(specs []tools.ToolSpec, name string) []tools.ToolSpec {
	out := specs[:0]
	for _, s := range specs {
		if s.Name != name {
			out = append(out, s)
		}
	}
	return out
}

func stripMCPNames(specs []tools.ToolSpec) []tools.ToolSpec {
	out := make([]tools.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if strings.HasPrefix(s.Name, "mcp__") {
			continue
		}
		out = append(out, s)
	}
	return out
}

// toolMatchesAllowlist supports exact names and trailing-wildcard prefixes (e.g. mcp__demo__*).
func toolMatchesAllowlist(name string, allow map[string]struct{}) bool {
	if _, ok := allow[name]; ok {
		return true
	}
	for pat := range allow {
		if strings.HasSuffix(pat, "*") && len(pat) > 1 {
			prefix := strings.TrimSuffix(pat, "*")
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
	}
	return false
}

// truncateMemoryBlock caps a memory block at maxRunes runes, appending a marker
// when truncated. Using rune boundaries avoids splitting multibyte UTF-8 characters
// (e.g. emoji or CJK) that byte-slicing would corrupt.
func truncateMemoryBlock(block string, maxRunes int) string {
	if utf8.RuneCountInString(block) <= maxRunes {
		return block
	}
	return text.TruncateRunes(block, maxRunes) + "\n[memory truncated]"
}
