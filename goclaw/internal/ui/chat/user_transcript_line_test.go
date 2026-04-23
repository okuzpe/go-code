package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAbbreviatePathForTranscript_shortUnchanged(t *testing.T) {
	t.Parallel()
	short := "/tmp/a"
	require.Equal(t, filepath.Clean(short), abbreviatePathForTranscript(short))
}

func TestAbbreviatePathForTranscript_longTail(t *testing.T) {
	t.Parallel()
	long := "/Users/someone/Projects/very-long-repo-name/sub/pkg/module"
	got := abbreviatePathForTranscript(long)
	require.True(t, strings.HasPrefix(got, "…"))
	require.Contains(t, got, "module")
	require.LessOrEqual(t, len([]rune(got)), pathChipMaxRunes+2)
}

func TestRenderUserTranscriptLine_quotedOutsideWorkspace(t *testing.T) {
	t.Parallel()
	th := DefaultTheme()
	in := "puedes arreglar el codigo?'/Users/omar.perez/Projects/go-code/goclaw'"
	got := renderUserTranscriptLine(in, th, "")
	require.Contains(t, got, "goclaw")
	require.Contains(t, got, "…", "long path should abbreviate when not under workdir")
	require.Contains(t, got, "]8;;file://", "OSC 8 hyperlink for full path")
}

func TestRenderUserTranscriptLine_quotedWorkspaceRootShowsAt(t *testing.T) {
	t.Parallel()
	th := DefaultTheme()
	root := t.TempDir()
	proj := filepath.Join(root, "goclaw")
	require.NoError(t, os.MkdirAll(proj, 0o755))
	inner := filepath.ToSlash(proj)
	in := "revisa '" + inner + "' por favor"
	got := renderUserTranscriptLine(in, th, proj)
	require.Contains(t, got, "@goclaw")
	require.Contains(t, got, "]8;;file://")
	require.NotContains(t, stripANSI(got), "…sers", "should not use ugly tail-only abbreviation for workspace root")
}

func TestRenderUserTranscriptLine_atRefUnchanged(t *testing.T) {
	t.Parallel()
	th := DefaultTheme()
	in := "see @internal/foo.go please"
	got := renderUserTranscriptLine(in, th, "")
	require.Contains(t, got, "@internal/foo.go")
}

func TestRenderUserTranscriptLine_atRefLongAbbreviates(t *testing.T) {
	t.Parallel()
	th := DefaultTheme()
	in := "see @internal/orchestrator/tool_transcript_snippet.go please"
	got := renderUserTranscriptLine(in, th, "")
	require.Contains(t, got, "@tool_transcript")
	require.Contains(t, got, "…")
}

func TestRenderUserTranscriptLine_bareUsersPath(t *testing.T) {
	t.Parallel()
	th := DefaultTheme()
	in := "fix things in /Users/x/p/go-code/goclaw thanks"
	got := renderUserTranscriptLine(in, th, "")
	require.Contains(t, got, "goclaw")
}
