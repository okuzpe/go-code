package tools

import (
	"context"
	"testing"
 
	"github.com/stretchr/testify/require"
)

func TestWebFetchBlocksLoopback(t *testing.T) {
	ctx := context.Background()
	tool := NewWebFetch()
	res, err := tool.Execute(ctx, `{"url":"http://127.0.0.1:8080/"}`)
	require.NoError(t, err)
	require.True(t, res.IsError, "expected blocked URL to be rejected")
}

func TestWebFetchBlocksMetadataIP(t *testing.T) {
	ctx := context.Background()
	tool := NewWebFetch()
	res, err := tool.Execute(ctx, `{"url":"http://169.254.169.254/latest/meta-data/"}`)
	require.NoError(t, err)
	require.True(t, res.IsError, "expected metadata IP to be blocked")
}

func TestWebFetchRejectsNonHTTP(t *testing.T) {
	ctx := context.Background()
	tool := NewWebFetch()
	res, err := tool.Execute(ctx, `{"url":"file:///etc/passwd"}`)
	require.NoError(t, err)
	require.True(t, res.IsError, "expected file:// to be rejected")
}
