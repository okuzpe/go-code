package memory_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/memory"
)

func TestStoreRoundtripListDelete(t *testing.T) {
	dir := t.TempDir()
	st := memory.New(dir)

	name, err := st.Save(memory.Entry{
		Name:        "hello world",
		Description: "test entry",
		Type:        memory.TypeUser,
		Body:        "remember this fact",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !strings.HasSuffix(name, ".md") {
		t.Fatalf("unexpected basename %q", name)
	}

	list, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 entry, got %d", len(list))
	}
	if list[0].Name != "hello world" || list[0].Type != memory.TypeUser {
		t.Fatalf("list[0]: %+v", list[0])
	}

	loaded, err := st.Load(name)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.Body != "remember this fact" {
		t.Fatalf("body: %q", loaded.Body)
	}

	ctx, err := st.RecentContext(5)
	if err != nil {
		t.Fatalf("RecentContext: %v", err)
	}
	if !strings.Contains(ctx, "hello world") {
		t.Fatalf("context: %q", ctx)
	}

	if err := st.Delete(name); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list2, _ := st.List()
	if len(list2) != 0 {
		t.Fatalf("after delete want 0, got %d", len(list2))
	}
}

func TestWriteIndex(t *testing.T) {
	dir := t.TempDir()
	st := memory.New(dir)
	if _, err := st.Save(memory.Entry{Name: "a", Type: memory.TypeProject, Body: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := memory.WriteIndex(st); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "MEMORY.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "a") {
		t.Fatalf("MEMORY.md: %s", b)
	}
}
