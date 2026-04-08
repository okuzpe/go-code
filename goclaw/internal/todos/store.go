// Package todos holds a session-scoped task list updated via the todo_write tool.
package todos

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// MaxItems is the maximum number of todos kept in the session store.
const MaxItems = 50

// MaxContentRunes is the maximum rune length per todo content field.
const MaxContentRunes = 500

// Item is one row in the session task list.
type Item struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Status  string `json:"status"`
}

// Store is a mutex-protected session task list.
type Store struct {
	mu    sync.Mutex
	items []Item
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{}
}

// Clear removes all todos (e.g. after /new).
func (s *Store) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = nil
}

// wireInput matches the JSON the model sends to todo_write.
type wireInput struct {
	Merge bool   `json:"merge"`
	Todos []Item `json:"todos"`
}

// Apply parses JSON from the tool and replaces or merges the list.
func (s *Store) Apply(jsonInput string) error {
	if s == nil {
		return fmt.Errorf("todo store is nil")
	}
	var in wireInput
	if err := json.Unmarshal([]byte(jsonInput), &in); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(in.Todos) == 0 {
		return fmt.Errorf("todos array must be non-empty")
	}
	if len(in.Todos) > MaxItems {
		return fmt.Errorf("at most %d todos per call", MaxItems)
	}
	for _, t := range in.Todos {
		if strings.TrimSpace(t.ID) == "" {
			return fmt.Errorf("each todo needs a non-empty id")
		}
		if strings.TrimSpace(t.Content) == "" {
			return fmt.Errorf("todo %q needs non-empty content", t.ID)
		}
		if len([]rune(t.Content)) > MaxContentRunes {
			return fmt.Errorf("todo %q content exceeds %d runes", t.ID, MaxContentRunes)
		}
		switch t.Status {
		case "pending", "in_progress", "completed":
		default:
			return fmt.Errorf("todo %q: invalid status %q (want pending, in_progress, or completed)", t.ID, t.Status)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !in.Merge {
		s.items = append([]Item(nil), in.Todos...)
		return nil
	}

	byID := make(map[string]int, len(s.items))
	for i, it := range s.items {
		byID[it.ID] = i
	}
	for _, t := range in.Todos {
		if idx, ok := byID[t.ID]; ok {
			s.items[idx] = t
		} else {
			if len(s.items) >= MaxItems {
				return fmt.Errorf("todo list already at %d items; merge cannot add new ids", MaxItems)
			}
			s.items = append(s.items, t)
			byID[t.ID] = len(s.items) - 1
		}
	}
	return nil
}

// FormatForPrompt returns a short Markdown block for the system prompt, or empty if no todos.
func (s *Store) FormatForPrompt() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) == 0 {
		return ""
	}
	var b strings.Builder
	for _, it := range s.items {
		b.WriteString("- [")
		b.WriteString(it.Status)
		b.WriteString("] ")
		b.WriteString(it.ID)
		b.WriteString(": ")
		b.WriteString(it.Content)
		b.WriteByte('\n')
	}
	return strings.TrimSuffix(b.String(), "\n")
}
