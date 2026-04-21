package memory

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
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
	last   map[string]string
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
		Path      string `json:"path"`
		Content   string `json:"content"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
		Diff      string `json:"diff"`
	}
	if err := json.Unmarshal([]byte(toolInput), &in); err != nil || strings.TrimSpace(in.Path) == "" {
		return
	}

	autoSessionQuota.mu.Lock()
	if autoSessionQuota.counts == nil {
		autoSessionQuota.counts = make(map[string]int)
	}
	if autoSessionQuota.last == nil {
		autoSessionQuota.last = make(map[string]string)
	}
	if autoSessionQuota.counts[sessionID] >= maxAutoEntriesPerSession {
		autoSessionQuota.mu.Unlock()
		slog.Info("memory: auto-capture quota reached", "session", sessionID)
		return
	}
	dedupKey := toolName + "|" + strings.TrimSpace(in.Path)
	if autoSessionQuota.last[sessionID] == dedupKey {
		autoSessionQuota.mu.Unlock()
		slog.Debug("memory: auto-capture duplicate skipped", "session", sessionID, "path", strings.TrimSpace(in.Path), "tool", toolName)
		return
	}
	autoSessionQuota.counts[sessionID]++
	autoSessionQuota.last[sessionID] = dedupKey
	autoSessionQuota.mu.Unlock()

	body := buildAutoCaptureBody(toolName, in.Path, in.Content, in.OldString, in.NewString, in.Diff)
	_, _ = store.Save(Entry{
		Name:        "auto-edit",
		Description: "auto-captured from " + toolName,
		Type:        TypeProject,
		Body:        body,
	})
}

func buildAutoCaptureBody(toolName, path, content, oldString, newString, diff string) string {
	path = strings.TrimSpace(path)
	var details []string
	switch toolName {
	case "write_file":
		details = append(details, "action: wrote file")
		if n := len([]byte(content)); n > 0 {
			details = append(details, "bytes: "+strconv.Itoa(n))
		}
	case "edit_file":
		details = append(details, "action: edited file")
		if oldN := len([]byte(oldString)); oldN > 0 {
			details = append(details, "old_string_bytes: "+strconv.Itoa(oldN))
		}
		if newN := len([]byte(newString)); newN > 0 {
			details = append(details, "new_string_bytes: "+strconv.Itoa(newN))
		}
	case "patch":
		details = append(details, "action: patched file")
		if n := len([]byte(diff)); n > 0 {
			details = append(details, "diff_bytes: "+strconv.Itoa(n))
		}
	default:
		details = append(details, "action: "+toolName)
	}
	if path != "" {
		details = append(details, "path: "+path)
	}
	return fmt.Sprintf("Auto-captured edit summary.\n%s", strings.Join(details, "\n"))
}
