package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/stretchr/testify/require"
)

func TestTodoWriteToolExecute(t *testing.T) {
	st := todos.NewStore()
	tw, err := NewTodoWrite(st)
	require.NoError(t, err)
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

func TestTodoWriteRejectsTooManyItems(t *testing.T) {
	st := todos.NewStore()
	tw, err := NewTodoWrite(st)
	require.NoError(t, err)
	todosJSON := make([]map[string]any, todos.MaxItems+1)
	for i := 0; i < len(todosJSON); i++ {
		todosJSON[i] = map[string]any{
			"id":      fmt.Sprintf("id-%d", i),
			"content": "item",
			"status":  "pending",
		}
	}
	payload, err := json.Marshal(map[string]any{"merge": false, "todos": todosJSON})
	require.NoError(t, err)
	res, err := tw.Execute(context.Background(), string(payload))
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "50")
}

func TestTodoWriteRejectsContentOverRuneLimit(t *testing.T) {
	st := todos.NewStore()
	tw, err := NewTodoWrite(st)
	require.NoError(t, err)
	long := strings.Repeat("a", todos.MaxContentRunes+1)
	payload, err := json.Marshal(map[string]any{
		"merge": false,
		"todos": []map[string]any{{
			"id": "1", "content": long, "status": "pending",
		}},
	})
	require.NoError(t, err)
	res, err := tw.Execute(context.Background(), string(payload))
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Content, "500")
}
