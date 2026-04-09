package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/tools"
)

// StopTaskTool cancels an in-flight spawn_agent worker by task_id.
type StopTaskTool struct{}

var _ tools.Tool = (*StopTaskTool)(nil)

// NewStopTask returns the stop_task tool.
func NewStopTask() *StopTaskTool { return &StopTaskTool{} }

func (StopTaskTool) Name() string { return "stop_task" }

func (StopTaskTool) Description() string {
	return "Cancel a running spawn_agent worker. Use the task_id from the spawn_agent JSON result. " +
		"Has no effect if the worker already finished."
}

func (StopTaskTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{
				"type":        "string",
				"description": "task_id from the spawn_agent tool result",
			},
		},
		"required": []string{"task_id"},
	}
}

type stopTaskInput struct {
	TaskID string `json:"task_id"`
}

func (StopTaskTool) Execute(ctx context.Context, input string) (tools.Result, error) {
	_ = ctx
	var in stopTaskInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return tools.Result{Content: fmt.Sprintf("invalid json input: %v", err), IsError: true}, nil
	}
	id := strings.TrimSpace(in.TaskID)
	if id == "" {
		return tools.Result{Content: "task_id is required", IsError: true}, nil
	}
	if StopWorkerCancel(id) {
		return tools.Result{
			Content: fmt.Sprintf("cancel signal sent for task_id=%q (worker stops if still running)", id),
		}, nil
	}
	return tools.Result{
		Content: fmt.Sprintf("no active worker matched task_id=%q", id),
		IsError: true,
	}, nil
}
