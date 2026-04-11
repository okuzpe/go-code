package slashcmd

import (
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/session"
	"github.com/stretchr/testify/require"
)

func TestResolveSessionForResume_exactAndPrefix(t *testing.T) {
	dir := t.TempDir()
	st, err := session.NewStore(dir)
	require.NoError(t, err)
	a := session.New()
	a.Add("user", "one")
	require.NoError(t, st.Save(a))
	b := session.New()
	b.Add("user", "two")
	require.NoError(t, st.Save(b))

	got, err := resolveSessionForResume(st, a.ID)
	require.NoError(t, err)
	require.Equal(t, a.ID, got.ID)
	require.Contains(t, got.PlainTranscript(), "one")

	prefix := a.ID[:6]
	got2, err := resolveSessionForResume(st, prefix)
	require.NoError(t, err)
	require.Equal(t, a.ID, got2.ID)
}

func TestResolveSessionForResume_ambiguousPrefix(t *testing.T) {
	dir := t.TempDir()
	st, err := session.NewStore(dir)
	require.NoError(t, err)
	shared := "abcabc"
	id1 := shared + strings.Repeat("a", 26)
	id2 := shared + strings.Repeat("b", 26)
	for _, id := range []string{id1, id2} {
		s := session.New()
		s.ID = id
		require.NoError(t, st.Save(s))
	}
	_, err = resolveSessionForResume(st, shared)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ambiguous")
}
