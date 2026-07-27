// SPDX-License-Identifier: MIT
package task

import (
	"container/heap"
	"sync"
)

// Queue is a thread-safe priority queue of tasks.
type Queue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  priorityHeap
	closed bool
}

// NewQueue creates an empty Queue.
func NewQueue() *Queue {
	q := &Queue{}
	q.cond = sync.NewCond(&q.mu)
	q.items = make(priorityHeap, 0)
	heap.Init(&q.items)
	return q
}

// Push adds a task to the queue.
func (q *Queue) Push(t *Task) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	heap.Push(&q.items, t)
	q.cond.Signal()
}

// Pop blocks until a task is available, then removes and returns the highest-priority task.
// Returns nil if the queue is closed.
func (q *Queue) Pop() *Task {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.items.Len() == 0 {
		if q.closed {
			return nil
		}
		q.cond.Wait()
	}
	return heap.Pop(&q.items).(*Task)
}

// TryPop returns the highest-priority task immediately, or nil if the queue is empty or closed.
func (q *Queue) TryPop() *Task {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.items.Len() == 0 || q.closed {
		return nil
	}
	return heap.Pop(&q.items).(*Task)
}

// Close wakes all blocked Pop calls and prevents further pushes.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

// Len returns the number of tasks in the queue.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.items.Len()
}

// IsEmpty returns true if the queue has no tasks.
func (q *Queue) IsEmpty() bool {
	return q.Len() == 0
}

// priorityHeap implements heap.Interface, ordering by priority (higher first).
type priorityHeap []*Task

func (h priorityHeap) Len() int { return len(h) }

func (h priorityHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority > h[j].Priority
	}
	return h[i].CreatedAt.Before(h[j].CreatedAt)
}

func (h priorityHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *priorityHeap) Push(x any) {
	*h = append(*h, x.(*Task))
}

func (h *priorityHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}
