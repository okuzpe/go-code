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

// Save appends all messages in s to its JSONL file, rotating if the file
// exceeds rotateAfterBytes.
func (st *Store) Save(s *Session) error {
	path := st.currentPath(s.ID)

	// Rotate if the existing file is too large.
	if info, err := os.Stat(path); err == nil && info.Size() >= rotateAfterBytes {
		if err := st.rotate(s.ID); err != nil {
			return fmt.Errorf("session store: rotate: %w", err)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("session store: open %s: %w", path, err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, m := range s.Messages {
		if err := enc.Encode(m); err != nil {
			return fmt.Errorf("session store: encode message: %w", err)
		}
	}
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
	var ids []string
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
