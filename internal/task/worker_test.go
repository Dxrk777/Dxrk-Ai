// SPDX-License-Identifier: MIT
package task

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/log"
)

func TestWorkerPool_ProcessesTasks(t *testing.T) {
	q := NewQueue()
	var count atomic.Int32
	handler := func(_ context.Context, task *Task) (any, error) {
		count.Add(1)
		return "ok", nil
	}

	wp := NewWorkerPool(q, handler, WithWorkerCount(2))
	wp.Start()

	for range 5 {
		q.Push(New(TypeGeneric, Payload{}))
	}

	time.Sleep(100 * time.Millisecond)
	wp.Stop()

	if n := count.Load(); n != 5 {
		t.Fatalf("handler called %d times, want 5", n)
	}
}

func TestWorkerPool_ErrorDoesNotCrash(t *testing.T) {
	q := NewQueue()
	var count atomic.Int32
	handler := func(_ context.Context, task *Task) (any, error) {
		count.Add(1)
		return nil, assertAnError
	}

	wp := NewWorkerPool(q, handler,
		WithWorkerCount(1),
		WithLogger(log.NewSlog(slog.Default())),
	)
	wp.Start()

	q.Push(New(TypeGeneric, Payload{}))
	q.Push(New(TypeGeneric, Payload{}))
	q.Push(New(TypeGeneric, Payload{}))

	time.Sleep(100 * time.Millisecond)
	wp.Stop()

	if n := count.Load(); n != 3 {
		t.Fatalf("handler called %d times, want 3", n)
	}
}

func TestWorkerPool_StopNoTasks(t *testing.T) {
	q := NewQueue()
	wp := NewWorkerPool(q, nil)
	wp.Start()
	wp.Stop()
}

func TestWorkerPool_CancelledTaskSkipped(t *testing.T) {
	q := NewQueue()
	var count atomic.Int32
	handler := func(_ context.Context, task *Task) (any, error) {
		count.Add(1)
		return "ok", nil
	}

	wp := NewWorkerPool(q, handler, WithWorkerCount(1))
	wp.Start()

	task := New(TypeGeneric, Payload{})
	task.Cancel()
	q.Push(task)

	time.Sleep(50 * time.Millisecond)
	wp.Stop()

	if n := count.Load(); n != 0 {
		t.Fatalf("cancelled task was processed %d times, want 0", n)
	}
}

func TestWorkerPool_ActiveWorkers(t *testing.T) {
	q := NewQueue()
	block := make(chan struct{})
	handler := func(_ context.Context, task *Task) (any, error) {
		<-block
		return "ok", nil
	}

	wp := NewWorkerPool(q, handler, WithWorkerCount(2))
	wp.Start()

	q.Push(New(TypeGeneric, Payload{}))
	q.Push(New(TypeGeneric, Payload{}))

	time.Sleep(50 * time.Millisecond)

	if n := wp.ActiveWorkers(); n != 2 {
		t.Fatalf("ActiveWorkers() = %d, want 2", n)
	}

	close(block)
	time.Sleep(50 * time.Millisecond)

	if n := wp.ActiveWorkers(); n != 0 {
		t.Fatalf("ActiveWorkers() after unblock = %d, want 0", n)
	}

	wp.Stop()
}

var assertAnError = assertAnErrorFn()

func assertAnErrorFn() error { return &testError{"assert"} }

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
