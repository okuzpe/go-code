// Package agents defines the seven built-in agent profiles.
package agents

import (
	"sort"
	"strings"

	"github.com/okuzpe/goclaw/internal/text"
)

// TERMINOLOGY
//   - Orchestrator (internal/orchestrator): the agent loop runtime. Drives ONE agent turn:
//     user message → LLM stream → tool calls → LLM feedback → repeat. Every agent profile
//     (including coordinator) runs inside an orchestrator instance.
//   - Coordinator (this file, Coordinator profile): a HUB MODE. The orchestrator runs the
//     coordinator profile, which uses spawn_agent to create isolated child orchestrators
//     (workers). The coordinator itself never touches files or shell — it delegates.
//     "Coordinator" is a profile choice; "orchestrator" is the runtime that runs it.

// Profile configures how the orchestrator behaves for a given agent type.
type Profile struct {
	Name          string
	Description   string   // optional; custom agents from YAML frontmatter "description"
	ModelOverride string   // empty = use global config model
	ToolAllowlist []string // nil = all tools allowed
	// DisallowedTools lists tool names that are removed from the effective allowlist.
	// Applied after ToolAllowlist filtering. Useful to exclude a specific tool without
	// redefining the full allowlist. Set via frontmatter "disallowed_tools".
	DisallowedTools []string
	ReadOnly        bool   // if true, write/bash tools are blocked
	SystemPrompt    string // appended to the base system prompt
	// MaxTurns caps the orchestrator loop for this profile (0 = use built-in default of 32).
	// Set via frontmatter "max_turns". Values above the built-in default are clamped to it.
	MaxTurns int
	// MemoryScope selects a per-agent memory directory instead of the global user memory store.
	// Valid values: "user" (~/.goclaw/agent-memory/<name>/),
	//               "project" (<workdir>/.goclaw/agent-memory/<name>/),
	//               "local"   (<workdir>/.goclaw/agent-memory-local/<name>/).
	// Empty (default) uses the global ~/.goclaw/memory/ store.
	// Set via frontmatter "memory".
	MemoryScope string
}

// Built-in profiles. Model overrides are intentionally empty so they
// inherit the global config; set ModelOverride to pin a specific model.
var (
	GeneralPurpose = Profile{
		Name: "general-purpose",
		SystemPrompt: `ACTION-FIRST PROTOCOL for file/code/repo/shell tasks:
  Step 1 — EXPLORE:  glob (map the tree) + read_file (key files) + grep (search)
  Step 2 — CHANGE:   edit_file (targeted) | patch (large rewrite) | write_file (new file)
  Step 3 — VERIFY:   bash or script (build, test, lint)
  Step 4 — REPORT:   ONE short paragraph: what was found and what changed.

FORBIDDEN after a coding/fix/review request:
  - Suggestion lists ("you could...", "consider...", "I recommend...")
  - Pre-action planning prose ("first I'll glob...", "then I'll read...")
  - Asking "should I proceed?" before Step 1
  - Stopping at Step 1 with only a list of gaps (no actual edits)
  - Reading only one file and then responding — read at least 5 files for any codebase analysis
  - Wrapping responses in JSON ({"response":...}, {"name":...}) unless the user asked for JSON

Review/audit/fix = find gaps → fix them → report. Not: find gaps → list them → ask.
Chat/social: answer directly, no tools.
Delegation: use spawn_agent for 3+ independent subtasks. Include all file paths and context — workers have isolated sessions and cannot see this conversation.`,
	}

	Explore = Profile{
		Name:          "explore",
		ToolAllowlist: []string{"read_file", "glob", "grep", "web_fetch", "web_search", "todo_write"},
		ReadOnly:      true,
		SystemPrompt:  "You are a fast, read-only explorer. Never modify files.",
	}

	Plan = Profile{
		Name:          "plan",
		ToolAllowlist: []string{"read_file", "glob", "grep", "web_search", "todo_write"},
		ReadOnly:      true,
		SystemPrompt: "You are a software architect. Produce a clear, step-by-step implementation plan the user can follow. " +
			"If the task is self-contained or greenfield (e.g. build a small app from scratch), answer directly from general knowledge without calling web_search. " +
			"Use read_file, glob, and grep when the plan must reflect this repository's layout or existing code. " +
			"Use web_search only for external docs, API versions, or facts you are unsure about — not for generic how-to or brainstorming. " +
			"When the plan is complete, end your response with: " +
			"\"Run `/plan save` to save this plan, then `/apply-plan` to execute it.\"",
	}

	Verification = Profile{
		Name:          "verification",
		ToolAllowlist: []string{"read_file", "bash", "script", "todo_write"},
		SystemPrompt:  "You are a verifier. Return only PASS or FAIL with a brief reason.",
	}

	Guide = Profile{
		Name:          "guide",
		ToolAllowlist: []string{},
		ReadOnly:      true,
		SystemPrompt:  "You answer questions about this codebase. Never run commands.",
	}

	StatusLine = Profile{
		Name:          "statusline",
		ToolAllowlist: []string{},
		ReadOnly:      true,
		SystemPrompt:  "Output a single short status line, no markdown.",
	}

	// Coordinator is a hub mode profile. The orchestrator runs this profile when the user
	// selects --profile coordinator. It uses spawn_agent to create isolated child
	// orchestrators (workers) and synthesizes their results. It never touches files or
	// shell tools directly — all actual work is delegated to workers.
	Coordinator = Profile{
		Name:          "coordinator",
		ToolAllowlist: []string{"spawn_agent", "stop_task", "todo_write"},
		ReadOnly:      true,
		SystemPrompt: "You are a coordinator (hub mode): delegate everything to isolated worker agents via spawn_agent, then synthesize their results. You never read files or run shell commands directly.\n" +
			"NEVER ask the user for clarification or more information before delegating. If the task is vague or ambiguous, make the most reasonable interpretation and spawn the appropriate workers immediately. NEVER say you cannot do something — you can always delegate via spawn_agent.\n" +
			"Break complex tasks into focused, self-contained sub-tasks. Each spawn_agent result includes task_id; use stop_task to cancel a running worker. " +
			"Workers are fully isolated — include all necessary file paths, function names, and context in each task description.\n" +
			"Report worker results in 1-3 lines maximum. Do not re-describe what workers did — the tool cards show it.\n" +
			"Profile selection guide:\n" +
			"- general-purpose: any task that writes, edits, or creates files, runs commands, or implements code.\n" +
			"- explore: read-only search, grep, or codebase understanding — no changes needed.\n" +
			"- plan: produce a step-by-step implementation plan — read-only output.\n" +
			"- verification: run tests or checks and report PASS/FAIL.\n" +
			"When uncertain which profile: default to general-purpose.",
	}
)

// All returns all built-in profiles indexed by name.
func All() map[string]Profile {
	profiles := []Profile{GeneralPurpose, Explore, Plan, Verification, Guide, StatusLine, Coordinator}
	m := make(map[string]Profile, len(profiles))
	for _, p := range profiles {
		m[p.Name] = p
	}
	return m
}

// SortedProfileNames returns all built-in profile names, sorted for CLI errors and help text.
func SortedProfileNames() []string {
	return SortedKeys(All())
}

// ProfileListHint is a comma-separated list of profile names for error messages.
func ProfileListHint() string {
	return strings.Join(SortedProfileNames(), ", ")
}

// SortedKeys returns profile map keys sorted lexically (built-in + custom merged maps).
func SortedKeys(profs map[string]Profile) []string {
	if len(profs) == 0 {
		return nil
	}
	names := make([]string, 0, len(profs))
	for k := range profs {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// JoinSortedProfileKeys returns SortedKeys joined with ", " for errors and slash-command hints.
func JoinSortedProfileKeys(profs map[string]Profile) string {
	if len(profs) == 0 {
		return ""
	}
	return strings.Join(SortedKeys(profs), ", ")
}

// Summary is a single-line description for listings (/agents, docs).
func (p Profile) Summary() string {
	if s := strings.TrimSpace(p.Description); s != "" {
		return text.TruncateRunes(s, 96)
	}
	switch p.Name {
	case "general-purpose":
		return "Full tools; general coding and edits."
	case "explore":
		return "Read-only explorer: read, search, web — no writes."
	case "plan":
		return "Read-only planning: architecture and step-by-step plans."
	case "verification":
		return "Verifier: PASS/FAIL style checks with limited tools."
	case "guide":
		return "Q&A about the codebase; no tools."
	case "statusline":
		return "Single-line status output; no tools."
	case "coordinator":
		return "Orchestrator: delegates to workers via spawn_agent."
	default:
		if s := strings.TrimSpace(p.SystemPrompt); s != "" {
			line := s
			if i := strings.IndexByte(line, '\n'); i >= 0 {
				line = strings.TrimSpace(line[:i])
			}
			line = strings.TrimSpace(line)
			if line != "" {
				return text.TruncateRunes(line, 96)
			}
		}
		if p.ReadOnly {
			return "Read-only custom profile."
		}
		return "Custom agent profile."
	}
}

var workspaceWriteTools = []string{"write_file", "edit_file", "patch"}

// disallowedToolSet is the set of tool names removed after allowlist filtering (orchestrator.buildRequest).
func (p Profile) disallowedToolSet() map[string]struct{} {
	out := make(map[string]struct{})
	for _, n := range p.DisallowedTools {
		n = strings.TrimSpace(n)
		if n != "" {
			out[n] = struct{}{}
		}
	}
	return out
}

// allowlistSet is non-nil only when ToolAllowlist is set; empty slice yields an empty map (no tools).
func (p Profile) allowlistSet() (allow map[string]struct{}, hasAllowlist bool) {
	if p.ToolAllowlist == nil {
		return nil, false
	}
	allow = make(map[string]struct{}, len(p.ToolAllowlist))
	for _, n := range p.ToolAllowlist {
		n = strings.TrimSpace(n)
		if n != "" {
			allow[n] = struct{}{}
		}
	}
	return allow, true
}

// profileToolMatchesAllowlist mirrors orchestrator.toolMatchesAllowlist for static profile analysis.
func profileToolMatchesAllowlist(name string, allow map[string]struct{}) bool {
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

func (p Profile) anyWriteToolAllowed(denied map[string]struct{}, allow map[string]struct{}, hasAllowlist bool) bool {
	for _, w := range workspaceWriteTools {
		if _, blocked := denied[w]; blocked {
			continue
		}
		if !hasAllowlist {
			return true
		}
		if profileToolMatchesAllowlist(w, allow) {
			return true
		}
	}
	return false
}

// AllowsWorkspaceFileWrites reports whether write_file, edit_file, or patch can appear in the
// tool list sent to the LLM (same rules as orchestrator.buildRequest).
func (p Profile) AllowsWorkspaceFileWrites() bool {
	if p.ReadOnly {
		return false
	}
	denied := p.disallowedToolSet()
	allow, hasAllowlist := p.allowlistSet()
	return p.anyWriteToolAllowed(denied, allow, hasAllowlist)
}

// AllowsSpawnAgentDelegation reports whether spawn_agent can appear on the model-visible tool list
// for this profile (nil allowlist means full registry, which includes spawn_agent on the parent orchestrator).
func (p Profile) AllowsSpawnAgentDelegation() bool {
	allow, hasAllowlist := p.allowlistSet()
	if !hasAllowlist {
		return true
	}
	if len(p.ToolAllowlist) == 0 {
		return false
	}
	denied := p.disallowedToolSet()
	if _, blocked := denied["spawn_agent"]; blocked {
		return false
	}
	return profileToolMatchesAllowlist("spawn_agent", allow)
}
