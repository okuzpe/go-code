package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/okuzpe/goclaw/internal/llm"
	"github.com/stretchr/testify/require"
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

func TestStoreRoundtripWithToolTurn(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	sess := New()
	sess.AddAssistant("I will run a tool.", []llm.ToolCallRecord{
		{ID: "call-1", Name: "bash", Input: `{"command":"echo hi"}`},
	})
	sess.AddToolResults([]llm.ToolResultRecord{
		{ToolUseID: "call-1", ToolName: "bash", Content: "hi\n", IsError: false},
	})
	sess.Add("user", "thanks")

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(sess.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if len(loaded.Messages) != 3 {
		t.Fatalf("want 3 messages after round-trip, got %d", len(loaded.Messages))
	}

	a := loaded.Messages[0]
	if a.Role != "assistant" || len(a.ToolCalls) != 1 || a.ToolCalls[0].ID != "call-1" || a.ToolCalls[0].Name != "bash" {
		t.Fatalf("assistant+tool_calls mismatch: %+v", a)
	}
	u := loaded.Messages[1]
	if u.Role != "user" || len(u.ToolResults) != 1 || u.ToolResults[0].ToolUseID != "call-1" || u.ToolResults[0].Content != "hi\n" {
		t.Fatalf("user tool_results mismatch: %+v", u)
	}
	if loaded.Messages[2].Role != "user" || loaded.Messages[2].Content != "thanks" {
		t.Fatalf("final user message: %+v", loaded.Messages[2])
	}
}

func TestStoreListIDsIgnoresRotationFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	id := "abc123def456"
	require.NoError(t, os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte("{}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, id+".1.jsonl"), []byte("{}\n"), 0o600))

	ids, err := store.ListIDs()
	if err != nil {
		t.Fatalf("ListIDs: %v", err)
	}
	require.Equal(t, []string{id}, ids)
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

func TestListSessionEntriesOrdersNewestFirst(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	require.NoError(t, err)
	s1 := New()
	require.NoError(t, st.Save(s1))
	oldID := s1.ID

	require.NoError(t, os.Chtimes(filepath.Join(dir, oldID+".jsonl"), time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)))

	s2 := New()
	require.NoError(t, st.Save(s2))

	got, err := st.ListSessionEntries()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 2)
	require.Equal(t, s2.ID, got[0].ID, "newer file should sort first")
}
