package orchestrator

import (
	"strings"

	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/tools"
)

const memorySnippetEntries = 8

// baseSystemPrompt is prepended to every profile.
// Written as short numbered rules so local models (Ollama) are more likely to obey.
const baseSystemPrompt = "You are the goclaw coding agent.\n\n" +
	"RULES (follow strictly):\n" +
	"1. TOOL CALLS: Always use the native tool/function-calling mechanism. " +
	"NEVER write tool requests as ```json, fenced JSON, or {\"name\":…} in your reply text.\n" +
	"2. GREETINGS: If the user just says hello, thanks, or asks a simple conversational question, " +
	"reply in plain text. DO NOT call bash, DO NOT call any tool.\n" +
	"3. WEB / NEWS / INTERNET: When the user asks you to search the web, find news, look up current events, " +
	"or get any online information, you MUST call the web_search tool. " +
	"You CAN search the internet — the web_search tool is available to you. " +
	"NEVER say \"I cannot search the web\" or \"I cannot access the internet\". " +
	"After web_search returns, summarize the results for the user.\n" +
	"4. TOOL PRIORITY: " +
	"read_file, glob, grep → read/search code. " +
	"write_file, edit_file → edit code. " +
	"web_search → internet queries. " +
	"web_fetch → fetch a specific URL. " +
	"bash → single commands (build, git, package managers); no pipes or chaining. " +
	"script → shell composition requiring pipes (|), && chaining, or redirections (if available). " +
	"NEVER use bash to echo a message, print a greeting, or do something a dedicated tool handles.\n" +
	"5. After any tool returns, answer the user in clear prose. Do not repeat raw JSON output.\n\n"

func (o *Orchestrator) buildRequest() llm.Request {
	model := o.cfg.Model()
	if o.profile.ModelOverride != "" {
		model = o.profile.ModelOverride
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
