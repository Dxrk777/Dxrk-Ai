package tasks

import (
	"fmt"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

type TaskType string

const (
	DreamTask             TaskType = "dream"
	InProcessTeammateTask TaskType = "in_process_teammate"
	LocalAgentTask        TaskType = "local_agent"
	LocalShellTask        TaskType = "local_shell"
	RemoteAgentTask       TaskType = "remote_agent"
	LocalWorkflowTask     TaskType = "local_workflow"
	MonitorMcpTask        TaskType = "monitor_mcp"
)

type TaskState string

const (
	TaskPending   TaskState = strconst.StrPending
	TaskRunning   TaskState = strconst.StrRunning
	TaskCompleted TaskState = strconst.StrCompleted
	TaskFailed    TaskState = strconst.StrFailed
	TaskCancelled TaskState = strconst.StrCancelled
)

type Task struct {
	ID          string
	Type        TaskType
	State       TaskState
	Description string
	Result      string
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	Error       error
	Metadata    map[string]string
}

type TaskRegistry struct {
	mu       sync.Mutex
	tasks    map[string]*Task
	maxTasks int
}

func NewTaskRegistry(maxTasks int) *TaskRegistry {
	if maxTasks <= 0 {
		maxTasks = 1000
	}
	return &TaskRegistry{
		tasks:    make(map[string]*Task),
		maxTasks: maxTasks,
	}
}

func (r *TaskRegistry) CreateTask(taskType TaskType, description string) *Task {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.tasks) >= r.maxTasks {
		r.evictOldest()
	}

	id := fmt.Sprintf("%s-%d", taskType, time.Now().UnixNano())
	task := &Task{
		ID:          id,
		Type:        taskType,
		State:       TaskPending,
		Description: description,
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]string),
	}
	r.tasks[id] = task
	return task
}

func (r *TaskRegistry) GetTask(id string) *Task {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tasks[id]
}

func (r *TaskRegistry) UpdateState(id string, state TaskState, result string, err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[id]
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}

	task.State = state
	if result != "" {
		task.Result = result
	}
	if err != nil {
		task.Error = err
	}

	now := time.Now()
	switch state {
	case TaskRunning:
		if task.StartedAt.IsZero() {
			task.StartedAt = now
		}
	case TaskCompleted, TaskFailed, TaskCancelled:
		task.CompletedAt = now
	}

	return nil
}

func (r *TaskRegistry) ListTasks(taskType TaskType) []*Task {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result []*Task
	for _, task := range r.tasks {
		if taskType == "" || task.Type == taskType {
			result = append(result, task)
		}
	}
	return result
}

func (r *TaskRegistry) CancelTask(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[id]
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}

	if task.State == TaskCompleted || task.State == TaskFailed || task.State == TaskCancelled {
		return fmt.Errorf("task %s is already in terminal state %s", id, task.State)
	}

	task.State = TaskCancelled
	task.CompletedAt = time.Now()
	return nil
}

func (r *TaskRegistry) Cleanup(maxAge time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	count := 0

	for id, task := range r.tasks {
		if (task.State == TaskCompleted || task.State == TaskFailed || task.State == TaskCancelled) &&
			!task.CompletedAt.IsZero() && task.CompletedAt.Before(cutoff) {
			delete(r.tasks, id)
			count++
		}
	}
	return count
}

func (r *TaskRegistry) evictOldest() {
	var oldest *Task
	var oldestID string

	for id, task := range r.tasks {
		if oldest == nil || task.CreatedAt.Before(oldest.CreatedAt) {
			oldest = task
			oldestID = id
		}
	}

	if oldest != nil {
		delete(r.tasks, oldestID)
	}
}
