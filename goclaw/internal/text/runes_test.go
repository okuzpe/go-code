package text

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTruncateRunes(t *testing.T) {
	require.Equal(t, "", TruncateRunes("", 5))
	require.Equal(t, "abcde", TruncateRunes("abcde", 5))
	require.Equal(t, "abcde…", TruncateRunes("abcdef", 5))
	require.Equal(t, "你好世…", TruncateRunes("你好世界", 3))
}
