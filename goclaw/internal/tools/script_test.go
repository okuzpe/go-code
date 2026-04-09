package tools

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScriptTool_name(t *testing.T) {
	require.Equal(t, "script", NewScript().Name())
}

func TestScriptTool_description(t *testing.T) {
	require.NotEmpty(t, NewScript().Description())
}

func TestScriptTool_inputSchema(t *testing.T) {
	schema := NewScript().InputSchema()
	require.NotNil(t, schema)
	m, ok := schema.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "object", m["type"])
}

func TestScriptTool_emptyScript(t *testing.T) {
	s := NewScript()
	result, err := s.Execute(context.Background(), `{"script":""}`)
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestScriptTool_invalidJSON(t *testing.T) {
	s := NewScript()
	result, err := s.Execute(context.Background(), "not-json")
	require.Error(t, err)
	require.True(t, result.IsError)
}

func TestScriptTool_echoScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("platform-specific echo syntax differs")
	}
	s := NewScript()
	result, err := s.Execute(context.Background(), `{"script":"echo hello-world"}`)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content, "hello-world")
}

func TestScriptTool_pipeComposition(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pipe syntax differs on Windows")
	}
	s := NewScript()
	result, err := s.Execute(context.Background(), `{"script":"echo line1\necho line2 | cat"}`)
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestScriptTool_failingScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exit code handling differs on Windows")
	}
	s := NewScript()
	result, err := s.Execute(context.Background(), `{"script":"exit 1"}`)
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestScriptTool_withCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pwd output differs on Windows")
	}
	dir := t.TempDir()
	s := NewScript()
	input := `{"script":"pwd","cwd":"` + dir + `"}`
	result, err := s.Execute(context.Background(), input)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content, "tmp")
}

func TestNewScriptWithTimeout_positive(t *testing.T) {
	s := NewScriptWithTimeout(120)
	require.Equal(t, 120, s.scriptTimeout())
}

func TestNewScriptWithTimeout_zero_fallback(t *testing.T) {
	s := NewScriptWithTimeout(0)
	require.Equal(t, BashTimeoutSec, s.scriptTimeout())
}

func TestNewScriptWithTimeout_above_max_clamped(t *testing.T) {
	s := NewScriptWithTimeout(9999)
	require.Equal(t, 3600, s.scriptTimeout())
}
