package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/okuzpe/goclaw/internal/text"
)

// SessionListEntry is one saved session with file metadata for CLI display.
type SessionListEntry struct {
	ID          string
	ModTime     time.Time
	PreviewText string // first user message (filled by ListSessionsDetailed; empty otherwise)
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

// ListSessionsDetailed returns current JSONL sessions sorted by id (same order as ListIDs),
// with modification time and a short preview from the first user message (for `sessions list --long`).
func (st *Store) ListSessionsDetailed() ([]SessionListEntry, error) {
	ids, err := st.ListIDs()
	if err != nil {
		return nil, err
	}
	out := make([]SessionListEntry, 0, len(ids))
	for _, id := range ids {
		path := st.currentPath(id)
		fi, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("session store: stat %s: %w", path, err)
		}
		prev, err := previewFirstUserMessage(path)
		if err != nil {
			prev = ""
		}
		out = append(out, SessionListEntry{
			ID:          id,
			ModTime:     fi.ModTime(),
			PreviewText: sanitizePreviewForTSV(prev),
		})
	}
	return out, nil
}

func previewFirstUserMessage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, jsonlScanInitialBytes)
	scanner.Buffer(buf, jsonlScanMaxBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var m llm.Message
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(m.Role), "user") {
			return firstLinePreview(m.Content), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func firstLinePreview(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	const maxRunes = 120
	s = text.TruncateRunes(s, maxRunes)
	return strings.TrimSpace(s)
}

func sanitizePreviewForTSV(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return strings.TrimSpace(s)
}

// FormatSessionListTSVLine returns one tab-separated line: id, RFC3339 mod time (UTC), preview.
func FormatSessionListTSVLine(e SessionListEntry) string {
	return strings.Join([]string{
		e.ID,
		e.ModTime.UTC().Format(time.RFC3339),
		e.PreviewText,
	}, "\t")
}
