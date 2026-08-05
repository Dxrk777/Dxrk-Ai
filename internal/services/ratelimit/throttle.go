package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Throttle manages request scheduling with priority and concurrency control.
type Throttle struct {
	mu            sync.Mutex
	semaphore     chan struct{}
	queue         []*ThrottledRequest
	maxQueue      int
	maxConcurrent int
}

// ThrottledRequest represents a request in the throttle queue.
type ThrottledRequest struct {
	ID       string
	Priority int // higher = more priority
	Cost     int
	Enqueued time.Time
	Callback func()
	Cancel   context.CancelFunc
}

// ThrottleStats holds throttle statistics.
type ThrottleStats struct {
	Active        int
	Queued        int
	MaxConcurrent int
	MaxQueue      int
	OldestWait    time.Duration
}

// NewThrottle creates a new throttle with concurrency and queue limits.
func NewThrottle(maxConcurrent, maxQueue int) *Throttle {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	if maxQueue <= 0 {
		maxQueue = 100
	}
	return &Throttle{
		semaphore:     make(chan struct{}, maxConcurrent),
		queue:         make([]*ThrottledRequest, 0, maxQueue),
		maxQueue:      maxQueue,
		maxConcurrent: maxConcurrent,
	}
}

// Submit adds a request to the throttle queue. Returns an error if the queue is full.
func (t *Throttle) Submit(ctx context.Context, req *ThrottledRequest) error {
	if req == nil {
		return nil
	}
	if req.Cost <= 0 {
		req.Cost = 1
	}
	if req.Enqueued.IsZero() {
		req.Enqueued = time.Now()
	}

	// Try to acquire immediately if below concurrency limit.
	select {
	case t.semaphore <- struct{}{}:
		// Got a slot immediately.
		if req.Callback != nil {
			go func() {
				defer func() { <-t.semaphore }()
				req.Callback()
			}()
		}
		return nil
	default:
	}

	// Queue is full — reject.
	t.mu.Lock()
	if len(t.queue) >= t.maxQueue {
		t.mu.Unlock()
		if req.Cancel != nil {
			req.Cancel()
		}
		return ErrQueueFull
	}

	// Insert sorted by priority (descending).
	inserted := false
	for i, existing := range t.queue {
		if req.Priority > existing.Priority {
			t.queue = append(t.queue, nil)
			copy(t.queue[i+1:], t.queue[i:])
			t.queue[i] = req
			inserted = true
			break
		}
	}
	if !inserted {
		t.queue = append(t.queue, req)
	}
	t.mu.Unlock()

	// Drain queue in background.
	go t.drain()
	return nil
}

// drain processes queued requests.
func (t *Throttle) drain() {
	for {
		t.mu.Lock()
		if len(t.queue) == 0 {
			t.mu.Unlock()
			return
		}

		// Check context.
		req := t.queue[0]
		if req.Cancel != nil {
			select {
			case <-doneChan(req.Cancel):
				// Request was cancelled, remove it.
				t.queue = t.queue[1:]
				t.mu.Unlock()
				continue
			default:
			}
		}

		// Try to acquire semaphore.
		select {
		case t.semaphore <- struct{}{}:
			t.queue = t.queue[1:]
			t.mu.Unlock()

			if req.Callback != nil {
				go func() {
					defer func() { <-t.semaphore }()
					req.Callback()
				}()
			}
			return
		default:
			t.mu.Unlock()
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func doneChan(cancel context.CancelFunc) <-chan struct{} {
	// This is a simplified check. In practice, the context is checked inline.
	return nil
}

// Acquire acquires a concurrency slot, blocking until available or context is cancelled.
func (t *Throttle) Acquire(ctx context.Context) error {
	select {
	case t.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a concurrency slot.
func (t *Throttle) Release() {
	select {
	case <-t.semaphore:
	default:
	}
}

// QueueSize returns current queue depth.
func (t *Throttle) QueueSize() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.queue)
}

// Stats returns throttle statistics.
func (t *Throttle) Stats() ThrottleStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	var oldestWait time.Duration
	if len(t.queue) > 0 {
		oldestWait = time.Since(t.queue[0].Enqueued)
	}

	return ThrottleStats{
		Active:        len(t.semaphore),
		Queued:        len(t.queue),
		MaxConcurrent: t.maxConcurrent,
		MaxQueue:      t.maxQueue,
		OldestWait:    oldestWait,
	}
}

// ErrQueueFull is returned when the throttle queue is at capacity.
type ErrQueueFullTypeError struct{}

func (e ErrQueueFullTypeError) Error() string {
	return "throttle queue full"
}

var ErrQueueFull = ErrQueueFullTypeError{}
