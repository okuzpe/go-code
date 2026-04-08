// Package orchestrator implements the main agent loop:
// user input → LLM → tool execution → feedback → repeat (max 32 iterations).
package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
)

const (
	maxIterations = 32
	maxToolCalls  = 64

	compactPreserveTail    = 24
	memorySnippetEntries   = 8
	compactionSnippetRunes = 120 // max runes per removed message line in the compaction summary

	// Per-provider context window estimates in tokens.
	// Anthropic claude-sonnet-4-6 / claude-opus-4: 200k token context.
	// Ollama default: conservative 32k (covers most local models; override via ModelContextTokens).
	anthropicContextTokens = 200_000
	ollamaContextTokens    = 32_000

	// compactedToolResult is the placeholder written over large tool-result payloads during phase-1 compaction.
	compactedToolResult = "[compacted]"
)

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
	"bash → ONLY for build commands, git, package managers, or tasks with no dedicated tool. " +
	"NEVER use bash to echo a message, print a greeting, or do something a dedicated tool handles.\n" +
	"5. After any tool returns, answer the user in clear prose. Do not repeat raw JSON output.\n\n"

// ToolApprover is invoked when permissions.Evaluate returns DecisionAsk.
// It should return true to run the tool, false to deny, or an error to abort.
type ToolApprover func(ctx context.Context, toolName, toolInput string) (bool, error)

// Option configures an Orchestrator.
type Option func(*Orchestrator)

// WithToolApprover registers interactive approval for Ask mode.
func WithToolApprover(a ToolApprover) Option {
	return func(o *Orchestrator) {
		o.approver = a
	}
}

// WithMemoryStore attaches a filesystem memory store; recent entries are injected into the system prompt.
func WithMemoryStore(mem *memory.Store) Option {
	return func(o *Orchestrator) {
		o.mem = mem
	}
}

// AfterToolHook is invoked after each tool finishes (before results are added to the session).
type AfterToolHook func(toolName string, resultBytes int, isError bool)

// WithAfterTool registers a hook for REPL progress / logging (optional).
func WithAfterTool(h AfterToolHook) Option {
	return func(o *Orchestrator) {
		o.afterTool = h
	}
}

// StreamSink receives streaming and tool lifecycle events during RunStreaming.
// All methods must be fast and non-blocking (or handle their own buffering),
// since they run on the orchestrator's goroutines.
type StreamSink interface {
	OnTextDelta(text string)
	OnToolUse(name, inputPreview string)
	OnToolResult(name string, resultBytes int, isError bool)
	OnDone(finalText string)
}

// WithTodoStore attaches a session-scoped task list; when non-nil, its snapshot is appended to the system prompt.
func WithTodoStore(s *todos.Store) Option {
	return func(o *Orchestrator) {
		o.todoStore = s
	}
}

// Orchestrator wires all subsystems and drives the agent loop.
type Orchestrator struct {
	cfg      config.Config
	llm      llm.Client
	session  *session.Session
	tools    *tools.Registry
	perms    *permissions.Policy
	hooks    *hooks.Registry
	profile  agents.Profile
	approver  ToolApprover
	mem       *memory.Store
	afterTool AfterToolHook
	todoStore *todos.Store
}

// New creates an Orchestrator with the provided subsystems.
func New(
	cfg config.Config,
	client llm.Client,
	sess *session.Session,
	reg *tools.Registry,
	policy *permissions.Policy,
	hookReg *hooks.Registry,
	profile agents.Profile,
	opts ...Option,
) *Orchestrator {
	o := &Orchestrator{
		cfg:     cfg,
		llm:     client,
		session: sess,
		tools:   reg,
		perms:   policy,
		hooks:   hookReg,
		profile: profile,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Run processes a single user message and returns the final assistant response.
func (o *Orchestrator) Run(ctx context.Context, userMessage string) (string, error) {
	return o.RunStreaming(ctx, userMessage, nil)
}

// RunStreaming processes a single user message. If sink is non-nil, it receives
// incremental deltas and tool lifecycle events.
func (o *Orchestrator) RunStreaming(ctx context.Context, userMessage string, sink StreamSink) (string, error) {
	o.session.Add("user", userMessage)

	toolCalls := 0

	for range maxIterations {
		o.maybeCompact()

		events, errc := o.llm.Stream(ctx, o.buildRequest())

		var response string
		var pendingTools []llm.ToolUse

		for e := range events {
			switch ev := e.(type) {
			case llm.TextDelta:
				response += ev.Text
				if sink != nil && ev.Text != "" {
					sink.OnTextDelta(ev.Text)
				}
			case llm.ToolUse:
				pendingTools = append(pendingTools, ev)
				if sink != nil {
					preview := ev.Input
					if len(preview) > 400 {
						preview = preview[:400] + "…"
					}
					sink.OnToolUse(ev.Name, preview)
				}
			case llm.Done:
				// stream finished
			}
		}

		if err := <-errc; err != nil {
			return "", fmt.Errorf("llm stream: %w", err)
		}

		if len(pendingTools) == 0 {
			o.session.AddAssistant(response, nil)
			if sink != nil {
				sink.OnDone(response)
			}
			return response, nil
		}

		records := make([]llm.ToolCallRecord, len(pendingTools))
		for i, tu := range pendingTools {
			records[i] = llm.ToolCallRecord(tu)
		}
		o.session.AddAssistant(response, records)

		results := make([]llm.ToolResultRecord, 0, len(pendingTools))
		for _, tu := range pendingTools {
			if toolCalls >= maxToolCalls {
				return "", fmt.Errorf("tool call limit (%d) reached", maxToolCalls)
			}
			toolCalls++

			out := o.executeTool(ctx, &tu)
			if out.Err != nil {
				return "", out.Err
			}
			if o.afterTool != nil {
				o.afterTool(tu.Name, len(out.Content), out.IsError)
			}
			if sink != nil {
				sink.OnToolResult(tu.Name, len(out.Content), out.IsError)
			}
			results = append(results, llm.ToolResultRecord{
				ToolUseID: tu.ID,
				ToolName:  tu.Name,
				Content:   out.Content,
				IsError:   out.IsError,
			})
		}
		o.session.AddToolResults(results)
	}

	return "", fmt.Errorf("iteration limit (%d) reached", maxIterations)
}

// ForceCompact runs compaction immediately: keeps the last compactPreserveTail messages
// and prepends a summary user message for the removed prefix. Ignores AutoCompactThreshold.
func (o *Orchestrator) ForceCompact() {
	o.compactToTail(compactPreserveTail)
}

// ReplaceSession switches the in-memory conversation to s (e.g. after /new in the REPL).
func (o *Orchestrator) ReplaceSession(s *session.Session) {
	if s == nil {
		return
	}
	o.session = s
	if o.todoStore != nil {
		o.todoStore.Clear()
	}
}

// SetProfile switches the active agent profile (tool allowlist, read-only flag, system prompt suffix).
func (o *Orchestrator) SetProfile(p agents.Profile) {
	o.profile = p
}

// ProfileName returns the active profile's name.
func (o *Orchestrator) ProfileName() string {
	return o.profile.Name
}

func (o *Orchestrator) maybeCompact() {
	if o.cfg.AutoCompactThreshold <= 0 {
		return
	}
	budget := contextBudgetTokens(o.cfg.Provider, o.cfg.ModelContextTokens)
	limit := int(float64(budget) * o.cfg.AutoCompactThreshold)
	if sessionTokenEstimate(o.session.Messages, o.cfg.Provider) < limit {
		return
	}

	// Phase 1: clear tool-result payloads in old turns — cheapest, preserves conversation structure.
	if msgs, cleared := clearOldToolResults(o.session.Messages, compactPreserveTail); cleared {
		o.session.Messages = msgs
		if sessionTokenEstimate(o.session.Messages, o.cfg.Provider) < limit {
			return
		}
	}

	// Phase 2: summarize and remove old messages.
	o.compactToTail(compactPreserveTail)
}

// clearOldToolResults replaces the Content of ToolResults in messages outside the preserved tail
// with a short placeholder, freeing context space without removing message structure.
// Returns the (possibly modified) slice and whether any content was cleared.
func clearOldToolResults(msgs []llm.Message, preserve int) ([]llm.Message, bool) {
	if len(msgs) <= preserve {
		return msgs, false
	}
	changed := false
	for i := 0; i < len(msgs)-preserve; i++ {
		for j := range msgs[i].ToolResults {
			if msgs[i].ToolResults[j].Content != "" && msgs[i].ToolResults[j].Content != compactedToolResult {
				msgs[i].ToolResults[j].Content = compactedToolResult
				changed = true
			}
		}
	}
	return msgs, changed
}

// contextBudgetTokens returns the effective context window in estimated tokens.
// cfgTokens > 0 overrides the per-provider default (set via ModelContextTokens in settings.json).
func contextBudgetTokens(provider string, cfgTokens int) int {
	if cfgTokens > 0 {
		return cfgTokens
	}
	switch provider {
	case "anthropic":
		return anthropicContextTokens
	default:
		return ollamaContextTokens
	}
}

// sessionTokenEstimate approximates tokens from message text (chars ÷ divisor by provider).
func sessionTokenEstimate(msgs []llm.Message, provider string) int {
	c := sessionCharEstimate(msgs)
	switch provider {
	case "anthropic":
		return (c + 2) / 3
	default:
		return (c + 3) / 4
	}
}

func sessionCharEstimate(msgs []llm.Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Content)
		for _, c := range m.ToolCalls {
			n += len(c.ID) + len(c.Name) + len(c.Input)
		}
		for _, r := range m.ToolResults {
			n += len(r.ToolUseID) + len(r.ToolName) + len(r.Content)
		}
	}
	return n
}

func (o *Orchestrator) compactToTail(preserve int) {
	msgs := o.session.Messages
	if len(msgs) <= preserve {
		return
	}
	head := msgs[:len(msgs)-preserve]
	tail := msgs[len(msgs)-preserve:]
	var sb strings.Builder
	sb.WriteString("[compaction] Summarized ")
	sb.WriteString(fmt.Sprintf("%d", len(head)))
	sb.WriteString(" earlier message(s); tail of ")
	sb.WriteString(fmt.Sprintf("%d", len(tail)))
	sb.WriteString(" kept. Excerpt: ")
	for i, m := range head {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		snip := m.Content
		if len([]rune(snip)) > compactionSnippetRunes {
			rs := []rune(snip)
			snip = string(rs[:compactionSnippetRunes]) + "…"
		}
		sb.WriteString(snip)
	}
	o.session.Messages = append([]llm.Message{llm.PlainMessage("user", sb.String())}, tail...)
}

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

type toolOutcome struct {
	Content string
	IsError bool
	Err     error // fatal — abort Run
}

func (o *Orchestrator) executeTool(ctx context.Context, tu *llm.ToolUse) toolOutcome {
	if o.profile.ReadOnly {
		if strings.HasPrefix(tu.Name, "mcp__") {
			return toolOutcome{
				Content: "mcp tools are disabled for read-only profiles",
				IsError: true,
			}
		}
		switch tu.Name {
		case "bash", "write_file", "edit_file":
			return toolOutcome{
				Content: fmt.Sprintf("%s is blocked for read-only profile", tu.Name),
				IsError: true,
			}
		}
	}

	switch o.perms.Evaluate(tu.Name) {
	case permissions.DecisionDeny:
		return toolOutcome{
			Content: fmt.Sprintf("permission denied for tool %q (policy mode deny)", tu.Name),
			IsError: true,
		}
	case permissions.DecisionAsk:
		if o.approver == nil {
			return toolOutcome{Err: fmt.Errorf("tool %q requires user approval; no approver configured", tu.Name)}
		}
		ok, err := o.approver(ctx, tu.Name, tu.Input)
		if err != nil {
			return toolOutcome{Err: fmt.Errorf("tool approval: %w", err)}
		}
		if !ok {
			return toolOutcome{
				Content: fmt.Sprintf("user declined execution of tool %q", tu.Name),
				IsError: true,
			}
		}
	}

	if err := o.hooks.Fire(ctx, hooks.Event{
		Type:     hooks.PreToolUse,
		ToolName: tu.Name,
		Input:    tu.Input,
	}); err != nil {
		return toolOutcome{Content: fmt.Sprintf("pre_tool_use hook blocked: %v", err), IsError: true}
	}

	t, ok := o.tools.Get(tu.Name)
	if !ok {
		return toolOutcome{Content: fmt.Sprintf("unknown tool %q", tu.Name), IsError: true}
	}

	res, err := t.Execute(ctx, tu.Input)

	hookEvent := hooks.Event{ToolName: tu.Name, Input: tu.Input, Output: res.Content}
	if err != nil {
		hookEvent.Type = hooks.PostToolUseFailure
		_ = o.hooks.Fire(ctx, hookEvent)
		return toolOutcome{Content: err.Error(), IsError: true}
	}
	if res.IsError {
		hookEvent.Type = hooks.PostToolUseFailure
	} else {
		hookEvent.Type = hooks.PostToolUse
	}
	_ = o.hooks.Fire(ctx, hookEvent)

	return toolOutcome{Content: res.Content, IsError: res.IsError}
}
