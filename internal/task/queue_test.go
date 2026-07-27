// SPDX-License-Identifier: MIT
package task

import (
	"testing"
)

func TestQueue_PushPop(t *testing.T) {
	q := NewQueue()
	task := New(TypeGeneric, Payload{})
	q.Push(task)

	popped := q.Pop()
	if popped.ID != task.ID {
		t.Fatalf("Pop() returned task %q, want %q", popped.ID, task.ID)
	}
}

func TestQueue_Empty(t *testing.T) {
	q := NewQueue()
	if !q.IsEmpty() {
		t.Fatal("new queue should be empty")
	}
	q.Push(New(TypeGeneric, Payload{}))
	if q.IsEmpty() {
		t.Fatal("queue with 1 item should not be empty")
	}
}

func TestQueue_TryPop(t *testing.T) {
	q := NewQueue()
	if got := q.TryPop(); got != nil {
		t.Fatalf("TryPop() on empty queue = %v, want nil", got)
	}

	task := New(TypeGeneric, Payload{})
	q.Push(task)
	popped := q.TryPop()
	if popped == nil {
		t.Fatal("TryPop() on non-empty queue returned nil")
	}
	if popped.ID != task.ID {
		t.Fatalf("TryPop() = %q, want %q", popped.ID, task.ID)
	}
}

func TestQueue_Priority(t *testing.T) {
	q := NewQueue()
	low := New(TypeGeneric, Payload{}, WithPriority(1))
	high := New(TypeGeneric, Payload{}, WithPriority(10))
	medium := New(TypeGeneric, Payload{}, WithPriority(5))

	q.Push(low)
	q.Push(high)
	q.Push(medium)

	if got := q.Pop(); got.ID != high.ID {
		t.Fatalf("expected high-priority task first, got %q", got.ID)
	}
	if got := q.Pop(); got.ID != medium.ID {
		t.Fatalf("expected medium-priority task second, got %q", got.ID)
	}
	if got := q.Pop(); got.ID != low.ID {
		t.Fatalf("expected low-priority task last, got %q", got.ID)
	}
}

func TestQueue_FIFOWithinPriority(t *testing.T) {
	q := NewQueue()
	t1 := New(TypeGeneric, Payload{}, WithPriority(0))
	t2 := New(TypeGeneric, Payload{}, WithPriority(0))
	t3 := New(TypeGeneric, Payload{}, WithPriority(0))

	q.Push(t1)
	q.Push(t2)
	q.Push(t3)

	if got := q.Pop(); got.ID != t1.ID {
		t.Fatalf("expected FIFO: t1 first, got %q", got.ID)
	}
	if got := q.Pop(); got.ID != t2.ID {
		t.Fatalf("expected FIFO: t2 second, got %q", got.ID)
	}
	if got := q.Pop(); got.ID != t3.ID {
		t.Fatalf("expected FIFO: t3 third, got %q", got.ID)
	}
}

func TestQueue_Len(t *testing.T) {
	q := NewQueue()
	if q.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", q.Len())
	}

	q.Push(New(TypeGeneric, Payload{}))
	q.Push(New(TypeGeneric, Payload{}))
	if q.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", q.Len())
	}

	q.Pop()
	if q.Len() != 1 {
		t.Fatalf("Len() after Pop() = %d, want 1", q.Len())
	}
}
