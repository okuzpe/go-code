package plugin

import (
	"path/filepath"
	"testing"

	"github.com/okuzpe/goclaw/internal/hooks"
)

func TestAllowed(t *testing.T) {
	if !Allowed("Demo", nil, nil) {
		t.Fatal("empty allow should allow")
	}
	if Allowed("Demo", nil, []string{"demo"}) {
		t.Fatal("deny should win")
	}
	if Allowed("Other", []string{"demo"}, nil) {
		t.Fatal("allow list should exclude")
	}
	if !Allowed("Demo", []string{"demo"}, nil) {
		t.Fatal("allow list should include")
	}
}

func TestRegisterHooksFromDirs(t *testing.T) {
	reg := hooks.New()
	dir := filepath.Join("testdata", "minimal")
	got := RegisterHooksFromDirs(reg, []string{dir}, ".", nil, nil)
	if len(got) != 1 || got[0] != "demo" {
		t.Fatalf("got %v", got)
	}
}

func TestRegisterHooksFromDirs_Deny(t *testing.T) {
	reg := hooks.New()
	dir := filepath.Join("testdata", "minimal")
	got := RegisterHooksFromDirs(reg, []string{dir}, ".", nil, []string{"demo"})
	if len(got) != 0 {
		t.Fatalf("expected deny, got %v", got)
	}
}
