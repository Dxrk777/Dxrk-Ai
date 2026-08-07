package tasktools

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/google/uuid"
)

// TaskStatus represents the current state of a task.
type TaskStatus int

const (
	StatusPending TaskStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCancelled
	StatusTimeout
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
	case StatusTimeout:
		return strconst.StrTimeout
	default:
		return strconst.StrUnknown
	}
}

// ParseTaskStatus converts a string to TaskStatus.
func ParseTaskStatus(s string) (TaskStatus, error) {
	switch strings.ToLower(s) {
	case strconst.StrPending:
		return StatusPending, nil
	case strconst.StrRunning:
		return StatusRunning, nil
	case strconst.StrCompleted:
		return StatusCompleted, nil
	case strconst.StrFailed:
		return StatusFailed, nil
	case strconst.StrCancelled:
		return StatusCancelled, nil
	case strconst.StrTimeout:
		return StatusTimeout, nil
	default:
		return StatusPending, fmt.Errorf("unknown task status: %q", s)
	}
}

// TaskPriority represents task urgency.
type TaskPriority int

const (
	PriorityLow TaskPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityUrgent
)

func (p TaskPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return strconst.StrNormal
	case PriorityHigh:
		return "high"
	case PriorityUrgent:
		return strconst.StrUrgent
	default:
		return strconst.StrNormal
	}
}

// ParseTaskPriority converts a string to TaskPriority.
func ParseTaskPriority(s string) (TaskPriority, error) {
	switch strings.ToLower(s) {
	case "low":
		return PriorityLow, nil
	case strconst.StrNormal, "":
		return PriorityNormal, nil
	case "high":
		return PriorityHigh, nil
	case strconst.StrUrgent:
		return PriorityUrgent, nil
	default:
		return PriorityNormal, fmt.Errorf("unknown task priority: %q", s)
	}
}

// Task represents a unit of work tracked by the system.
type Task struct {
	ID          string         `json:"id"`
	ParentID    string         `json:"parent_id,omitempty"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Status      TaskStatus     `json:"status"`
	Priority    TaskPriority   `json:"priority"`
	AssignedTo  string         `json:"assigned_to,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Result      string         `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Progress    float64        `json:"progress"`
}

// CreateTaskOpts configures a new task.
type CreateTaskOpts struct {
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	ParentID    string         `json:"parent_id,omitempty"`
	Priority    string         `json:"priority,omitempty"`
	AssignedTo  string         `json:"assigned_to,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// TaskUpdate applies selective updates to a task.
type TaskUpdate struct {
	Status   *TaskStatus    `json:"status,omitempty"`
	Progress *float64       `json:"progress,omitempty"`
	Result   *string        `json:"result,omitempty"`
	Error    *string        `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ListTaskOpts configures task listing and filtering.
type ListTaskOpts struct {
	Status     string `json:"status,omitempty"`
	Priority   string `json:"priority,omitempty"`
	AssignedTo string `json:"assigned_to,omitempty"`
	ParentID   string `json:"parent_id,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	SortBy     string `json:"sort_by,omitempty"`
	SortDir    string `json:"sort_dir,omitempty"`
}

// TaskTree is a recursive task with children.
type TaskTree struct {
	Task     *Task       `json:"task"`
	Children []*TaskTree `json:"children,omitempty"`
}

// TaskManager provides thread-safe task CRUD and querying.
type TaskManager struct {
	tasks map[string]*Task
	mu    sync.RWMutex
}

// NewTaskManager creates a new TaskManager.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*Task),
	}
}

// CreateTask creates a new task with the given options.
func (m *TaskManager) CreateTask(opts CreateTaskOpts) *Task {
	priority := PriorityNormal
	if opts.Priority != "" {
		if p, err := ParseTaskPriority(opts.Priority); err == nil {
			priority = p
		}
	}

	now := time.Now()
	task := &Task{
		ID:          uuid.New().String()[:8],
		ParentID:    opts.ParentID,
		Title:       opts.Title,
		Description: opts.Description,
		Status:      StatusPending,
		Priority:    priority,
		AssignedTo:  opts.AssignedTo,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    opts.Metadata,
		Tags:        opts.Tags,
		Progress:    0,
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	return task
}

// GetTask retrieves a task by ID.
func (m *TaskManager) GetTask(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[id]
	return task, ok
}

// UpdateTask applies an update to a task.
func (m *TaskManager) UpdateTask(id string, update TaskUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}

	if update.Status != nil {
		task.Status = *update.Status
		if *update.Status == StatusCompleted {
			now := time.Now()
			task.CompletedAt = &now
			task.Progress = 1.0
		}
	}
	if update.Progress != nil {
		task.Progress = *update.Progress
	}
	if update.Result != nil {
		task.Result = *update.Result
	}
	if update.Error != nil {
		task.Error = *update.Error
	}
	if update.Metadata != nil {
		if task.Metadata == nil {
			task.Metadata = make(map[string]any)
		}
		for k, v := range update.Metadata {
			task.Metadata[k] = v
		}
	}

	task.UpdatedAt = time.Now()
	return nil
}

// CancelTask marks a task as cancelled.
func (m *TaskManager) CancelTask(id string) error {
	status := StatusCancelled
	return m.UpdateTask(id, TaskUpdate{Status: &status})
}

// DeleteTask removes a task.
func (m *TaskManager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[id]; !ok {
		return fmt.Errorf("task %q not found", id)
	}
	delete(m.tasks, id)
	return nil
}

// ListTasks returns tasks matching the given filters.
func (m *TaskManager) ListTasks(opts ListTaskOpts) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		if opts.Status != "" && task.Status.String() != opts.Status {
			continue
		}
		if opts.Priority != "" && task.Priority.String() != opts.Priority {
			continue
		}
		if opts.AssignedTo != "" && task.AssignedTo != opts.AssignedTo {
			continue
		}
		if opts.ParentID != "" && task.ParentID != opts.ParentID {
			continue
		}
		result = append(result, task)
	}

	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = strconst.StrCreatedAt
	}
	sortDir := opts.SortDir
	if sortDir == "" {
		sortDir = "desc"
	}

	sort.Slice(result, func(i, j int) bool {
		less := taskFieldLess(result[i], result[j], sortBy)
		if sortDir == "asc" {
			return less
		}
		return !less
	})

	if opts.Limit > 0 && len(result) > opts.Limit {
		offset := opts.Offset
		if offset >= len(result) {
			return []*Task{}
		}
		end := offset + opts.Limit
		if end > len(result) {
			end = len(result)
		}
		return result[offset:end]
	}

	return result
}

// GetSubtasks returns all direct children of a parent task.
func (m *TaskManager) GetSubtasks(parentID string) []*Task {
	return m.ListTasks(ListTaskOpts{ParentID: parentID})
}

// GetTaskTree returns a task and all its descendants as a tree.
func (m *TaskManager) GetTaskTree(id string) *TaskTree {
	task, ok := m.GetTask(id)
	if !ok {
		return nil
	}
	return m.buildTree(task)
}

func (m *TaskManager) buildTree(task *Task) *TaskTree {
	node := &TaskTree{Task: task}
	children := m.GetSubtasks(task.ID)
	for _, child := range children {
		node.Children = append(node.Children, m.buildTree(child))
	}
	return node
}

func taskFieldLess(a, b *Task, field string) bool {
	switch field {
	case strconst.StrTitle:
		return a.Title < b.Title
	case strconst.StrPriority:
		return int(a.Priority) > int(b.Priority)
	case strconst.StrStatus:
		return int(a.Status) < int(b.Status)
	case "updated_at":
		return a.UpdatedAt.Before(b.UpdatedAt)
	default:
		return a.CreatedAt.Before(b.CreatedAt)
	}
}
