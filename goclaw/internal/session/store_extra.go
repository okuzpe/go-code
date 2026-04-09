package session

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// SessionListEntry is one saved session with file metadata for CLI display.
type SessionListEntry struct {
	ID      string
	ModTime time.Time
}

// ListSessionEntries returns current JSONL sessions with modification times (newest first).
func (st *Store) ListSessionEntries() ([]SessionListEntry, error) {
	entries, err := os.ReadDir(st.dir)
	if err != nil {
		return nil, fmt.Errorf("session store: readdir: %w", err)
	}
	var out []SessionListEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".jsonl") && strings.Count(name, ".") == 1 {
			id := strings.TrimSuffix(name, ".jsonl")
			info, ierr := e.Info()
			if ierr != nil {
				continue
			}
			out = append(out, SessionListEntry{ID: id, ModTime: info.ModTime()})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ModTime.Equal(out[j].ModTime) {
			return out[i].ID < out[j].ID
		}
		return out[i].ModTime.After(out[j].ModTime)
	})
	return out, nil
}
