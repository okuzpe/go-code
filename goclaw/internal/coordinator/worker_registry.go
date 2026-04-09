package coordinator

import (
	"context"
	"sync"
)

// workerCancels holds context.CancelFunc values for in-flight spawn_agent workers,
// keyed by the worker session id (task_id in tool results).
var workerCancels sync.Map // string -> context.CancelFunc

func registerWorkerCancel(taskID string, cancel context.CancelFunc) {
	if taskID == "" || cancel == nil {
		return
	}
	workerCancels.Store(taskID, cancel)
}

func unregisterWorkerCancel(taskID string) {
	workerCancels.Delete(taskID)
}

// StopWorkerCancel cancels a running worker by task id. Returns false if none matched.
func StopWorkerCancel(taskID string) bool {
	if taskID == "" {
		return false
	}
	v, ok := workerCancels.LoadAndDelete(taskID)
	if !ok {
		return false
	}
	fn, _ := v.(context.CancelFunc)
	if fn != nil {
		fn()
	}
	return true
}
