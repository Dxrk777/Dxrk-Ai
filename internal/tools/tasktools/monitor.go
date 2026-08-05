package tasktools

import (
	"fmt"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// TaskEventType identifies the kind of task event.
type TaskEventType int

const (
	EventCreated TaskEventType = iota
	EventStarted
	EventProgress
	EventCompleted
	EventFailed
	EventCancelled
	EventTimeout
	EventOutput
)

func (e TaskEventType) String() string {
	switch e {
	case EventCreated:
		return "created"
	case EventStarted:
		return "started"
	case EventProgress:
		return strconst.StrProgress
	case EventCompleted:
		return strconst.StrCompleted
	case EventFailed:
		return strconst.StrFailed
	case EventCancelled:
		return strconst.StrCancelled
	case EventTimeout:
		return strconst.StrTimeout
	case EventOutput:
		return "output"
	default:
		return strconst.StrUnknown
	}
}

// ParseTaskEventType converts a string to TaskEventType.
func ParseTaskEventType(s string) (TaskEventType, error) {
	switch s {
	case "created":
		return EventCreated, nil
	case "started":
		return EventStarted, nil
	case strconst.StrProgress:
		return EventProgress, nil
	case strconst.StrCompleted:
		return EventCompleted, nil
	case strconst.StrFailed:
		return EventFailed, nil
	case strconst.StrCancelled:
		return EventCancelled, nil
	case strconst.StrTimeout:
		return EventTimeout, nil
	case "output":
		return EventOutput, nil
	default:
		return EventCreated, fmt.Errorf("unknown event type: %q", s)
	}
}

// TaskEvent represents a lifecycle event for a task.
type TaskEvent struct {
	TaskID    string         `json:"task_id"`
	EventType TaskEventType  `json:"event_type"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

// TaskMonitor dispatches task events to subscribers.
type TaskMonitor struct {
	mu        sync.RWMutex
	listeners map[<-chan TaskEvent]chan TaskEvent
	eventBuf  int
}

// NewTaskMonitor creates a new event monitor.
func NewTaskMonitor() *TaskMonitor {
	return &TaskMonitor{
		listeners: make(map[<-chan TaskEvent]chan TaskEvent),
		eventBuf:  64,
	}
}

// Subscribe returns a channel that receives events of the given types.
// If no types are specified, all events are received.
func (m *TaskMonitor) Subscribe(eventTypes ...TaskEventType) <-chan TaskEvent {
	ch := make(chan TaskEvent, m.eventBuf)
	m.mu.Lock()
	m.listeners[ch] = ch
	m.mu.Unlock()

	if len(eventTypes) == 0 {
		return ch
	}

	filtered := make(chan TaskEvent, m.eventBuf)
	go func() {
		defer close(filtered)
		for ev := range ch {
			for _, t := range eventTypes {
				if ev.EventType == t {
					filtered <- ev
					break
				}
			}
		}
	}()

	m.mu.Lock()
	delete(m.listeners, ch)
	m.listeners[filtered] = filtered
	m.mu.Unlock()

	return filtered
}

// Unsubscribe removes a listener channel.
func (m *TaskMonitor) Unsubscribe(ch <-chan TaskEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if internal, ok := m.listeners[ch]; ok {
		close(internal)
		delete(m.listeners, ch)
	}
}

// Notify sends an event to all subscribers.
func (m *TaskMonitor) Notify(event TaskEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.listeners {
		select {
		case ch <- event:
		default:
		}
	}
}

// WatchTask returns a channel that emits events for a specific task.
func (m *TaskMonitor) WatchTask(taskID string) <-chan TaskEvent {
	ch := make(chan TaskEvent, m.eventBuf)
	m.mu.Lock()
	m.listeners[ch] = ch
	m.mu.Unlock()

	filtered := make(chan TaskEvent, m.eventBuf)
	go func() {
		defer close(filtered)
		for ev := range ch {
			if ev.TaskID == taskID {
				filtered <- ev
			}
		}
	}()

	m.mu.Lock()
	delete(m.listeners, ch)
	m.listeners[filtered] = filtered
	m.mu.Unlock()

	return filtered
}

// WaitFor blocks until the given task reaches a terminal state or times out.
func (m *TaskMonitor) WaitFor(taskID string, timeout time.Duration) (*Task, error) {
	deadline := time.After(timeout)
	watchCh := m.WatchTask(taskID)

	for {
		select {
		case ev, ok := <-watchCh:
			if !ok {
				return nil, fmt.Errorf("watch channel closed for task %q", taskID)
			}
			switch ev.EventType {
			case EventCompleted, EventFailed, EventCancelled, EventTimeout:
				result, _ := ev.Data["task"].(*Task)
				if result != nil {
					return result, nil
				}
				return nil, fmt.Errorf("task %q reached terminal state %s", taskID, ev.EventType)
			}
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for task %q", taskID)
		}
	}
}

// WaitForAll blocks until all listed tasks reach terminal states or times out.
func (m *TaskMonitor) WaitForAll(taskIDs []string, timeout time.Duration) error {
	deadline := time.After(timeout)
	watchCh := make(chan TaskEvent, 64)
	m.mu.Lock()
	m.listeners[watchCh] = watchCh
	m.mu.Unlock()

	remaining := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		remaining[id] = true
	}

	for len(remaining) > 0 {
		select {
		case ev, ok := <-watchCh:
			if !ok {
				return fmt.Errorf("monitor channel closed")
			}
			if remaining[ev.TaskID] {
				switch ev.EventType {
				case EventCompleted, EventFailed, EventCancelled, EventTimeout:
					delete(remaining, ev.TaskID)
				}
			}
		case <-deadline:
			return fmt.Errorf("timeout waiting for tasks: remaining=%d", len(remaining))
		}
	}

	return nil
}
