package orchestrator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolCardSummaryBody_globShowsSamplePaths(t *testing.T) {
	t.Parallel()
	preview := "**/*.go"
	content := strings.Join([]string{
		"a.go",
		"b.go",
		"internal/orchestrator/tool_transcript_snippet.go",
	}, "\n") + "\n\n[truncated at 999 matches]"
	got := ToolCardSummaryBody("glob", preview, content, false)
	require.Contains(t, got, "**/*.go")
	require.Contains(t, got, "a.go")
	require.Contains(t, got, "@tool_transcript")
	require.Contains(t, got, "hit tool match cap")
}

func TestToolCardSummaryBody_globNoMatches(t *testing.T) {
	t.Parallel()
	got := ToolCardSummaryBody("glob", "*.xyz", "(no matches)", false)
	require.Contains(t, got, "*.xyz")
	require.Contains(t, got, "no matches")
}

func TestToolCardSummaryBody_bashSingleLine(t *testing.T) {
	t.Parallel()
	got := ToolCardSummaryBody("bash", "go test ./...", "ok\n", false)
	require.Equal(t, "go test ./...", got)
}

func TestTranscriptOutcomeSnippet_webSearchSkipsProvenanceLine(t *testing.T) {
	t.Parallel()
	content := "backend used: ddg (fallback from brave)\n\n1) Example result title"
	got := TranscriptOutcomeSnippet("web_search", content, false)
	require.Equal(t, "1) Example result title", got)
}

func TestTranscriptOutcomeSnippet_webSearchFallsBackToProvenance(t *testing.T) {
	t.Parallel()
	content := "backend used: ddg"
	got := TranscriptOutcomeSnippet("web_search", content, false)
	require.Equal(t, "backend used: ddg", got)
}
