// Package swarm provides a minimal peer mailbox on disk (V3+), separate from coordinator spawn_agent.
// Messages are stored per recipient under a hub directory for tests and future tooling.
package swarm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Message is one persisted swarm message.
type Message struct {
	ID        int64     `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Hub is a directory containing one subdirectory per mailbox id (recipient).
type Hub struct {
	root string
}

// Open returns a hub rooted at dir (created if missing).
func Open(dir string) (*Hub, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("swarm: empty root")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return &Hub{root: filepath.Clean(abs)}, nil
}

func safeSeg(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || s == "." || s == ".." {
		return false
	}
	return !strings.ContainsAny(s, `/\:`)
}

// Post writes a message into the recipient's inbox and returns its id.
func (h *Hub) Post(from, to, body string) (int64, error) {
	if !safeSeg(to) || !safeSeg(from) {
		return 0, fmt.Errorf("swarm: invalid from/to")
	}
	inbox := filepath.Join(h.root, to)
	if err := os.MkdirAll(inbox, 0o700); err != nil {
		return 0, err
	}
	id, err := nextID(inbox)
	if err != nil {
		return 0, err
	}
	msg := Message{
		ID:        id,
		From:      from,
		To:        to,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return 0, err
	}
	name := fmt.Sprintf("%d.json", id)
	path := filepath.Join(inbox, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return 0, err
	}
	return id, nil
}

func nextID(dir string) (int64, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var max int64
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		n := strings.TrimSuffix(e.Name(), ".json")
		v, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			continue
		}
		if v > max {
			max = v
		}
	}
	return max + 1, nil
}

// ReadSince returns messages for mailbox `to` with id > afterID, sorted by id ascending.
func (h *Hub) ReadSince(to string, afterID int64) ([]Message, error) {
	if !safeSeg(to) {
		return nil, fmt.Errorf("swarm: invalid to")
	}
	inbox := filepath.Join(h.root, to)
	ents, err := os.ReadDir(inbox)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []int64
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		n := strings.TrimSuffix(e.Name(), ".json")
		v, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			continue
		}
		if v > afterID {
			ids = append(ids, v)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var out []Message
	for _, id := range ids {
		path := filepath.Join(inbox, fmt.Sprintf("%d.json", id))
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var m Message
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}
