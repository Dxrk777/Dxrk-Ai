package tasktools

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TaskScheduler manages task execution with concurrency limits.
type TaskScheduler struct {
	queue         chan *Task
	tasks         *TaskManager
	maxConcurrent int
	running       sync.WaitGroup
	quit          chan struct{}
	depGraph      *DependencyGraph
}

// NewTaskScheduler creates a new scheduler with the given concurrency limit.
func NewTaskScheduler(maxConcurrent int) *TaskScheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}
	return &TaskScheduler{
		queue:         make(chan *Task, 256),
		tasks:         NewTaskManager(),
		maxConcurrent: maxConcurrent,
		quit:          make(chan struct{}),
		depGraph:      NewDependencyGraph(),
	}
}

// GetTaskManager returns the underlying task manager.
func (s *TaskScheduler) GetTaskManager() *TaskManager {
	return s.tasks
}

// GetDependencyGraph returns the dependency graph.
func (s *TaskScheduler) GetDependencyGraph() *DependencyGraph {
	return s.depGraph
}

// Submit sends a task to the execution queue immediately.
func (s *TaskScheduler) Submit(task *Task) error {
	s.tasks.mu.Lock()
	s.tasks.tasks[task.ID] = task
	s.tasks.mu.Unlock()

	status := StatusRunning
	task.Status = status
	task.UpdatedAt = time.Now()

	select {
	case s.queue <- task:
		s.running.Add(1)
		return nil
	default:
		return fmt.Errorf("task queue is full")
	}
}

// ScheduleOpts configures delayed or conditional scheduling.
type ScheduleOpts struct {
	Delay       time.Duration `json:"delay,omitempty"`
	Cron        string        `json:"cron,omitempty"`
	AfterTaskID string        `json:"after_task_id,omitempty"`
}

// Schedule queues a task for later execution.
func (s *TaskScheduler) Schedule(task *Task, opts ScheduleOpts) error {
	s.tasks.mu.Lock()
	s.tasks.tasks[task.ID] = task
	s.tasks.mu.Unlock()

	if opts.AfterTaskID != "" {
		if err := s.depGraph.AddDependency(task.ID, opts.AfterTaskID); err != nil {
			return fmt.Errorf("add dependency: %w", err)
		}
	}

	if opts.Delay > 0 {
		go func() {
			time.Sleep(opts.Delay)
			_ = s.Submit(task)
		}()
		return nil
	}

	return s.Submit(task)
}

// RetryOpts configures retry behavior.
type RetryOpts struct {
	MaxRetries  int           `json:"max_retries"`
	Backoff     time.Duration `json:"backoff"`
	Exponential bool          `json:"exponential"`
}

// Retry re-executes a failed task with backoff.
func (s *TaskScheduler) Retry(task *Task, opts RetryOpts) error {
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 3
	}
	if opts.Backoff <= 0 {
		opts.Backoff = time.Second
	}

	go func() {
		for attempt := 0; attempt < opts.MaxRetries; attempt++ {
			backoff := opts.Backoff
			if opts.Exponential {
				backoff = opts.Backoff * time.Duration(1<<uint(attempt))
			}
			time.Sleep(backoff)

			task.Status = StatusPending
			task.Error = ""
			task.UpdatedAt = time.Now()

			if err := s.Submit(task); err == nil {
				return
			}
		}
		task.Status = StatusFailed
		task.Error = fmt.Sprintf("exhausted %d retries", opts.MaxRetries)
		task.UpdatedAt = time.Now()
	}()

	return nil
}

// Timeout sets a context deadline for a task's execution.
func (s *TaskScheduler) Timeout(task *Task, timeout time.Duration) {
	go func() {
		time.Sleep(timeout)
		s.tasks.mu.Lock()
		if task.Status == StatusRunning {
			task.Status = StatusTimeout
			task.Error = fmt.Sprintf("exceeded timeout of %s", timeout)
			task.UpdatedAt = time.Now()
		}
		s.tasks.mu.Unlock()
	}()
}

// WithTimeout creates a context with the given timeout.
func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// CancelAll cancels all running and pending tasks.
func (s *TaskScheduler) CancelAll() error {
	s.tasks.mu.Lock()
	defer s.tasks.mu.Unlock()

	for _, task := range s.tasks.tasks {
		if task.Status == StatusRunning || task.Status == StatusPending {
			task.Status = StatusCancelled
			task.UpdatedAt = time.Now()
		}
	}
	return nil
}

// TaskStats holds aggregate task statistics.
type TaskStats struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Pending   int `json:"pending"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
}

// GetStats returns current queue statistics.
func (s *TaskScheduler) GetStats() TaskStats {
	s.tasks.mu.RLock()
	defer s.tasks.mu.RUnlock()

	stats := TaskStats{}
	for _, task := range s.tasks.tasks {
		stats.Total++
		switch task.Status {
		case StatusRunning:
			stats.Running++
		case StatusPending:
			stats.Pending++
		case StatusCompleted:
			stats.Completed++
		case StatusFailed:
			stats.Failed++
		case StatusCancelled:
			stats.Cancelled++
		}
	}
	return stats
}

// DependencyGraph tracks task dependencies and resolves execution order.
type DependencyGraph struct {
	mu    sync.RWMutex
	edges map[string][]string // taskID -> list of taskIDs it depends on
}

// NewDependencyGraph creates a new dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		edges: make(map[string][]string),
	}
}

// AddDependency declares that taskID cannot run until dependsOn completes.
func (g *DependencyGraph) AddDependency(taskID, dependsOn string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if taskID == dependsOn {
		return fmt.Errorf("task %q cannot depend on itself", taskID)
	}

	for _, dep := range g.edges[taskID] {
		if dep == dependsOn {
			return nil
		}
	}

	g.edges[taskID] = append(g.edges[taskID], dependsOn)
	return nil
}

// ResolveExecutionOrder returns tasks in topological order (dependency-first).
func (g *DependencyGraph) ResolveExecutionOrder() ([][]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	inDegree := make(map[string]int)
	reverse := make(map[string][]string)

	for task, deps := range g.edges {
		if _, ok := inDegree[task]; !ok {
			inDegree[task] = 0
		}
		for _, dep := range deps {
			inDegree[task]++
			reverse[dep] = append(reverse[dep], task)
			if _, ok := inDegree[dep]; !ok {
				inDegree[dep] = 0
			}
		}
	}

	var levels [][]string
	visited := 0

	for {
		var ready []string
		for task, deg := range inDegree {
			if deg == 0 {
				ready = append(ready, task)
			}
		}

		if len(ready) == 0 {
			break
		}

		levels = append(levels, ready)
		visited += len(ready)

		for _, task := range ready {
			inDegree[task] = -1
			for _, next := range reverse[task] {
				inDegree[next]--
			}
		}
	}

	if visited != len(inDegree) {
		return nil, fmt.Errorf("dependency cycle detected")
	}

	return levels, nil
}
