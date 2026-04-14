package memory

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync"

	"github.com/okuzpe/goclaw/internal/config"
)

const maxAutoEntriesPerSession = 128

// autoSessionQuota guards per-session auto-capture counts with a mutex so the
// check-then-increment is atomic and the map doesn't leak old sessions.
var autoSessionQuota struct {
	mu     sync.Mutex
	counts map[string]int
}

func init() {
	autoSessionQuota.counts = make(map[string]int)
}

// MaybeAutoCaptureFromTool appends a short project memory line after successful write_file / edit_file / patch
// when cfg.MemoryAutoExtract is true. Best-effort; capped per session.
func MaybeAutoCaptureFromTool(cfg config.Config, store *Store, sessionID, toolName, toolInput string, isError bool) {
	if !cfg.MemoryAutoExtract || store == nil || isError || strings.TrimSpace(sessionID) == "" {
		return
	}
	if toolName != "write_file" && toolName != "edit_file" && toolName != "patch" {
		return
	}
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(toolInput), &in); err != nil || strings.TrimSpace(in.Path) == "" {
		return
	}

	autoSessionQuota.mu.Lock()
	if autoSessionQuota.counts[sessionID] >= maxAutoEntriesPerSession {
		autoSessionQuota.mu.Unlock()
		slog.Info("memory: auto-capture quota reached", "session", sessionID)
		return
	}
	autoSessionQuota.counts[sessionID]++
	autoSessionQuota.mu.Unlock()

	_, _ = store.Save(Entry{
		Name:        "auto-edit",
		Description: "auto-captured from " + toolName,
		Type:        TypeProject,
		Body:        "Workspace path touched: " + strings.TrimSpace(in.Path),
	})
}
