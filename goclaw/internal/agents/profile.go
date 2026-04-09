// Package agents defines the six built-in agent profiles.
package agents

import (
	"sort"
	"strings"
)

// Profile configures how the orchestrator behaves for a given agent type.
type Profile struct {
	Name           string
	ModelOverride  string   // empty = use global config model
	ToolAllowlist  []string // nil = all tools allowed
	ReadOnly       bool     // if true, write/bash tools are blocked
	SystemPrompt   string   // appended to the base system prompt
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
			"Use web_search only for external docs, API versions, or facts you are unsure about — not for generic how-to or brainstorming.",
	}

	Verification = Profile{
		Name:          "verification",
		ToolAllowlist: []string{"read_file", "bash", "todo_write"},
		SystemPrompt:  "You are a verifier. Return only PASS or FAIL with a brief reason.",
	}

	Guide = Profile{
		Name:         "guide",
		ToolAllowlist: []string{},
		ReadOnly:     true,
		SystemPrompt: "You answer questions about this codebase. Never run commands.",
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
			"Never use file or shell tools directly; always delegate to workers.",
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
