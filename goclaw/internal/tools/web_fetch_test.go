package tools_test

import (
	"context"
	"testing"

	"github.com/okuzpe/goclaw/internal/tools"
)

func TestWebFetchBlocksLoopback(t *testing.T) {
	ctx := context.Background()
	tool := tools.NewWebFetch()
	res, err := tool.Execute(ctx, `{"url":"http://127.0.0.1:8080/"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected blocked URL to be rejected")
	}
}

func TestWebFetchBlocksMetadataIP(t *testing.T) {
	ctx := context.Background()
	tool := tools.NewWebFetch()
	res, err := tool.Execute(ctx, `{"url":"http://169.254.169.254/latest/meta-data/"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected metadata IP to be blocked")
	}
}

func TestWebFetchRejectsNonHTTP(t *testing.T) {
	ctx := context.Background()
	tool := tools.NewWebFetch()
	res, err := tool.Execute(ctx, `{"url":"file:///etc/passwd"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected file:// to be rejected")
	}
}
