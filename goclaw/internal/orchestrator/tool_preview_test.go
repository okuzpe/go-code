package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatToolUsePreview(t *testing.T) {
	tests := []struct {
		tool, input, wantSubstr string
	}{
		{"web_search", `{"query":"noticias venezuela"}`, "noticias venezuela"},
		{"read_file", `{"path":"internal/app/run.go"}`, "internal/app/run.go"},
		{"bash", `{"command":"go test ./..."}`, "go test ./..."},
		{"unknown", `{"query":"x","extra":1}`, "x"},
		{"spawn_agent", `{"profile":"explore","task":"search the web for alog","interactive":false}`, "explore"},
		{"spawn_agent", `{"profile":"explore","task":"search the web for alog","interactive":false}`, "one-shot"},
		{"spawn_agent", `{"profile":"explore","task":"x","interactive":true}`, "interactive"},
		{"stop_task", `{"task_id":"abc123"}`, "abc123"},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := FormatToolUsePreview(tt.tool, tt.input)
			require.NotEmpty(t, got)
			require.Contains(t, got, tt.wantSubstr)
		})
	}
}

func TestFormatToolUsePreview_empty(t *testing.T) {
	require.Empty(t, FormatToolUsePreview("web_search", ""))
}

func TestToolWorkingPhrase(t *testing.T) {
	for _, tool := range []string{"web_search", "web_fetch", "read_file", "write_file", "edit_file", "glob", "grep", "bash", "script", "todo_write"} {
		require.NotEmpty(t, ToolWorkingPhrase(tool), "tool=%s", tool)
	}
	require.Equal(t, "Running MCP tool", ToolWorkingPhrase("mcp__srv__ping"))
	require.NotEmpty(t, ToolWorkingPhrase("unknown_custom_tool"))
}

func TestToolFinishedPhrase(t *testing.T) {
	for _, tool := range []string{"web_search", "web_fetch", "read_file", "write_file", "edit_file", "glob", "grep", "bash", "script", "todo_write"} {
		require.NotEmpty(t, ToolFinishedPhrase(tool), "tool=%s", tool)
	}
	require.Equal(t, "Ran MCP tool", ToolFinishedPhrase("mcp__srv__ping"))
	require.NotEmpty(t, ToolFinishedPhrase("custom_tool"))
}

func TestFormatToolUsePreview_allKnownTools(t *testing.T) {
	cases := []struct{ tool, input string }{
		{"web_fetch", `{"url":"http://example.com"}`},
		{"write_file", `{"path":"out.txt","content":"x"}`},
		{"edit_file", `{"path":"f.go","old_string":"a","new_string":"b"}`},
		{"glob", `{"pattern":"*.go"}`},
		{"grep", `{"pattern":"func","path":"."}`},
		{"script", `{"script":"echo hi"}`},
		{"todo_write", `{"merge":false,"todos":[]}`},
		{"spawn_agent", `{"profile":"explore","task":"do stuff"}`},
		{"mcp__server__tool", `{"key":"value"}`},
	}
	for _, c := range cases {
		got := FormatToolUsePreview(c.tool, c.input)
		require.NotEmpty(t, got, "tool=%s", c.tool)
	}
}
