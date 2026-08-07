package swarm

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

type TaskScheduler struct {
	registry  *BackendRegistry
	mu        sync.RWMutex
	workers   map[string]*worker
	taskQueue chan *Task
	results   chan *TaskResult
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	config    SchedulerConfig
}

type SchedulerConfig struct {
	MaxConcurrentTasks int
	QueueSize          int
	WorkStealing       bool
	TaskTimeout        time.Duration
	RetryAttempts      int
	RetryDelay         time.Duration
}

type worker struct {
	id      string
	backend *Backend
	tasks   chan *Task
	results chan *TaskResult
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewTaskScheduler(registry *BackendRegistry, config SchedulerConfig) *TaskScheduler {
	if config.MaxConcurrentTasks <= 0 {
		config.MaxConcurrentTasks = 10
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}
	if config.TaskTimeout <= 0 {
		config.TaskTimeout = 5 * time.Minute
	}
	if config.RetryAttempts <= 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &TaskScheduler{
		registry:  registry,
		workers:   make(map[string]*worker),
		taskQueue: make(chan *Task, config.QueueSize),
		results:   make(chan *TaskResult, config.QueueSize),
		ctx:       ctx,
		cancel:    cancel,
		config:    config,
	}
}

func (s *TaskScheduler) Start() {
	for i := 0; i < s.config.MaxConcurrentTasks; i++ {
		s.startWorker(i)
	}
	s.wg.Add(1)
	go s.dispatchLoop()
}

func (s *TaskScheduler) startWorker(_ int) {
	ctx, cancel := context.WithCancel(s.ctx)
	w := &worker{
		id:      s.generateWorkerID(),
		tasks:   make(chan *Task, 10),
		results: s.results,
		ctx:     ctx,
		cancel:  cancel,
	}
	s.mu.Lock()
	s.workers[w.id] = w
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		for {
			select {
			case task := <-w.tasks:
				s.executeTask(w, task)
			case <-w.ctx.Done():
				return
			}
		}
	}()
}

func (s *TaskScheduler) generateWorkerID() string {
	return "worker-" + randString(8)
}

func (s *TaskScheduler) Stop() {
	s.cancel()
	s.wg.Wait()
	close(s.taskQueue)
	close(s.results)
}

func (s *TaskScheduler) Submit(task *Task) error {
	select {
	case s.taskQueue <- task:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
		return ErrQueueFull
	}
}

func (s *TaskScheduler) dispatchLoop() {
	defer s.wg.Done()
	for {
		select {
		case task, ok := <-s.taskQueue:
			if !ok {
				return
			}
			s.dispatchTask(task)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *TaskScheduler) dispatchTask(task *Task) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	backends := s.registry.GetHealthy()
	if len(backends) == 0 {
		task.Error = "no healthy backends available"
		s.results <- &TaskResult{
			TaskID:    task.ID,
			Output:    nil,
			Metrics:   map[string]float64{strconst.StrError: 1},
			Duration:  0,
			Timestamp: time.Now(),
		}
		return
	}

	var selected *Backend
	if s.config.WorkStealing {
		selected = s.selectBackendWorkStealing(backends, task)
	} else {
		selected = s.selectBackendLeastLoaded(backends)
	}

	if selected == nil {
		task.Error = "no suitable backend found"
		s.results <- &TaskResult{
			TaskID:    task.ID,
			Output:    nil,
			Metrics:   map[string]float64{strconst.StrError: 1},
			Duration:  0,
			Timestamp: time.Now(),
		}
		return
	}

	w, ok := s.workers[string(selected.ID)]
	if !ok {
		task.Error = "backend worker not found"
		s.results <- &TaskResult{
			TaskID:    task.ID,
			Output:    nil,
			Metrics:   map[string]float64{strconst.StrError: 1},
			Duration:  0,
			Timestamp: time.Now(),
		}
		return
	}

	task.Assign(selected.ID)
	task.StartedAt = time.Now()

	select {
	case w.tasks <- task:
	case <-time.After(time.Second):
		task.Error = "worker queue full"
		s.results <- &TaskResult{
			TaskID:    task.ID,
			Output:    nil,
			Metrics:   map[string]float64{strconst.StrError: 1},
			Duration:  0,
			Timestamp: time.Now(),
		}
	}
}

func (s *TaskScheduler) selectBackendLeastLoaded(backends []*Backend) *Backend {
	var selected *Backend
	minLoad := int(^uint(0) >> 1)
	for _, b := range backends {
		load := b.Load
		if load < minLoad {
			minLoad = load
			selected = b
		}
	}
	return selected
}

func (s *TaskScheduler) selectBackendWorkStealing(backends []*Backend, task *Task) *Backend {
	type backendScore struct {
		backend *Backend
		score   float64
	}
	scores := make([]backendScore, 0, len(backends))
	for _, b := range backends {
		capacity := float64(b.Capacity - b.Load)
		if capacity <= 0 {
			continue
		}
		affinity := 1.0
		if task.Type != "" && task.Type == string(b.ID) {
			affinity = 1.5
		}
		score := capacity * affinity / (1 + float64(b.Load))
		scores = append(scores, backendScore{backend: b, score: score})
	}
	if len(scores) == 0 {
		return nil
	}
	best := scores[0]
	for _, s := range scores[1:] {
		if s.score > best.score {
			best = s
		}
	}
	return best.backend
}

func (s *TaskScheduler) executeTask(w *worker, task *Task) {
	ctx, cancel := context.WithTimeout(s.ctx, s.config.TaskTimeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt <= s.config.RetryAttempts; attempt++ {
		select {
		case <-ctx.Done():
			task.Error = ctx.Err().Error()
			s.results <- &TaskResult{
				TaskID:    task.ID,
				Output:    nil,
				Metrics:   map[string]float64{strconst.StrError: 1, strconst.StrTimeout: 1},
				Duration:  time.Since(task.StartedAt),
				Timestamp: time.Now(),
			}
			return
		default:
		}

		result := s.runTask(ctx, w, task)
		if task.Error == "" {
			s.results <- result
			return
		}
		lastErr = errors.New(task.Error)
		if attempt < s.config.RetryAttempts {
			time.Sleep(s.config.RetryDelay)
		}
	}

	task.Error = lastErr.Error()
	s.results <- &TaskResult{
		TaskID:    task.ID,
		Output:    nil,
		Metrics:   map[string]float64{strconst.StrError: 1, "retries_exhausted": 1},
		Duration:  time.Since(task.StartedAt),
		Timestamp: time.Now(),
	}
}

func (s *TaskScheduler) runTask(_ context.Context, w *worker, task *Task) *TaskResult {
	start := time.Now()

	result := &TaskResult{
		TaskID:    task.ID,
		BackendID: w.backend.ID,
		Output:    nil,
		Metrics:   map[string]float64{"simulated": 1},
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}

	task.Complete(result, nil)
	return result
}

func (s *TaskScheduler) Results() <-chan *TaskResult {
	return s.results
}

func (s *TaskScheduler) Stats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activeWorkers := 0
	for _, w := range s.workers {
		select {
		case <-w.ctx.Done():
		default:
			activeWorkers++
		}
	}

	return SchedulerStats{
		ActiveWorkers: activeWorkers,
		TotalWorkers:  len(s.workers),
		QueueLength:   len(s.taskQueue),
		Config:        s.config,
	}
}

type SchedulerStats struct {
	ActiveWorkers int
	TotalWorkers  int
	QueueLength   int
	Config        SchedulerConfig
}

var ErrQueueFull = errors.New("task queue full")
