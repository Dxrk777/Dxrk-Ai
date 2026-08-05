// SPDX-License-Identifier: MIT
package task

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/Dxrk777/Dxrk-Ai/internal/log"
	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// WorkerPool manages a configurable number of goroutines consuming tasks from a Queue.
type WorkerPool struct {
	queue   *Queue
	handler Handler
	workers int
	logger  log.Logger
	wg      sync.WaitGroup
	active  atomic.Int32
	stopped atomic.Bool
}

// WorkerPoolOption configures a WorkerPool.
type WorkerPoolOption func(*WorkerPool)

// WithLogger sets the logger for the worker pool.
func WithLogger(logger log.Logger) WorkerPoolOption {
	return func(wp *WorkerPool) { wp.logger = logger }
}

// WithWorkerCount sets the number of concurrent workers (default 4).
func WithWorkerCount(n int) WorkerPoolOption {
	return func(wp *WorkerPool) { wp.workers = n }
}

// NewWorkerPool creates a worker pool. Call Start() to begin processing.
func NewWorkerPool(queue *Queue, handler Handler, opts ...WorkerPoolOption) *WorkerPool {
	wp := &WorkerPool{
		queue:   queue,
		handler: handler,
		workers: 4,
		logger:  log.NewSlog(slog.Default()),
	}
	for _, opt := range opts {
		opt(wp)
	}
	return wp
}

// Start launches the worker goroutines and returns immediately.
func (wp *WorkerPool) Start() {
	for i := range wp.workers {
		wp.wg.Add(1)
		go wp.runWorker(i)
	}
}

// Stop signals all workers to stop by closing the queue and waits for them to finish.
func (wp *WorkerPool) Stop() {
	wp.stopped.Store(true)
	wp.queue.Close()
	wp.wg.Wait()
}

// ActiveWorkers returns the number of workers currently processing a task.
func (wp *WorkerPool) ActiveWorkers() int {
	return int(wp.active.Load())
}

// Submit adds a task to the worker pool's queue.
func (wp *WorkerPool) Submit(t *Task) {
	wp.queue.Push(t)
}

// Queue returns the underlying task queue.
func (wp *WorkerPool) Queue() *Queue {
	return wp.queue
}

func (wp *WorkerPool) runWorker(id int) {
	defer wp.wg.Done()
	logger := wp.logger.With("worker", id)

	for {
		task := wp.queue.Pop()
		if task == nil {
			// queue closed; worker exits
			return
		}

		if task.Status == StatusCancelled {
			continue
		}

		wp.active.Add(1)
		task.SetRunning()

		result, err := wp.handler(nil, task)
		if err != nil {
			task.Fail(err)
			logger.Error("task failed", strconst.StrTaskId, task.ID, "type", task.Type, strconst.StrError, err)
		} else {
			task.Complete(result)
			logger.Debug("task completed", strconst.StrTaskId, task.ID, "type", task.Type)
		}
		wp.active.Add(-1)
	}
}
