package hooks

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrQueueClosed   = errors.New("hooks: queue is closed")
	ErrQueueFull     = errors.New("hooks: queue is full")
	ErrWorkerStopped = errors.New("hooks: worker stopped")
)

// HookTask represents a hook execution task.
type HookTask struct {
	Event   HookEvent
	Config  HookConfig
	Result  chan<- HookResult
	Context context.Context
	Cancel  context.CancelFunc
}

// HookQueue manages async hook execution with a worker pool.
type HookQueue struct {
	mu        sync.Mutex
	tasks     chan *HookTask
	workers   int
	workerWG  sync.WaitGroup
	closed    bool
	executor  *HookExecutor
	processed uint64
	failed    uint64
	startedAt time.Time
}

// HookQueueOption configures a HookQueue.
type HookQueueOption func(*HookQueue)

// WithQueueWorkers sets the number of worker goroutines.
func WithQueueWorkers(n int) HookQueueOption {
	return func(q *HookQueue) { q.workers = n }
}

// WithQueueExecutor sets a custom executor.
func WithQueueExecutor(e *HookExecutor) HookQueueOption {
	return func(q *HookQueue) { q.executor = e }
}

// WithQueueBuffer sets the task channel buffer size.
func WithQueueBuffer(n int) HookQueueOption {
	return func(q *HookQueue) {
		if n > 0 {
			close(q.tasks)
			q.tasks = make(chan *HookTask, n)
		}
	}
}

// NewHookQueue creates a new hook queue with worker pool.
func NewHookQueue(opts ...HookQueueOption) *HookQueue {
	q := &HookQueue{
		tasks:     make(chan *HookTask, 100),
		workers:   4,
		executor:  NewHookExecutor(),
		startedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(q)
	}
	return q
}

// Start launches the worker pool.
func (q *HookQueue) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	for i := 0; i < q.workers; i++ {
		q.workerWG.Add(1)
		go q.worker(i)
	}
}

// Stop gracefully shuts down the queue.
func (q *HookQueue) Stop(ctx context.Context) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	close(q.tasks)
	q.mu.Unlock()

	done := make(chan struct{})
	go func() {
		q.workerWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Submit adds a task to the queue. Returns ErrQueueFull if buffer is full.
func (q *HookQueue) Submit(ctx context.Context, event HookEvent, config HookConfig) (HookResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	resultCh := make(chan HookResult, 1)

	task := &HookTask{
		Event:   event,
		Config:  config,
		Result:  resultCh,
		Context: ctx,
		Cancel:  cancel,
	}

	select {
	case q.tasks <- task:
		atomic.AddUint64(&q.processed, 1)
	case <-ctx.Done():
		cancel()
		return HookResult{}, ctx.Err()
	default:
		cancel()
		return HookResult{}, ErrQueueFull
	}

	select {
	case result := <-resultCh:
		if !result.Success {
			atomic.AddUint64(&q.failed, 1)
		}
		return result, nil
	case <-ctx.Done():
		cancel()
		return HookResult{}, ctx.Err()
	}
}

// SubmitAsync adds a task without waiting for result.
func (q *HookQueue) SubmitAsync(event HookEvent, config HookConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan HookResult, 1)

	task := &HookTask{
		Event:   event,
		Config:  config,
		Result:  resultCh,
		Context: ctx,
		Cancel:  cancel,
	}

	select {
	case q.tasks <- task:
		atomic.AddUint64(&q.processed, 1)
		go func() {
			<-resultCh
		}()
		return nil
	default:
		cancel()
		return ErrQueueFull
	}
}

// Stats returns queue statistics.
func (q *HookQueue) Stats() QueueStats {
	return QueueStats{
		Processed: atomic.LoadUint64(&q.processed),
		Failed:    atomic.LoadUint64(&q.failed),
		Workers:   q.workers,
		QueueLen:  len(q.tasks),
		QueueCap:  cap(q.tasks),
		Uptime:    time.Since(q.startedAt),
	}
}

// QueueStats holds queue statistics.
type QueueStats struct {
	Processed uint64
	Failed    uint64
	Workers   int
	QueueLen  int
	QueueCap  int
	Uptime    time.Duration
}

func (q *HookQueue) worker(_ int) {
	defer q.workerWG.Done()

	for task := range q.tasks {
		if task.Context.Err() != nil {
			continue
		}

		result := q.executor.Execute(task.Context, task.Config, task.Event)

		select {
		case task.Result <- result:
		case <-task.Context.Done():
		}
	}
}

// IsRunning returns true if the queue is running.
func (q *HookQueue) IsRunning() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return !q.closed
}
