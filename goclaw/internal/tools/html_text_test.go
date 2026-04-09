package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsLikelyHTMLResponse(t *testing.T) {
	require.True(t, isLikelyHTMLResponse("text/html; charset=utf-8", []byte("x")))
	require.True(t, isLikelyHTMLResponse("", []byte("<!DOCTYPE html><html>")))
	require.False(t, isLikelyHTMLResponse("text/plain", []byte("just text")))
}

func TestHTMLResponseToPlainText(t *testing.T) {
	raw := `<!DOCTYPE html><html><head><title>News</title><script>evil()</script><style>.x{}</style></head>` +
		`<body><p>First paragraph alpha beta.</p><p>Second gamma delta epsilon.</p></body></html>`
	out, ok := htmlResponseToPlainText([]byte(raw))
	require.True(t, ok)
	require.Contains(t, out, "First paragraph")
	require.Contains(t, out, "gamma delta")
	require.NotContains(t, strings.ToLower(out), "evil")
	require.NotContains(t, out, "<p>")
}

func TestHTMLResponseToPlainTextShortYieldsFalse(t *testing.T) {
	_, ok := htmlResponseToPlainText([]byte("<html><body><p>Hi</p></body></html>"))
	require.False(t, ok)
}

func TestNormalizeFetchedWhitespace(t *testing.T) {
	got := normalizeFetchedWhitespace("  a  b  \n\n\n  c  ")
	require.Equal(t, "a b\nc", got)
}
