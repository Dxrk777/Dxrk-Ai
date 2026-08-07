// SPDX-License-Identifier: MIT
// Package task provides an async task system with queue and worker pool.
//
// Ported from Claude Code's task system (see references/claude-code-source/src/Task.ts):
//   - Task with typed ID, status lifecycle, and metadata
//   - Thread-safe FIFO queue with optional priority
//   - Configurable worker pool
package task

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// TaskType classifies tasks for routing and rendering.
type TaskType string

const (
	TypeLocalBash  TaskType = "local_bash"
	TypeLocalAgent TaskType = "local_agent"
	TypeDream      TaskType = "dream"
	TypeGeneric    TaskType = "generic"
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus int

const (
	StatusPending TaskStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCancelled
)

func (s TaskStatus) String() string {
	switch s {
	case StatusPending:
		return strconst.StrPending
	case StatusRunning:
		return strconst.StrRunning
	case StatusCompleted:
		return strconst.StrCompleted
	case StatusFailed:
		return strconst.StrFailed
	case StatusCancelled:
		return strconst.StrCancelled
	default:
		return strconst.StrUnknown
	}
}

// TaskID generates typed IDs like "b3a1f2..." (prefix + 8 random hex chars).
type TaskID string

// NewTaskID creates a typed task ID.
func NewTaskID(typ TaskType) TaskID {
	prefix := "g"
	switch typ {
	case TypeLocalBash:
		prefix = "b"
	case TypeLocalAgent:
		prefix = "a"
	case TypeDream:
		prefix = "d"
	}
	b := make([]byte, 4)
	rand.Read(b)
	return TaskID(fmt.Sprintf("%s%s", prefix, hex.EncodeToString(b)))
}

// Payload is user-defined data carried by a task.
type Payload struct {
	Data map[string]any
}

// Task represents an asynchronous unit of work.
type Task struct {
	ID        TaskID
	Type      TaskType
	Status    TaskStatus
	Priority  int // higher = sooner
	Payload   Payload
	Result    any
	Err       error
	CreatedAt time.Time
	UpdatedAt time.Time

	mu       sync.Mutex
	cond     *sync.Cond
	cancel   context.CancelFunc
	metadata map[string]string
}

// New creates a Task with pending status and metadata.
func New(typ TaskType, payload Payload, opts ...Option) *Task {
	t := &Task{
		ID:        NewTaskID(typ),
		Type:      typ,
		Status:    StatusPending,
		Priority:  0,
		Payload:   payload,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		metadata:  make(map[string]string),
	}
	t.cond = sync.NewCond(&t.mu)
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Option configures a Task.
type Option func(*Task)

// WithPriority sets the task priority (higher = sooner).
func WithPriority(p int) Option { return func(t *Task) { t.Priority = p } }

// WithMetadata sets a metadata key-value pair.
func WithMetadata(key, value string) Option {
	return func(t *Task) { t.metadata[key] = value }
}

// SetRunning transitions the task to running status.
func (t *Task) SetRunning() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = StatusRunning
	t.UpdatedAt = time.Now()
}

// Complete marks the task as completed with a result.
func (t *Task) Complete(result any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = StatusCompleted
	t.Result = result
	t.UpdatedAt = time.Now()
	t.cond.Broadcast()
}

// Fail marks the task as failed with an error.
func (t *Task) Fail(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = StatusFailed
	t.Err = err
	t.UpdatedAt = time.Now()
	t.cond.Broadcast()
}

// Cancel marks the task as cancelled.
func (t *Task) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Status == StatusPending || t.Status == StatusRunning {
		t.Status = StatusCancelled
		t.UpdatedAt = time.Now()
		if t.cancel != nil {
			t.cancel()
		}
		t.cond.Broadcast()
	}
}

// Wait blocks until the task reaches a terminal state.
func (t *Task) Wait() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for t.Status != StatusCompleted && t.Status != StatusFailed && t.Status != StatusCancelled {
		t.cond.Wait()
	}
}

// Metadata returns a copy of the task's metadata.
func (t *Task) Metadata() map[string]string {
	t.mu.Lock()
	defer t.mu.Unlock()
	cpy := make(map[string]string, len(t.metadata))
	for k, v := range t.metadata {
		cpy[k] = v
	}
	return cpy
}

// Handler processes a task and returns a result or error.
type Handler func(context.Context, *Task) (any, error)
