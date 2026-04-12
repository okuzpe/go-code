package coordinator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/okuzpe/goclaw/internal/orchestrator"
)

// ErrInteractiveWorkerNotFound is returned when DeliverWorkerMessage targets a task_id
// that is not a running interactive worker (e.g. worker finished and was unregistered).
var ErrInteractiveWorkerNotFound = errors.New("interactive worker not found")

// workerJob is one user message delivered to an interactive worker's loop.
type workerJob struct {
	Text string
	Sink orchestrator.StreamSink
	Done chan error
}

// interactiveWorker holds runtime state for spawn_agent(interactive: true).
type interactiveWorker struct {
	taskID  string
	profile string
	inbox   chan workerJob
	mu      sync.Mutex
	summary string
	result  string
	status  string // running | completed | failed
}

var interactiveReg sync.Map // string(taskID) -> *interactiveWorker

func storeInteractive(w *interactiveWorker) {
	interactiveReg.Store(w.taskID, w)
}

func deleteInteractive(taskID string) {
	interactiveReg.Delete(taskID)
}

func loadInteractive(taskID string) (*interactiveWorker, bool) {
	v, ok := interactiveReg.Load(taskID)
	if !ok {
		return nil, false
	}
	w, _ := v.(*interactiveWorker)
	return w, w != nil
}

// ListInteractiveWorkers returns a snapshot of running interactive workers.
func ListInteractiveWorkers() []InteractiveWorkerInfo {
	out := make([]InteractiveWorkerInfo, 0, 8)
	interactiveReg.Range(func(k, v any) bool {
		id, _ := k.(string)
		w, _ := v.(*interactiveWorker)
		if w == nil {
			return true
		}
		w.mu.Lock()
		out = append(out, InteractiveWorkerInfo{
			TaskID:  id,
			Profile: w.profile,
			Status:  w.status,
			Summary: w.summary,
		})
		w.mu.Unlock()
		return true
	})
	return out
}

// InteractiveWorkerInfo is a row for /workers and debugging.
type InteractiveWorkerInfo struct {
	TaskID  string
	Profile string
	Status  string
	Summary string
}

// ResolveInteractiveTaskID returns the full task id if prefix uniquely matches one interactive worker.
func ResolveInteractiveTaskID(prefix string) (full string, ok bool) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", false
	}
	matches := make([]string, 0, 4)
	interactiveReg.Range(func(k, _ any) bool {
		id, _ := k.(string)
		if strings.HasPrefix(id, prefix) {
			matches = append(matches, id)
		}
		return true
	})
	if len(matches) != 1 {
		return "", false
	}
	return matches[0], true
}

// DeliverWorkerMessage enqueues one user turn for the worker and blocks until it finishes or ctx ends.
func DeliverWorkerMessage(ctx context.Context, taskID, text string, sink orchestrator.StreamSink) error {
	id := strings.TrimSpace(taskID)
	w, ok := loadInteractive(id)
	if !ok {
		return fmt.Errorf("%w: no interactive worker with task_id %q (use /workers)", ErrInteractiveWorkerNotFound, id)
	}
	t := strings.TrimSpace(text)
	if t == "" {
		return fmt.Errorf("empty message")
	}
	done := make(chan error, 1)
	job := workerJob{Text: t, Sink: sink, Done: done}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case w.inbox <- job:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// Snapshot returns the accumulated result text from the most recent worker turn.
func (w *interactiveWorker) Snapshot() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.result
}

// SnapshotInteractiveWorker returns the most recent result text for the interactive worker
// identified by taskID, or ("", false) if the worker is not found.
func SnapshotInteractiveWorker(taskID string) (string, bool) {
	w, ok := loadInteractive(strings.TrimSpace(taskID))
	if !ok {
		return "", false
	}
	return w.Snapshot(), true
}

func (w *interactiveWorker) setState(summary, result, status string) {
	w.mu.Lock()
	w.summary = summary
	w.result = result
	w.status = status
	w.mu.Unlock()
}

func runInteractiveWorkerLoop(
	ctx context.Context,
	cancel context.CancelFunc,
	w *interactiveWorker,
	orch *orchestrator.Orchestrator,
	initialTask string,
	initialSink orchestrator.StreamSink,
) {
	defer cancel()
	defer unregisterWorkerCancel(w.taskID)
	defer deleteInteractive(w.taskID)

	runTurn := func(userText string, sink orchestrator.StreamSink) error {
		res, err := orch.RunStreaming(ctx, userText, wrapNestedWorkerSink(sink))
		if err != nil {
			w.setState(fmt.Sprintf("error: %v", err), res, "failed")
			return err
		}
		w.setState(firstNonEmptyLine(res), res, "running")
		return nil
	}

	if err := runTurn(initialTask, initialSink); err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			w.setState("stopped", "", "failed")
			return
		case job := <-w.inbox:
			err := runTurn(job.Text, job.Sink)
			if job.Done != nil {
				select {
				case job.Done <- err:
				default:
				}
			}
		}
	}
}
