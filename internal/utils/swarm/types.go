package swarm

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

const (
	DefaultHeartbeatInterval = 5 * time.Second
	DefaultTaskTimeout       = 30 * time.Second
	DefaultLeaseDuration     = 10 * time.Second
	DefaultMaxRetries        = 3
)

var (
	ErrBackendNotFound   = errors.New("backend not found")
	ErrBackendUnhealthy  = errors.New("backend unhealthy")
	ErrTaskNotFound      = errors.New("task not found")
	ErrNoCapacity        = errors.New("no capacity available")
	ErrNotLeader         = errors.New("not leader")
	ErrLeaseExpired      = errors.New("lease expired")
	ErrInvalidConfig     = errors.New("invalid configuration")
	ErrSwarmShuttingDown = errors.New("swarm shutting down")
)

type BackendID string
type TaskID string

type BackendStatus int

const (
	StatusUnknown BackendStatus = iota
	StatusStarting
	StatusHealthy
	StatusDegraded
	StatusUnhealthy
	StatusStopping
	StatusStopped
)

func (s BackendStatus) String() string {
	switch s {
	case StatusUnknown:
		return strconst.StrUnknown
	case StatusStarting:
		return "starting"
	case StatusHealthy:
		return "healthy"
	case StatusDegraded:
		return "degraded"
	case StatusUnhealthy:
		return "unhealthy"
	case StatusStopping:
		return "stopping"
	case StatusStopped:
		return "stopped"
	default:
		return strconst.StrUnknown
	}
}

type BackendCapabilities map[string]int

type Backend struct {
	ID            BackendID
	Name          string
	Address       string
	Capabilities  BackendCapabilities
	Capacity      int
	Load          int
	Status        BackendStatus
	Metadata      map[string]string
	LastHeartbeat time.Time
	RegisteredAt  time.Time
	LeaseID       string
	mu            sync.RWMutex
}

func (b *Backend) AvailableCapacity() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Capacity - b.Load
}

func (b *Backend) CanHandle(task *Task) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.Status != StatusHealthy && b.Status != StatusDegraded {
		return false
	}
	if b.Load >= b.Capacity {
		return false
	}
	for reqCap, reqAmount := range task.RequiredCapabilities {
		if avail, ok := b.Capabilities[reqCap]; !ok || avail < reqAmount {
			return false
		}
	}
	return true
}

func (b *Backend) IncrementLoad() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Load >= b.Capacity {
		return false
	}
	b.Load++
	return true
}

func (b *Backend) DecrementLoad() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Load > 0 {
		b.Load--
	}
}

func (b *Backend) UpdateHeartbeat() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.LastHeartbeat = time.Now()
}

func (b *Backend) SetStatus(status BackendStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Status = status
}

func (b *Backend) MarshalJSON() ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	type Alias Backend
	return json.Marshal(&struct {
		*Alias
		Status string `json:"status"`
	}{
		Alias:  (*Alias)(b),
		Status: b.Status.String(),
	})
}

type TaskPriority int

const (
	PriorityLow TaskPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

type Task struct {
	ID                   TaskID
	Type                 string
	Payload              json.RawMessage
	RequiredCapabilities BackendCapabilities
	Priority             TaskPriority
	Timeout              time.Duration
	Retries              int
	MaxRetries           int
	CreatedAt            time.Time
	StartedAt            time.Time
	CompletedAt          time.Time
	AssignedBackend      BackendID
	Result               *TaskResult
	Error                string
	mu                   sync.RWMutex
}

func (t *Task) IsCompleted() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return !t.CompletedAt.IsZero()
}

func (t *Task) IsAssigned() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.AssignedBackend != ""
}

func (t *Task) Assign(backendID BackendID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.AssignedBackend = backendID
	t.StartedAt = time.Now()
}

func (t *Task) Complete(result *TaskResult, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.CompletedAt = time.Now()
	t.Result = result
	if err != nil {
		t.Error = err.Error()
	}
}

func (t *Task) CanRetry() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Retries < t.MaxRetries
}

func (t *Task) IncrementRetry() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Retries++
	t.AssignedBackend = ""
	t.StartedAt = time.Time{}
	t.Error = ""
}

type TaskResult struct {
	TaskID    TaskID
	BackendID BackendID
	Output    json.RawMessage
	Metrics   map[string]float64
	Duration  time.Duration
	Timestamp time.Time
}

type SwarmConfig struct {
	HeartbeatInterval     time.Duration
	TaskTimeout           time.Duration
	LeaseDuration         time.Duration
	MaxRetries            int
	EnableWorkStealing    bool
	EnableLoadBalancing   bool
	MinBackends           int
	MaxBackends           int
	LeaderElectionEnabled bool
}

func DefaultSwarmConfig() *SwarmConfig {
	return &SwarmConfig{
		HeartbeatInterval:     DefaultHeartbeatInterval,
		TaskTimeout:           DefaultTaskTimeout,
		LeaseDuration:         DefaultLeaseDuration,
		MaxRetries:            DefaultMaxRetries,
		EnableWorkStealing:    true,
		EnableLoadBalancing:   true,
		MinBackends:           1,
		MaxBackends:           100,
		LeaderElectionEnabled: true,
	}
}

func (c *SwarmConfig) Validate() error {
	if c.HeartbeatInterval <= 0 {
		return ErrInvalidConfig
	}
	if c.TaskTimeout <= 0 {
		return ErrInvalidConfig
	}
	if c.LeaseDuration <= 0 {
		return ErrInvalidConfig
	}
	if c.MaxRetries < 0 {
		return ErrInvalidConfig
	}
	if c.MinBackends < 0 || c.MaxBackends < c.MinBackends {
		return ErrInvalidConfig
	}
	return nil
}

type SwarmEventType int

const (
	EventBackendRegistered SwarmEventType = iota
	EventBackendUnregistered
	EventBackendStatusChanged
	EventBackendHeartbeat
	EventTaskSubmitted
	EventTaskAssigned
	EventTaskStarted
	EventTaskCompleted
	EventTaskFailed
	EventTaskRetry
	EventLeaderElected
	EventLeaderLost
	EventWorkStolen
	EventLoadBalanced
)

func (e SwarmEventType) String() string {
	switch e {
	case EventBackendRegistered:
		return "backend_registered"
	case EventBackendUnregistered:
		return "backend_unregistered"
	case EventBackendStatusChanged:
		return "backend_status_changed"
	case EventBackendHeartbeat:
		return "backend_heartbeat"
	case EventTaskSubmitted:
		return "task_submitted"
	case EventTaskAssigned:
		return "task_assigned"
	case EventTaskStarted:
		return "task_started"
	case EventTaskCompleted:
		return "task_completed"
	case EventTaskFailed:
		return "task_failed"
	case EventTaskRetry:
		return "task_retry"
	case EventLeaderElected:
		return "leader_elected"
	case EventLeaderLost:
		return "leader_lost"
	case EventWorkStolen:
		return "work_stolen"
	case EventLoadBalanced:
		return "load_balanced"
	default:
		return strconst.StrUnknown
	}
}

type SwarmEvent struct {
	Type      SwarmEventType
	Timestamp time.Time
	BackendID BackendID
	TaskID    TaskID
	Data      map[string]interface{}
}

type EventHandler func(event SwarmEvent)

func GenerateTaskID() TaskID {
	return TaskID("task-" + randString(8))
}

func GenerateBackendID() BackendID {
	return BackendID("backend-" + randString(8))
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}
