package text

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtRefDisplayLabel_shortUnchanged(t *testing.T) {
	t.Parallel()
	require.Equal(t, "@goclaw", AtRefDisplayLabel("@goclaw"))
	require.Equal(t, "@internal/foo.go", AtRefDisplayLabel("@internal/foo.go"))
}

func TestAtRefDisplayLabel_longUsesBasename(t *testing.T) {
	t.Parallel()
	in := "@internal/orchestrator/tool_transcript_snippet.go"
	got := AtRefDisplayLabel(in)
	require.True(t, strings.HasPrefix(got, "@tool_transcript"))
	require.Contains(t, got, "…")
	require.LessOrEqual(t, len([]rune(got)), AtRefDisplayMaxRunes+1)
}

func TestAtRefDisplayLabel_bareRelativePath(t *testing.T) {
	t.Parallel()
	got := AtRefDisplayLabel("internal/orchestrator/tool_transcript_snippet.go")
	require.True(t, strings.HasPrefix(got, "@"))
	require.Contains(t, got, "tool_transcript")
}

func TestAtRefDisplayLabel_absPath(t *testing.T) {
	t.Parallel()
	got := AtRefDisplayLabel("/Users/me/proj/internal/orchestrator/tool_transcript_snippet.go")
	require.True(t, strings.HasPrefix(got, "@tool_transcript"))
	require.Contains(t, got, "…")
}

func TestAtRefDisplayMaybePathLine_grepStyle(t *testing.T) {
	t.Parallel()
	line := "internal/orchestrator/tool_transcript_snippet.go:42:func foo()"
	got := AtRefDisplayMaybePathLine(line)
	require.True(t, strings.HasPrefix(got, "@tool_transcript"), got)
	require.Contains(t, got, ":42:func foo()")
}
