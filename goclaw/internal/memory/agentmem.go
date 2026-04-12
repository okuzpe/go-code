package memory

import "path/filepath"

// PerAgentMemoryDir returns the filesystem path for a per-agent memory store based on scope.
//
// Scope values (D19 / custom-agents.md §5):
//   - "user"    → <userConfigDir>/agent-memory/<agentName>/   (not project-scoped; no VCS)
//   - "project" → <workdir>/<projectConfigDir>/agent-memory/<agentName>/   (can be committed)
//   - "local"   → <workdir>/<projectConfigDir>/agent-memory-local/<agentName>/   (not committed)
//
// Any other scope returns "" so callers can fall back to the global user store.
func PerAgentMemoryDir(scope, agentName, userConfigDir, workdir, projectConfigDir string) string {
	switch scope {
	case "user":
		return filepath.Join(userConfigDir, "agent-memory", agentName)
	case "project":
		return filepath.Join(workdir, projectConfigDir, "agent-memory", agentName)
	case "local":
		return filepath.Join(workdir, projectConfigDir, "agent-memory-local", agentName)
	default:
		return ""
	}
}
