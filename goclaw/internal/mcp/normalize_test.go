package mcp

import (
	"strings"
	"testing"
 
	"github.com/stretchr/testify/require"
)

func TestNormalizeMCPToolName(t *testing.T) {
	got := NormalizeMCPToolName("my-server", "do_thing")
	require.Equal(t, "mcp__my-server__do_thing", got)

	got2 := NormalizeMCPToolName("a/b", "x")
	require.True(t, strings.HasPrefix(got2, "mcp__"))
	require.Contains(t, got2, "__x")
}
