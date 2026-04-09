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
	require.Equal(t, "Searching the web", ToolWorkingPhrase("web_search"))
	require.Equal(t, "Running MCP tool", ToolWorkingPhrase("mcp__srv__ping"))
}
