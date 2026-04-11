package inputprefix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtFragmentAtCursor(t *testing.T) {
	// cursor right after @token at start
	frag, start, ok := AtFragmentAtCursor("@go.mod", 7)
	require.True(t, ok)
	require.Equal(t, "@go.mod", frag)
	require.Equal(t, 0, start)

	// mid-sentence: cursor inside the @token
	frag, start, ok = AtFragmentAtCursor("check @go.mod here", 13)
	require.True(t, ok)
	require.Equal(t, "@go.mod", frag)
	require.Equal(t, 6, start)

	// cursor after space — NOT inside an @-token
	_, _, ok = AtFragmentAtCursor("@go.mod ", 8)
	require.False(t, ok)

	// no @ at all
	_, _, ok = AtFragmentAtCursor("hello world", 5)
	require.False(t, ok)

	// cursor at zero
	_, _, ok = AtFragmentAtCursor("", 0)
	require.False(t, ok)

	// just "@" typed, cursor after it
	frag, start, ok = AtFragmentAtCursor("prefix @", 8)
	require.True(t, ok)
	require.Equal(t, "@", frag)
	require.Equal(t, 7, start)
}

func TestParseAtPathBuffer(t *testing.T) {
	q, ok := ParseAtPathBuffer("@internal/foo")
	require.True(t, ok)
	require.Equal(t, "internal/foo", q)
	_, ok = ParseAtPathBuffer("hello")
	require.False(t, ok)
}

func TestTUIAtPathSuggestions(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "alpha"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "alpha", "z.go"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o600))

	s := TUIAtPathSuggestions(root, "@internal")
	require.NotEmpty(t, s)
	var rels []string
	for _, x := range s {
		rels = append(rels, x.RelPath)
	}
	require.Contains(t, rels, "internal/alpha/z.go")

	s2 := TUIAtPathSuggestions(root, "@")
	require.NotEmpty(t, s2)
}

func TestAtTabExpand(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o600))

	got, ok := AtTabExpand(root, "@READ")
	require.True(t, ok)
	require.Equal(t, "@README.md", got)
}

func TestTUIAtPathSuggestions_componentMatch(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".goclaw"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".goclaw", "plan.md"), []byte("x"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "goclaw"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "goclaw", "go.mod"), []byte("x"), 0o600))

	// "@goclaw" should find both "goclaw/" and ".goclaw/" via component-contains match.
	sugs := TUIAtPathSuggestions(root, "@goclaw")
	var rels []string
	for _, s := range sugs {
		rels = append(rels, s.RelPath)
	}
	require.Contains(t, rels, "goclaw")
	require.Contains(t, rels, ".goclaw")

	// "@plan" should find ".goclaw/plan.md" by component basename match.
	sugs2 := TUIAtPathSuggestions(root, "@plan")
	var rels2 []string
	for _, s := range sugs2 {
		rels2 = append(rels2, s.RelPath)
	}
	require.Contains(t, rels2, ".goclaw/plan.md")
}

func TestAtTabExpand_componentMatchTab(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".goclaw"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".goclaw", "plan.md"), []byte("x"), 0o600))

	// "@plan" matches ".goclaw/plan.md" via component match; Tab should complete to it.
	got, ok := AtTabExpand(root, "@plan")
	require.True(t, ok)
	require.Equal(t, "@.goclaw/plan.md ", got) // file: trailing space
}

func TestAtTabExpand_multiSuggestPicksFirst(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "alpha.go"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "beta.go"), []byte("x"), 0o600))

	// "@" has multiple suggestions with different first chars — Tab picks the first (alphabetical).
	got, ok := AtTabExpand(root, "@")
	require.True(t, ok)
	require.Equal(t, "@alpha.go ", got) // file: trailing space marks completion
}

func TestAtTabExpand_dirFallbackNoTrailingSpace(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tmp"), 0o700))

	// Multiple dirs, LCP == "@" — Tab picks first dir, no trailing space.
	got, ok := AtTabExpand(root, "@")
	require.True(t, ok)
	require.Equal(t, "@src/", got) // directory: no trailing space
}
