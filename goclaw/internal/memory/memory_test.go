package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStoreRoundtripListDelete(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)

	name, err := st.Save(Entry{
		Name:        "hello world",
		Description: "test entry",
		Type:        TypeUser,
		Body:        "remember this fact",
	})
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(name, ".md"), "unexpected basename %q", name)

	list, err := st.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "hello world", list[0].Name)
	require.Equal(t, TypeUser, list[0].Type)

	loaded, err := st.Load(name)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, "remember this fact", loaded.Body)

	ctx, err := st.RecentContext(5)
	require.NoError(t, err)
	require.Contains(t, ctx, "hello world")

	require.NoError(t, st.Delete(name))
	list2, _ := st.List()
	require.Len(t, list2, 0)
}

func TestWriteIndex(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	_, err := st.Save(Entry{Name: "a", Type: TypeProject, Body: "x"})
	require.NoError(t, err)
	require.NoError(t, WriteIndex(st))
	b, err := os.ReadFile(filepath.Join(dir, "MEMORY.md"))
	require.NoError(t, err)
	require.Contains(t, string(b), "a")
}

func TestRelevantContext_FallbackToRecencyWhenQueryHasNoTerms(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	_, err := st.Save(Entry{Name: "first", Type: TypeProject, Body: "alpha change"})
	require.NoError(t, err)
	_, err = st.Save(Entry{Name: "second", Type: TypeProject, Body: "beta change"})
	require.NoError(t, err)

	// Query tokenizes to empty (stopwords only), should still return recency-based context.
	got, err := st.RelevantContext(2, "the and for with", 0.2)
	require.NoError(t, err)
	require.Contains(t, got, "first")
	require.Contains(t, got, "second")
}
