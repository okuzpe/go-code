package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatPrefixToolReply(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		body        string
		isError     bool
		mustContain []string
	}{
		{
			name:        "success bash reply includes tool name and body",
			toolName:    "bash",
			body:        "hello",
			isError:     false,
			mustContain: []string{"bash", "hello"},
		},
		{
			name:        "error read_file reply marks error",
			toolName:    "read_file",
			body:        "oops",
			isError:     true,
			mustContain: []string{"read_file", "oops", "error"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatPrefixToolReply(tt.toolName, tt.body, tt.isError)
			for _, fragment := range tt.mustContain {
				require.Contains(t, got, fragment)
			}
		})
	}
}
