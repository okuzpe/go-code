// Package agents defines the seven built-in agent profiles.
package agents

import (
	"sort"
	"strings"

	"github.com/okuzpe/goclaw/internal/text"
)

// Profile configures how the orchestrator behaves for a given agent type.
type Profile struct {
	Name          string
	Description   string   // optional; custom agents from YAML frontmatter "description"
	ModelOverride string   // empty = use global config model
	ToolAllowlist []string // nil = all tools allowed
	ReadOnly      bool     // if true, write/bash tools are blocked
	SystemPrompt  string   // appended to the base system prompt
}

// Built-in profiles. Model overrides are intentionally empty so they
// inherit the global config; set ModelOverride to pin a specific model.
var (
	GeneralPurpose = Profile{
		Name: "general-purpose",
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
		ToolAllowlist: []string{"read_file", "bash", "todo_write"},
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

	// Coordinator orchestrates complex tasks by delegating to worker agents via spawn_agent.
	// It never uses file or shell tools directly — all work is delegated.
	Coordinator = Profile{
		Name:          "coordinator",
		ToolAllowlist: []string{"spawn_agent", "stop_task", "todo_write"},
		ReadOnly:      true,
		SystemPrompt: "You are a coordinator agent. Break complex tasks into focused, self-contained " +
			"sub-tasks and delegate them to worker agents using spawn_agent. " +
			"Each spawn_agent result includes task_id; use stop_task with that id to cancel a worker that is still running. " +
			"Workers are fully isolated — they cannot see this conversation, so include all " +
			"necessary file paths, function names, and context in each task description. " +
			"Synthesize worker results into a clear final response for the user. " +
			"Never use file or shell tools directly; always delegate to workers.\n" +
			"Profile selection guide:\n" +
			"- general-purpose: any task that writes, edits, or creates files, runs commands, or implements code — use this for ALL coding and editing tasks.\n" +
			"- explore: read-only search, grep, or understanding the codebase — no changes needed.\n" +
			"- plan: produce a step-by-step implementation plan — read-only, output only.\n" +
			"- verification: run tests or checks and report PASS/FAIL — use after implementation.",
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
	m := All()
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
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
