package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractAtTokens(t *testing.T) {
	// single token at start
	require.Equal(t, []string{"@go.mod"}, extractAtTokens("@go.mod"))

	// inline token
	require.Equal(t, []string{"@go.mod"}, extractAtTokens("check @go.mod for issues"))

	// multiple tokens
	require.Equal(t, []string{"@go.mod", "@README.md"}, extractAtTokens("compare @go.mod and @README.md"))

	// trailing slash kept
	require.Equal(t, []string{"@internal/"}, extractAtTokens("list @internal/ please"))

	// email-style: @ after non-whitespace is skipped
	require.Empty(t, extractAtTokens("john@example.com"))

	// no tokens
	require.Empty(t, extractAtTokens("hello world"))

	// path traversal skipped
	require.Empty(t, extractAtTokens("@../secret"))

	// absolute path skipped
	require.Empty(t, extractAtTokens("@/etc/passwd"))

	// deduplication
	toks := extractAtTokens("@go.mod and again @go.mod")
	require.Equal(t, []string{"@go.mod"}, toks)
}
