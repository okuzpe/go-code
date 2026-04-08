package session

import (
	"os"
	"testing"

	"github.com/okuzpe/goclaw/internal/llm"
)

func TestStoreRoundtrip(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	sess := New()
	sess.Add("user", "hello")
	sess.Add("assistant", "hi there")

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(sess.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil for existing session")
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(loaded.Messages))
	}

	want := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	for i, m := range loaded.Messages {
		if m.Role != want[i].Role || m.Content != want[i].Content {
			t.Errorf("message[%d]: got role=%q content=%q, want role=%q content=%q", i, m.Role, m.Content, want[i].Role, want[i].Content)
		}
		if len(m.ToolCalls) != 0 || len(m.ToolResults) != 0 {
			t.Errorf("message[%d]: expected no tool fields, got calls=%d results=%d", i, len(m.ToolCalls), len(m.ToolResults))
		}
	}
}

func TestStoreLoadMissing(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	sess, err := store.Load("nonexistent-id")
	if err != nil {
		t.Fatalf("Load of missing session should not error: %v", err)
	}
	if sess != nil {
		t.Errorf("Load of missing session should return nil, got %+v", sess)
	}
}

func TestStoreRotation(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	sess := New()

	// Fill with a payload large enough to trigger rotation on the next save.
	large := make([]byte, rotateAfterBytes+1)
	for i := range large {
		large[i] = 'x'
	}
	sess.Add("user", string(large))

	if err := store.Save(sess); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	// Second save — file is over the threshold so rotation should happen.
	sess.Add("assistant", "reply")
	if err := store.Save(sess); err != nil {
		t.Fatalf("second Save (rotation): %v", err)
	}

	// The rotated file (.1.jsonl) must exist.
	rotated := store.rotatedPath(sess.ID, 1)
	if _, err := os.Stat(rotated); err != nil {
		t.Errorf("rotation file %s not found after rotation: %v", rotated, err)
	}
}
