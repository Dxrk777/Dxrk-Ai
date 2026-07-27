// SPDX-License-Identifier: MIT
package task

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNewTaskID_HasPrefix(t *testing.T) {
	tests := []struct {
		typ    TaskType
		prefix string
	}{
		{TypeLocalBash, "b"},
		{TypeLocalAgent, "a"},
		{TypeDream, "d"},
		{TypeGeneric, "g"},
	}
	for _, tt := range tests {
		t.Run(string(tt.typ), func(t *testing.T) {
			id := NewTaskID(tt.typ)
			if !strings.HasPrefix(string(id), tt.prefix) {
				t.Fatalf("NewTaskID(%v) = %q, want prefix %q", tt.typ, id, tt.prefix)
			}
			if len(id) != 9 { // prefix + 8 hex chars
				t.Fatalf("NewTaskID(%v) length = %d, want 9", tt.typ, len(id))
			}
		})
	}
}

func TestNewTaskID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		id := NewTaskID(TypeGeneric)
		if seen[string(id)] {
			t.Fatalf("duplicate task ID: %q", id)
		}
		seen[string(id)] = true
	}
}

func TestTaskLifecycle(t *testing.T) {
	task := New(TypeGeneric, Payload{Data: map[string]any{"key": "val"}})
	if task.Status != StatusPending {
		t.Fatalf("initial status = %v, want pending", task.Status)
	}

	task.SetRunning()
	if task.Status != StatusRunning {
		t.Fatalf("after SetRunning status = %v, want running", task.Status)
	}

	task.Complete("done")
	if task.Status != StatusCompleted {
		t.Fatalf("after Complete status = %v, want completed", task.Status)
	}
	if task.Result != "done" {
		t.Fatalf("Result = %v, want %q", task.Result, "done")
	}
}

func TestTaskFail(t *testing.T) {
	task := New(TypeGeneric, Payload{})
	task.SetRunning()

	expected := errors.New("something broke")
	task.Fail(expected)
	if task.Status != StatusFailed {
		t.Fatalf("after Fail status = %v, want failed", task.Status)
	}
	if !errors.Is(task.Err, expected) {
		t.Fatalf("Err = %v, want %v", task.Err, expected)
	}
}

func TestTaskCancel_Pending(t *testing.T) {
	task := New(TypeGeneric, Payload{})
	task.Cancel()
	if task.Status != StatusCancelled {
		t.Fatalf("after Cancel status = %v, want cancelled", task.Status)
	}
}

func TestTaskCancel_Running(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	task := New(TypeGeneric, Payload{}, func(t *Task) { t.cancel = cancel })
	task.SetRunning()
	task.Cancel()
	if task.Status != StatusCancelled {
		t.Fatalf("after Cancel status = %v, want cancelled", task.Status)
	}
	if ctx.Err() == nil {
		t.Fatal("context was not cancelled")
	}
}

func TestTaskWait(t *testing.T) {
	task := New(TypeGeneric, Payload{})
	go func() {
		task.Complete("ok")
	}()
	task.Wait()
	if task.Status != StatusCompleted {
		t.Fatalf("after Wait status = %v, want completed", task.Status)
	}
}

func TestTaskOptions(t *testing.T) {
	task := New(TypeLocalBash, Payload{},
		WithPriority(5),
		WithMetadata("env", "prod"),
	)
	if task.Priority != 5 {
		t.Fatalf("Priority = %d, want 5", task.Priority)
	}
	md := task.Metadata()
	if md["env"] != "prod" {
		t.Fatalf("Metadata env = %q, want %q", md["env"], "prod")
	}
}

func TestTaskStatusString(t *testing.T) {
	tests := []struct {
		s    TaskStatus
		want string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusCancelled, "cancelled"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Fatalf("TaskStatus(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestTaskStatusUnknown(t *testing.T) {
	if got := TaskStatus(99).String(); got != "unknown" {
		t.Fatalf("String() = %q, want %q", got, "unknown")
	}
}
