package coordinator

import (
	"context"
	"sync"
	"time"
)

// Worker represents a single agent worker.
type Worker struct {
	mu        sync.RWMutex
	ID        string       `json:"id"`
	Status    AgentStatus  `json:"status"`
	Task      string       `json:"task,omitempty"`
	Result    *AgentResult `json:"result,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	StartedAt *time.Time   `json:"started_at,omitempty"`
	DoneAt    *time.Time   `json:"done_at,omitempty"`
	cancel    context.CancelFunc
}

// NewWorker creates a new worker with the given ID.
func NewWorker(id string) *Worker {
	return &Worker{
		ID:        id,
		Status:    AgentIdle,
		CreatedAt: time.Now(),
	}
}

// AssignTask marks the worker as busy and sets its task.
func (w *Worker) AssignTask(task string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.Status = AgentBusy
	w.Task = task
	now := time.Now()
	w.StartedAt = &now
}

// Complete marks the worker as done with a result.
func (w *Worker) Complete(output string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	w.DoneAt = &now
	w.Status = AgentDone
	w.Result = &AgentResult{
		AgentID:  w.ID,
		Output:   output,
		Duration: now.Sub(*w.StartedAt),
	}
}

// Fail marks the worker as failed with an error.
func (w *Worker) Fail(err string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	w.DoneAt = &now
	w.Status = AgentFailed
	w.Result = &AgentResult{
		AgentID:  w.ID,
		Error:    err,
		Duration: now.Sub(*w.StartedAt),
	}
}

// Reset resets the worker to idle state for reuse.
func (w *Worker) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.Status = AgentIdle
	w.Task = ""
	w.Result = nil
	w.StartedAt = nil
	w.DoneAt = nil
}

// Stop cancels any in-progress work.
func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
	}
	if w.Status == AgentBusy {
		w.Fail("stopped by coordinator")
	}
}

// IsIdle returns true if the worker is available for work.
func (w *Worker) IsIdle() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Status == AgentIdle
}

// GetStatus returns the current agent status.
func (w *Worker) GetStatus() AgentStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Status
}

// GetResult returns the agent result if complete.
func (w *Worker) GetResult() *AgentResult {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Result
}
