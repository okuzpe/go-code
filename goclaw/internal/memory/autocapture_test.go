package memory

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
)

func TestMaybeAutoCaptureFromTool(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	cfg := config.Config{MemoryAutoExtract: true}
	MaybeAutoCaptureFromTool(cfg, st, t.Name(), "write_file", `{"path":"foo/bar.go","content":"x"}`, false)
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list: %+v", list)
	}
	if list[0].Type != TypeProject {
		t.Fatalf("type %s", list[0].Type)
	}
}

func TestMaybeAutoCaptureFromTool_NoOpWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	cfg := config.Config{MemoryAutoExtract: false}
	MaybeAutoCaptureFromTool(cfg, st, t.Name(), "write_file", `{"path":"x"}`, false)
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no entries")
	}
}
