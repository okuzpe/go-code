package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/okuzpe/goclaw/internal/todos"
)

// TodoWriteTool updates the session-scoped task list (injected into the system prompt).
type TodoWriteTool struct {
	store *todos.Store
}

var _ Tool = (*TodoWriteTool)(nil)

// NewTodoWrite returns a tool bound to store (must be non-nil).
func NewTodoWrite(store *todos.Store) (*TodoWriteTool, error) {
	if store == nil {
		return nil, fmt.Errorf("goclaw/tools: NewTodoWrite(nil)")
	}
	return &TodoWriteTool{store: store}, nil
}

func (t *TodoWriteTool) Name() string { return "todo_write" }

func (t *TodoWriteTool) Description() string {
	return "Update the session task list shown in the system prompt. " +
		"Use merge:true to upsert by id; merge:false to replace the whole list. " +
		"Statuses: pending, in_progress, completed. " +
		fmt.Sprintf("At most %d items; content max %d runes each.", todos.MaxItems, todos.MaxContentRunes)
}

func (t *TodoWriteTool) InputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"todos"},
		"properties": map[string]any{
			"merge": map[string]any{
				"type":        "boolean",
				"description": "If true, update existing ids and append new ones; if false, replace the entire list.",
			},
			"todos": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "content", "status"},
					"properties": map[string]any{
						"id":      map[string]any{"type": "string", "description": "Stable identifier for this task."},
						"content": map[string]any{"type": "string", "description": "Short description of the task."},
						"status":  map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
					},
				},
			},
		},
	}
}

func (t *TodoWriteTool) Execute(ctx context.Context, input string) (Result, error) {
	_ = ctx
	if err := json.Unmarshal([]byte(input), new(json.RawMessage)); err != nil {
		return Result{Content: fmt.Sprintf("invalid JSON: %v", err), IsError: true}, nil
	}
	if err := t.store.Apply(input); err != nil {
		return Result{Content: err.Error(), IsError: true}, nil
	}
	snap := t.store.FormatForPrompt()
	if snap == "" {
		return Result{Content: "task list updated (empty)"}, nil
	}
	return Result{Content: "task list updated:\n" + snap}, nil
}
