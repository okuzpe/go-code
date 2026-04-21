// Package toolpolicy centralizes tool execution metadata so the orchestrator
// avoids ad-hoc name switches for parallel batches, caching, and spawn rules.
package toolpolicy

import "strings"

const spawnAgentToolName = "spawn_agent"

// PreventsParallelBatch reports whether this tool must not share a parallel batch
// with other tools (e.g. spawn_agent to avoid duplicate workers / GPU contention).
func PreventsParallelBatch(toolName string) bool {
	return strings.TrimSpace(toolName) == spawnAgentToolName
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
	case "bash", "script", "run_tests":
		return true
	default:
		return false
	}
}
