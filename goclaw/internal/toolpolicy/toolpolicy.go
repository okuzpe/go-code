// Package toolpolicy centralizes tool execution metadata so the orchestrator
// avoids ad-hoc name switches for parallel batches, caching, and spawn rules.
package toolpolicy

import "strings"

const spawnAgentToolName = "spawn_agent"

// IsWorkspaceWriteTool reports whether the tool can modify files or repo state.
func IsWorkspaceWriteTool(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "write_file", "write_files", "edit_file", "patch", "create_project", "git_tool":
		return true
	default:
		return false
	}
}

// PreventsParallelBatch reports whether this tool must not share a parallel batch
// with other tools. Keep parallel batches read-only and stateless; writes, shell
// verification, and agent spawning stay serialized to avoid order-dependent races.
func PreventsParallelBatch(toolName string) bool {
	toolName = strings.TrimSpace(toolName)
	return toolName == spawnAgentToolName ||
		IsWorkspaceWriteTool(toolName) ||
		IsVerifyTool(toolName)
}

// PendingToolsBlockParallel reports whether any tool in the list blocks parallel execution.
func PendingToolsBlockParallel(names []string) bool {
	for _, n := range names {
		if PreventsParallelBatch(n) {
			return true
		}
	}
	return false
}

// CacheableWithinTurn reports whether identical (name+input) calls in the same user turn
// may return cached results.
func CacheableWithinTurn(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "glob", "grep", "web_search":
		return true
	default:
		return false
	}
}

// IsVerifyTool reports whether the tool counts toward satisfying the post-write verify gate.
func IsVerifyTool(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "bash", "run_command", "script", "run_tests":
		return true
	default:
		return false
	}
}
