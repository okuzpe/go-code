package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatPrefixToolReply(t *testing.T) {
	s := FormatPrefixToolReply("bash", "hello", false)
	require.Contains(t, s, "bash")
	require.Contains(t, s, "hello")

	s2 := FormatPrefixToolReply("read_file", "oops", true)
	require.Contains(t, s2, "read_file")
	require.Contains(t, s2, "oops")
	require.Contains(t, s2, "error")
}
