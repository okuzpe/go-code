package coordinator

import (
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
)

// hubDelegationOnly reports whether p is a read-only hub profile that only delegates
// via spawn_agent (and related meta tools). Such profiles must not be nested as workers.
func hubDelegationOnly(p agents.Profile) bool {
	if p.Name == "coordinator" {
		return true
	}
	if !p.ReadOnly || p.ToolAllowlist == nil {
		return false
	}
	allowedHubTools := map[string]struct{}{
		"spawn_agent": {},
		"stop_task":   {},
		"todo_write":  {},
	}
	hasSpawn := false
	for _, n := range p.ToolAllowlist {
		if _, ok := allowedHubTools[n]; !ok {
			return false
		}
		if n == "spawn_agent" {
			hasSpawn = true
		}
	}
	return hasSpawn
}

// spawnWorkerProfileNames returns sorted profile names that may be passed to spawn_agent
// as the worker profile (excludes hub-only delegation profiles).
func spawnWorkerProfileNames(profs map[string]agents.Profile) []string {
	if len(profs) == 0 {
		return nil
	}
	out := make([]string, 0, len(profs))
	for _, name := range agents.SortedKeys(profs) {
		if hubDelegationOnly(profs[name]) {
			continue
		}
		out = append(out, name)
	}
	return out
}

func joinWorkerProfileHint(profs map[string]agents.Profile) string {
	names := spawnWorkerProfileNames(profs)
	if len(names) == 0 {
		return "general-purpose, explore, plan, verification"
	}
	return strings.Join(names, ", ")
}
