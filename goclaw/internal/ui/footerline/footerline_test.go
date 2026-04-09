package footerline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionLabel(t *testing.T) {
	require.Equal(t, "", SessionLabel(""))
	require.Equal(t, "", SessionLabel("   "))
	require.Equal(t, "sess·abc", SessionLabel("abc"))
	require.Equal(t, "sess·123456789012", SessionLabel("123456789012"))
	require.Equal(t, "sess·123456789012…", SessionLabel("1234567890123"))
	require.Equal(t, "sess·你好你好你好你好", SessionLabel("你好你好你好你好"))
}

func TestJoin(t *testing.T) {
	require.Equal(t, "Working", Join("Working", "", 80))
	require.Equal(t, "sess·ab", Join("", "ab", 80))

	got := Join("Thinking…", "session-one", 80)
	require.Contains(t, got, "Thinking…")
	require.Contains(t, got, "sess·session-one")

	narrow := Join("abcdefghijklmnopqrstuvwxyz", "abc", 28)
	require.Contains(t, narrow, "sess·abc")
}

func TestJoinPrefersSessionWhenVeryNarrow(t *testing.T) {
	require.Equal(t, "sess·id", Join(strings.Repeat("x", 200), "id", 14))
}
