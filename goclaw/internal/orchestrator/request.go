package orchestrator

import (
	_ "embed"
	"strings"

	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/tools"
)

//go:embed base_system_prompt.md
var baseSystemPrompt string

const memorySnippetEntries = 8

func (o *Orchestrator) buildRequest() llm.Request {
	model := o.cfg.Model()
	if o.profile.ModelOverride != "" {
		model = o.profile.ModelOverride
	} else if o.turnModel != "" {
		model = o.turnModel
	}

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
	if o.profile.ReadOnly {
		specs = stripToolName(specs, "bash")
		specs = stripToolName(specs, "write_file")
		specs = stripToolName(specs, "edit_file")
		specs = stripToolName(specs, "patch")
		specs = stripMCPNames(specs)
	}

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
		sys = sys + "\n\n## Workspace\n" + o.workdir + "\nUse relative paths in tool calls (e.g. go.mod, internal/tools/read_file.go)."
	}
	if o.projectContext != "" {
		sys = sys + "\n\n## Project context\n" + o.projectContext
	}
	if o.skillsPrompt != "" {
		sys = sys + "\n\n## Loaded skills (SKILL.md)\n" + o.skillsPrompt
	}
	if o.mem != nil {
		if block, err := o.mem.RecentContext(memorySnippetEntries); err == nil && block != "" {
			sys = sys + "\n\n## Persistent memory (recent)\n" + block
		}
	}
	if o.todoStore != nil {
		if block := o.todoStore.FormatForPrompt(); block != "" {
			sys = sys + "\n\n## Session task list (todo_write)\n" + block
		}
	}

	if hint := userLanguageSystemSuffix(lastUserNaturalText(o.session.Messages), o.cfg); hint != "" {
		sys = sys + hint
	}

	return llm.Request{
		Model:     model,
		System:    sys,
		Messages:  o.session.Messages,
		Tools:     llmTools,
		MaxTokens: 8192,
	}
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
