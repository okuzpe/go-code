package coordinator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStopTaskUnknownID(t *testing.T) {
	tool := NewStopTask()
	res, err := tool.Execute(context.Background(), `{"task_id":"nonexistent-task-id"}`)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestStopTaskCancelsRegisteredWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	taskID := "test-stop-task-id"
	registerWorkerCancel(taskID, cancel)

	tool := NewStopTask()
	res, err := tool.Execute(context.Background(), `{"task_id":"`+taskID+`"}`)
	require.NoError(t, err)
	require.False(t, res.IsError)

	select {
	case <-ctx.Done():
	default:
		t.Fatal("expected worker context canceled")
	}
}
