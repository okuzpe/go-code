package memory

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/okuzpe/goclaw/internal/config"
)

const maxAutoEntriesPerSession = 128

var autoCountPerSession sync.Map // session id -> *int32

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
	v, _ := autoCountPerSession.LoadOrStore(sessionID, new(int32))
	cnt := v.(*int32)
	if atomic.LoadInt32(cnt) >= maxAutoEntriesPerSession {
		slog.Info("memory: auto-capture quota reached", "session", sessionID)
		return
	}
	atomic.AddInt32(cnt, 1)
	_, _ = store.Save(Entry{
		Name:        "auto-edit",
		Description: "auto-captured from " + toolName,
		Type:        TypeProject,
		Body:        "Workspace path touched: " + strings.TrimSpace(in.Path),
	})
}
