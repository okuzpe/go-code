package inputprefix

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyzePassthrough(t *testing.T) {
	a, err := Analyze("hello world")
	require.NoError(t, err)
	require.Equal(t, KindPassthrough, a.Kind)
	require.Equal(t, "hello world", a.Original)
}

func TestAnalyzeBangBash(t *testing.T) {
	a, err := Analyze("! echo ok")
	require.NoError(t, err)
	require.Equal(t, KindLocalTool, a.Kind)
	require.Equal(t, "bash", a.ToolName)
	require.Contains(t, a.ToolInputJSON, "echo ok")
	require.Equal(t, "! echo ok", a.UserLine)
}

func TestAnalyzeBangEmpty(t *testing.T) {
	_, err := Analyze("!")
	require.Error(t, err)
}

func TestAnalyzeAtReadFile(t *testing.T) {
	a, err := Analyze("@README.md")
	require.NoError(t, err)
	require.Equal(t, KindLocalTool, a.Kind)
	require.Equal(t, "read_file", a.ToolName)
	require.Contains(t, a.ToolInputJSON, "README.md")
}

func TestAnalyzeAtEmpty(t *testing.T) {
	_, err := Analyze("@")
	require.Error(t, err)
}

func TestAnalyzeAmpSpawn(t *testing.T) {
	a, err := Analyze("& fix the bug in main.go")
	require.NoError(t, err)
	require.Equal(t, KindLocalTool, a.Kind)
	require.Equal(t, "spawn_agent", a.ToolName)
	require.Contains(t, a.ToolInputJSON, "general-purpose")
	require.Contains(t, a.ToolInputJSON, "fix the bug")
}

func TestAnalyzeMultilineRejected(t *testing.T) {
	_, err := Analyze("! ls\nmore")
	require.Error(t, err)
	_, err = Analyze("& task\nmore")
	require.Error(t, err)
}

func TestAnalyzeAtMultilinePassthrough(t *testing.T) {
	// Multi-line @ message: "@go.mod\nexplain this" should now be KindPassthrough
	// so inline expansion can read the file and include the instruction together.
	a, err := Analyze("@go.mod\nexplain this")
	require.NoError(t, err)
	require.Equal(t, KindPassthrough, a.Kind)
}

func TestBtwRewrite(t *testing.T) {
	s := BtwRewrite("  what is Go?  ")
	require.Contains(t, s, "what is Go?")
	require.Contains(t, s, "Side question")
}
