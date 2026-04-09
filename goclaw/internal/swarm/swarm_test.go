package swarm

import (
	"path/filepath"
	"testing"
)

func TestHubPostRead(t *testing.T) {
	dir := t.TempDir()
	h, err := Open(filepath.Join(dir, "hub"))
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.Post("alice", "bob", "hello")
	if err != nil || id != 1 {
		t.Fatalf("post: id=%d err=%v", id, err)
	}
	msgs, err := h.ReadSince("bob", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Body != "hello" || msgs[0].From != "alice" {
		t.Fatalf("msgs=%+v", msgs)
	}
	msgs2, err := h.ReadSince("bob", 1)
	if err != nil || len(msgs2) != 0 {
		t.Fatalf("expected empty after ack, got %+v err=%v", msgs2, err)
	}
}

func TestHubInvalidName(t *testing.T) {
	h, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Post("a", "../x", "nope"); err == nil {
		t.Fatal("expected error")
	}
}
