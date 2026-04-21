package toolpolicy

import "testing"

func TestPendingToolsBlockParallel(t *testing.T) {
	t.Parallel()
	if PendingToolsBlockParallel([]string{"grep", "glob"}) {
		t.Fatal("expected false")
	}
	if PendingToolsBlockParallel([]string{"read_file", "bash"}) {
		t.Fatal("expected false")
	}
	if !PendingToolsBlockParallel([]string{"spawn_agent", "read_file"}) {
		t.Fatal("expected true when spawn_agent present")
	}
	if !PendingToolsBlockParallel([]string{"spawn_agent"}) {
		t.Fatal("expected true for spawn_agent alone")
	}
}

func TestCacheableWithinTurn(t *testing.T) {
	t.Parallel()
	if !CacheableWithinTurn("glob") || !CacheableWithinTurn("grep") || !CacheableWithinTurn("web_search") {
		t.Fatal("expected cacheable")
	}
	if CacheableWithinTurn("read_file") {
		t.Fatal("read_file should not be cached")
	}
}
