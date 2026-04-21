// Package agents defines the built-in agent profiles (see All()).
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
	// MaxTurns caps the orchestrator loop for this profile (0 = use configured default iteration cap).
	// Set via frontmatter "max_turns". Values above the effective iteration cap are clamped to it.
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
		SystemPrompt: `The embedded base system prompt already defines workflow, tool-first rules, review-and-fix, paths, parallel tools, and scope — follow it for all file/repo/shell work.
Follow the cycle for any coding or fix request: EXPLORE (glob/grep/read_file) → APPLY (edit_file/write_file/patch) → VERIFY (bash/script: go build, go test, or project verify.sh). If verification fails, diagnose the error, fix it, and re-verify (max 3 retries). Report with evidence if still failing after retries.
Phase discipline: analyze the request → gather evidence with tools → propose changes through edits → second pass (re-read touched regions or grep for missed references) → run verification commands — then summarize for the user.
Use spawn_agent when you have 3+ independent subtasks suited to separate workers; each task description must be self-contained (absolute paths, symbols, acceptance criteria) because workers do not see this conversation.
For single-threaded work, use tools directly; do not delegate trivial one-shot tasks.`,
	}

	// Builder is a direct-coding profile: same full tool surface as general-purpose,
	// with stronger bias toward short, action-first replies and immediate tool use.
	Builder = Profile{
		Name: "builder",
		SystemPrompt: `Same tool surface as general-purpose: follow the base system prompt for workflow, tool-first rules, paths, and scope.
Prefer acting over explaining — use read_file, glob, grep, bash, write_file, edit_file, and patch to deliver outcomes; keep user-visible prose minimal after tools run.
Follow the cycle for any coding or fix request: EXPLORE (glob/grep/read_file) → APPLY (edit_file/write_file/patch) → VERIFY (bash/script: go build, go test, or project verify.sh). If verification fails, diagnose the error, fix it, and re-verify (max 3 retries). Report with evidence if still failing after retries.
Phase discipline: analyze → tool-backed exploration → apply edits (your proposal on disk) → quick second pass on changed code → verify — then a minimal summary.
Use spawn_agent when you have 3+ independent subtasks suited to separate workers; each task description must be self-contained (absolute paths, symbols, acceptance criteria) because workers do not see this conversation.
For single-threaded work, use tools directly; do not delegate trivial one-shot tasks.`,
	}

	Explore = Profile{
		Name:          "explore",
		ToolAllowlist: []string{"read_file", "glob", "grep", "web_fetch", "web_search", "todo_write"},
		ReadOnly:      true,
		SystemPrompt: "You are a fast, read-only explorer. Never modify files. You cannot spawn sub-agents. " +
			"If read_file output shows truncation or is empty, call read_file again with offset_lines/limit_lines until you have the range you need.\n\n" +
			"Before your final answer to the user, make at least **three** tool invocations total using read_file, glob, grep, web_fetch, and/or web_search " +
			"(repeated use of the same tool counts). " +
			"Exception: if the question is answerable without repository or web evidence, use fewer tools and add one sentence explaining why.",
	}

	Plan = Profile{
		Name:          "plan",
		ToolAllowlist: []string{"read_file", "glob", "grep", "web_search", "todo_write"},
		ReadOnly:      true,
		SystemPrompt: "You are a software architect. Follow this flow unless the user asks for something narrower:\n" +
			"1) **Understand** — restate the goal, scope, and constraints in your own words.\n" +
			"2) **Analyze** — what parts of this repository or problem domain matter; do not invent paths or symbols you have not seen.\n" +
			"3) **Propose** — a recommended approach; when tradeoffs exist, name them briefly and pick a default with rationale.\n" +
			"4) **Concretize** — ordered, verifiable steps and explicit acceptance criteria (what \"done\" means).\n" +
			"5) **Close** — how to persist and execute: `/plan save`, `/plan review`, `/plan approve` when required, `/plan run` or `/apply-plan` (add `--hub` for coordinator execution); mention `/plan save` then `/apply-plan --preview` for a cautious path.\n\n" +
			"If the task is self-contained or greenfield (e.g. build a small app from scratch), you may answer from general knowledge without web_search. " +
			"If the request is purely conceptual and needs no repository evidence, answer without tools.\n" +
			"Use read_file, glob, and grep when the plan must reflect this repository's layout or existing code. " +
			"Use web_search only for external docs, API versions, or facts you are unsure about — not for generic how-to or brainstorming. " +
			"Keep the plan in chat until the user persists it — do not create plan markdown files on disk unless they use `/plan save`.\n\n" +
			"This profile cannot edit the repo or run shell — there is no automatic handoff to builder. " +
			"Use native tool calls from the API only for reads/search; never paste `{\"name\":...}` tool JSON as plain assistant text (it does not run). " +
			"Always end with a short **Next steps (you run these)** block: `/plan save` (optional path) → `/apply-plan --preview` → `/apply-plan` to execute (that switches to general-purpose; add `--hub` if they use coordinator). " +
			"If `plan_require_apply_approval` is enabled in their config, mention `/plan review` → `/plan approve` before apply. " +
			"Close with **one explicit question** asking whether they want to save and preview-apply, or what to adjust first — do not assume they already ran slash commands.\n\n" +
			"When the plan is grounded in this repository, include a **Critical files** subsection (see PLAN_PROFILE_MODE rules in the system prompt for the exact 3–5 file requirement).",
	}

	Verification = Profile{
		Name:          "verification",
		ToolAllowlist: []string{"read_file", "bash", "script", "todo_write"},
		SystemPrompt: "You are a verifier. Use read_file, bash, or script as needed to check the user's claim or implementation. " +
			"The first line of your reply must be exactly one of: VERDICT: PASS, VERDICT: FAIL, or VERDICT: PARTIAL (no other text on that line). " +
			"Then one short paragraph: what you ran, evidence, and limits of the check. " +
			"When verification requires commands, run them; do not skip checks.",
	}

	// CodeReview is read-only for workspace files: no write_file, edit_file, or patch on the allowlist.
	// ReadOnly is false so the orchestrator still exposes bash (required for non-hub read-only shell).
	CodeReview = Profile{
		Name:        "code-review",
		Description: "Structured review of a git diff: read tools + bash; no workspace writes.",
		ToolAllowlist: []string{
			"read_file", "glob", "grep", "bash", "web_fetch", "web_search", "todo_write",
		},
		ReadOnly: false,
		SystemPrompt: "You are a senior code reviewer. You cannot modify the repository: write_file, edit_file, and patch are not available.\n" +
			"The user message includes a git diff (or states if the tree is clean).\n\n" +
			"Output:\n" +
			"1) Short summary (2–4 sentences).\n" +
			"2) Findings as bullets. Each: severity (blocker / major / minor / nit), category (correctness / security / performance / maintainability / tests / docs), " +
			"location (path or hunk), description, suggested fix in prose only.\n\n" +
			"Use read_file or grep only when the diff lacks surrounding context.\n" +
			"Use bash only for non-destructive introspection (e.g. git log, git show, go vet on one package). " +
			"If the diff is empty, say so and suggest comparing refs (e.g. /review --staged, /review main HEAD).",
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
		SystemPrompt: "You are a coordinator (hub mode): delegate everything to isolated worker agents via spawn_agent, then synthesize their results. You never read files or run shell commands directly. For any task involving the repo, your first tool output is spawn_agent (not read_file).\n" +
			"Phases: (1) Analyze — restate the goal, constraints, and acceptance criteria; split work when it helps delegation. " +
			"(2) Delegate tool use — spawn workers (general-purpose, builder, explore, etc.) with self-contained task text; they run read/search/edit/shell. " +
			"(3) Propose / merge — integrate worker outputs; if workers disagree, state tradeoffs briefly. " +
			"(4) Second pass — when risk is high or the user asked for assurance, spawn verification or code-review (or another focused worker) before claiming completion. " +
			"(5) Execute / close — give a short final answer; only claim disk or command outcomes that appear in spawn_agent results.\n" +
			"For open-ended tasks without a written implementation plan, do not block on clarifying questions — make a reasonable interpretation and spawn workers immediately. " +
			"When the user message is an explicit saved implementation plan (for example after /apply-plan), execute it in order: prefer sequential spawn_agent calls (one major step at a time) unless two steps are clearly independent and safe to parallelize on the same workspace. Use todo_write to track plan progress.\n" +
			"Break complex tasks into focused, self-contained sub-tasks. Each spawn_agent result includes task_id; use stop_task to cancel a running worker. " +
			"Workers are fully isolated — include all necessary file paths, function names, and context in each task description.\n" +
			"Never fabricate worker results or code you did not see in a spawn_agent return; only summarize and integrate what workers actually reported. " +
			"Do not claim you read a file unless a worker's output shows it — you have no direct read_file.\n" +
			"Report worker results in 1-3 lines maximum. Do not re-describe what workers did — the tool cards show it.\n" +
			"Profile selection guide:\n" +
			"- builder or general-purpose: any task that writes, edits, or creates files, runs commands, or implements code.\n" +
			"- explore: read-only search, grep, or codebase understanding — no changes needed.\n" +
			"- plan: produce a step-by-step implementation plan — read-only output.\n" +
			"- verification: run tests or checks; worker must start with VERDICT: PASS, VERDICT: FAIL, or VERDICT: PARTIAL.\n" +
			"- code-review: read-only review of a git diff (no file writes); use after the user runs /review or for PR-style feedback.\n" +
			"When uncertain which profile: default to general-purpose or builder.",
	}
)

// All returns all built-in profiles indexed by name.
func All() map[string]Profile {
	profiles := []Profile{GeneralPurpose, Builder, Explore, Plan, Verification, CodeReview, Guide, StatusLine, Coordinator}
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
	case "builder":
		return "Full tools; action-first coding and edits (default session profile)."
	case "explore":
		return "Read-only explorer: read, search, web — no writes."
	case "plan":
		return "Read-only planning: architecture and step-by-step plans."
	case "verification":
		return "Verifier: VERDICT: PASS/FAIL/PARTIAL style checks with limited tools."
	case "code-review":
		return "Read-only review of a diff: no writes; bash for git/vet only."
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

var workspaceWriteTools = []string{"write_file", "write_files", "create_project", "edit_file", "patch"}

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
