package footerline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionLabel(t *testing.T) {
	require.Equal(t, "", SessionLabel(""))
	require.Equal(t, "", SessionLabel("   "))
	require.Equal(t, "#abc", SessionLabel("abc"))
	require.Equal(t, "#12345678…", SessionLabel("123456789012"))
	require.Equal(t, "#12345678…", SessionLabel("1234567890123"))
	require.Equal(t, "#你好你好你好你好", SessionLabel("你好你好你好你好"))
}

func TestJoin(t *testing.T) {
	require.Equal(t, "Working", Join("Working", "", 80))
	require.Equal(t, "#ab", Join("", "ab", 80))

	got := Join("Thinking…", "session-one", 80)
	require.Contains(t, got, "Thinking…")
	require.Contains(t, got, "#session")

	narrow := Join("abcdefghijklmnopqrstuvwxyz", "abc", 28)
	require.Contains(t, narrow, "#abc")
}

func TestJoinPrefersSessionWhenVeryNarrow(t *testing.T) {
	got := Join(strings.Repeat("x", 200), "id", 14)
	require.Contains(t, got, "#id")
	require.Contains(t, got, "…")
}

func TestHintsWithSession(t *testing.T) {
	require.Equal(t, "Enter · /help", HintsWithSession("Enter · /help", "", 80))
	require.Equal(t, "#ab", HintsWithSession("", "ab", 80))
	require.Equal(t, "Enter · /help  #ab", HintsWithSession("Enter · /help", "ab", 80))

	narrow := HintsWithSession("abcdefghijklmnopqrstuvwxyz0123456789", "abc", 28)
	require.Contains(t, narrow, "#abc")
	require.Contains(t, narrow, "\n")
}

func TestAlignedHintsSession_rightAlignsWhenRoom(t *testing.T) {
	row := AlignedHintsSession("goclaw v1", "3 msgs", "Esc · /help", "e388b3d6f0cb", 96)
	require.Contains(t, row, "goclaw v1")
	require.Contains(t, row, "3 msgs")
	require.Contains(t, row, "/help")
	require.Contains(t, row, "#e388b3d6")
}

func TestAlignedHintsSession_fallsBackWhenNarrow(t *testing.T) {
	row := AlignedHintsSession("goclaw v1", "99 msgs", "Esc · Ctrl+C×2 quit · PgUp/Dn · /help", "abc", 24)
	require.Contains(t, row, "\n")
	require.Contains(t, row, "#abc")
}

func TestIdleFooterTwoLines(t *testing.T) {
	out := IdleFooterTwoLines("goclaw v1", "3 msgs", "Esc · /help", "abcd1234", 96)
	lines := strings.Split(out, "\n")
	require.Len(t, lines, 2)
	require.Contains(t, lines[0], "goclaw v1")
	require.Contains(t, lines[0], "3 msgs")
	require.Contains(t, lines[0], "#abcd123")
	require.NotContains(t, lines[0], "/help")
	require.Contains(t, lines[1], "/help")
	require.Contains(t, lines[1], "Esc")
}

func TestTrimToMaxWidth(t *testing.T) {
	require.Equal(t, "hi", TrimToMaxWidth("hi", 10))
	require.True(t, strings.HasSuffix(TrimToMaxWidth("abcdefghijklmnop", 8), "…"))
}

func TestIdleFooterTwoLines_skipsEmptyTopRow(t *testing.T) {
	out := IdleFooterTwoLines("", "", "Esc · /help", "", 96)
	require.NotContains(t, out, "\n")
	require.Contains(t, out, "Esc")
}

func TestIdleFooterTwoLines_skipsEmptySecondRow(t *testing.T) {
	out := IdleFooterTwoLines("goclaw", "3 msgs", "", "", 40)
	require.NotContains(t, out, "\n")
	require.Contains(t, out, "3 msgs")
}
