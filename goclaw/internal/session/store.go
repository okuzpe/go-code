package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/okuzpe/goclaw/internal/llm"
)

const (
	rotateAfterBytes = 256 * 1024 // 256 KB
	maxRotatedFiles  = 3

	jsonlScanInitialBytes = 64 * 1024
	jsonlScanMaxBytes     = 16 * 1024 * 1024
)

// Store persists sessions as JSONL files under a directory.
// File layout:
//
//	<dir>/<session-id>.jsonl          ← current
//	<dir>/<session-id>.1.jsonl        ← oldest rotation
//	<dir>/<session-id>.2.jsonl
//	<dir>/<session-id>.3.jsonl        ← most recent rotation
type Store struct {
	dir string
}

// NewStore returns a Store backed by dir.
// The directory is created if it does not exist.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("session store: mkdir %s: %w", dir, err)
	}
	return &Store{dir: dir}, nil
}

// Save writes all messages in s to its JSONL file atomically (temp file + rename),
// rotating if the existing file exceeds rotateAfterBytes.
// An atomic write guarantees the previous file is never left in a partial state
// if a disk-full or I/O error occurs mid-write.
func (st *Store) Save(s *Session) error {
	path := st.currentPath(s.ID)

	// Rotate if the existing file is too large.
	if info, err := os.Stat(path); err == nil && info.Size() >= rotateAfterBytes {
		if err := st.rotate(s.ID); err != nil {
			return fmt.Errorf("session store: rotate: %w", err)
		}
	}

	// Write to a sibling temp file, then rename atomically so a failed write
	// never truncates the live session file.
	tmp, err := os.CreateTemp(st.dir, ".session-*.jsonl.tmp")
	if err != nil {
		return fmt.Errorf("session store: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	// Clean up temp file on any error path.
	committed := false
	defer func() {
		if !committed {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	enc := json.NewEncoder(tmp)
	for _, m := range s.Messages {
		if err := enc.Encode(m); err != nil {
			return fmt.Errorf("session store: encode message: %w", err)
		}
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("session store: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("session store: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("session store: rename: %w", err)
	}
	committed = true
	return nil
}

// Load reads the JSONL file for sessionID and returns a Session.
// Returns (nil, nil) if no file exists for that ID.
func (st *Store) Load(sessionID string) (*Session, error) {
	path := st.currentPath(sessionID)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("session store: open %s: %w", path, err)
	}
	defer f.Close()

	sess := &Session{
		ID:       sessionID,
		Messages: make([]llm.Message, 0, 32),
	}

	scanner := bufio.NewScanner(f)
	scanBuf := make([]byte, 0, jsonlScanInitialBytes)
	scanner.Buffer(scanBuf, jsonlScanMaxBytes)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m llm.Message
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			return nil, fmt.Errorf("session store: decode line: %w", err)
		}
		sess.Messages = append(sess.Messages, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("session store: scan: %w", err)
	}
	return sess, nil
}

// rotate renames the current file to the next rotation slot and prunes old ones.
func (st *Store) rotate(id string) error {
	// Shift existing rotations up: .3 deleted, .2→.3, .1→.2, current→.1
	for i := maxRotatedFiles; i > 0; i-- {
		from := st.rotatedPath(id, i-1)
		to := st.rotatedPath(id, i)
		if i == maxRotatedFiles {
			os.Remove(to) //nolint:errcheck // best-effort delete of oldest
		}
		if _, err := os.Stat(from); err == nil {
			if err := os.Rename(from, to); err != nil {
				return fmt.Errorf("rename %s → %s: %w", from, to, err)
			}
		}
	}
	return nil
}

// ListIDs returns all session IDs that have a current JSONL file in the store.
func (st *Store) ListIDs() ([]string, error) {
	entries, err := os.ReadDir(st.dir)
	if err != nil {
		return nil, fmt.Errorf("session store: readdir: %w", err)
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		// Only current files: no dot-separated rotation suffix.
		if strings.HasSuffix(name, ".jsonl") && strings.Count(name, ".") == 1 {
			ids = append(ids, strings.TrimSuffix(name, ".jsonl"))
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (st *Store) currentPath(id string) string {
	return filepath.Join(st.dir, id+".jsonl")
}

// rotatedPath returns the path for rotation index n.
// n=0 means the current file (no suffix); n=1..maxRotatedFiles are the rotations.
func (st *Store) rotatedPath(id string, n int) string {
	if n == 0 {
		return st.currentPath(id)
	}
	return filepath.Join(st.dir, fmt.Sprintf("%s.%d.jsonl", id, n))
}
