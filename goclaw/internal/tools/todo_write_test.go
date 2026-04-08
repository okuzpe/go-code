package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/todos"
)

func TestTodoWriteToolExecute(t *testing.T) {
	st := todos.NewStore()
	tw := NewTodoWrite(st)
	raw := `{"merge":false,"todos":[{"id":"1","content":"ship feature","status":"pending"}]}`
	res, err := tw.Execute(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "ship feature") {
		t.Fatalf("result: %q", res.Content)
	}
}
