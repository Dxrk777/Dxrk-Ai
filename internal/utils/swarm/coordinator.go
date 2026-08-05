package swarm

import (
	"context"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

type SwarmCoordinator struct {
	registry     *BackendRegistry
	scheduler    *TaskScheduler
	health       *HealthMonitor
	election     *LeaderElection
	eventBus     *EventBus
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	config       CoordinatorConfig
	isLeader     bool
	leaderID     BackendID
	unsubscribes []func()
}

type CoordinatorConfig struct {
	ElectionTimeout    time.Duration
	HeartbeatInterval  time.Duration
	TaskTimeout        time.Duration
	MaxRetries         int
	EnableWorkStealing bool
}

func NewSwarmCoordinator(registry *BackendRegistry, config CoordinatorConfig) *SwarmCoordinator {
	if config.ElectionTimeout <= 0 {
		config.ElectionTimeout = 10 * time.Second
	}
	if config.HeartbeatInterval <= 0 {
		config.HeartbeatInterval = 5 * time.Second
	}
	if config.TaskTimeout <= 0 {
		config.TaskTimeout = 5 * time.Minute
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}

	ctx, cancel := context.WithCancel(context.Background())

	swarmConfig := &SwarmConfig{
		HeartbeatInterval:     config.HeartbeatInterval,
		TaskTimeout:           config.TaskTimeout,
		LeaseDuration:         config.ElectionTimeout,
		MaxRetries:            config.MaxRetries,
		EnableWorkStealing:    config.EnableWorkStealing,
		EnableLoadBalancing:   true,
		MinBackends:           1,
		MaxBackends:           100,
		LeaderElectionEnabled: true,
	}

	schedulerConfig := SchedulerConfig{
		MaxConcurrentTasks: 10,
		QueueSize:          100,
		WorkStealing:       config.EnableWorkStealing,
		TaskTimeout:        config.TaskTimeout,
		RetryAttempts:      config.MaxRetries,
		RetryDelay:         time.Second,
	}

	health := NewHealthMonitor(registry, config.HeartbeatInterval, 5*time.Second, 3)
	scheduler := NewTaskScheduler(registry, schedulerConfig)
	eventBus := NewEventBus(ctx)
	election := NewLeaderElection(swarmConfig, registry, eventBus)

	c := &SwarmCoordinator{
		registry:     registry,
		scheduler:    scheduler,
		health:       health,
		election:     election,
		eventBus:     eventBus,
		ctx:          ctx,
		cancel:       cancel,
		config:       config,
		unsubscribes: make([]func(), 0),
	}

	health.RegisterCallback(c.onHealthChange)

	return c
}

func (c *SwarmCoordinator) Start() {
	c.health.Start()
	c.scheduler.Start()
	c.election.Start()
	c.eventBus.Start()

	c.wg.Add(1)
	go c.coordinatorLoop()
}

func (c *SwarmCoordinator) Stop() {
	c.cancel()
	c.wg.Wait()
	c.health.Stop()
	c.scheduler.Stop()
	c.election.Stop()
	c.eventBus.Stop()

	for _, unsub := range c.unsubscribes {
		unsub()
	}
}

func (c *SwarmCoordinator) coordinatorLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.heartbeat()
		case result := <-c.scheduler.Results():
			c.handleTaskResult(result)
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *SwarmCoordinator) heartbeat() {
	if c.isLeader {
		c.eventBus.Publish(SwarmEvent{
			Type:      EventBackendHeartbeat,
			BackendID: c.leaderID,
			Timestamp: time.Now(),
		})
	}
}

func (c *SwarmCoordinator) handleTaskResult(result *TaskResult) {
	c.eventBus.Publish(SwarmEvent{
		Type:      EventTaskCompleted,
		BackendID: result.BackendID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			strconst.StrTaskId: result.TaskID,
			"duration":         result.Duration,
			"timestamp":        result.Timestamp,
		},
	})
}

func (c *SwarmCoordinator) onHealthChange(backendID BackendID, oldStatus, newStatus BackendStatus) {
	c.eventBus.Publish(SwarmEvent{
		Type:      EventBackendStatusChanged,
		BackendID: backendID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"backend_id": string(backendID),
			"old_status": oldStatus.String(),
			"new_status": newStatus.String(),
		},
	})

	if newStatus == StatusUnhealthy {
		c.rescheduleTasks(backendID)
	}
}

func (c *SwarmCoordinator) rescheduleTasks(backendID BackendID) {
}

func (c *SwarmCoordinator) SubmitTask(task *Task) error {
	return c.scheduler.Submit(task)
}

func (c *SwarmCoordinator) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {
	return nil, nil
}

func (c *SwarmCoordinator) RegisterBackend(ctx context.Context, b *Backend) error {
	return c.registry.Register(ctx, b)
}

func (c *SwarmCoordinator) UnregisterBackend(ctx context.Context, backendID BackendID) error {
	return c.registry.Unregister(ctx, backendID)
}

func (c *SwarmCoordinator) GetBackends() []*Backend {
	return c.registry.GetAll()
}

func (c *SwarmCoordinator) GetHealthyBackends() []*Backend {
	return c.registry.GetHealthy()
}

func (c *SwarmCoordinator) Subscribe(eventType SwarmEventType, handler EventHandler) {
	unsub := c.eventBus.Subscribe(eventType, handler)
	c.mu.Lock()
	c.unsubscribes = append(c.unsubscribes, unsub)
	c.mu.Unlock()
}

func (c *SwarmCoordinator) Unsubscribe(eventType SwarmEventType, handler EventHandler) {
}

func (c *SwarmCoordinator) IsLeader() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isLeader
}

func (c *SwarmCoordinator) LeaderID() BackendID {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.leaderID
}

func (c *SwarmCoordinator) Stats() CoordinatorStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	schedulerStats := c.scheduler.Stats()
	healthStats := c.health.GetAllHealth()

	return CoordinatorStats{
		IsLeader:        c.isLeader,
		LeaderID:        c.leaderID,
		BackendCount:    len(c.registry.GetAll()),
		HealthyBackends: len(c.health.GetAllHealth()),
		SchedulerStats:  schedulerStats,
		BackendHealth:   healthStats,
	}
}

type CoordinatorStats struct {
	IsLeader        bool
	LeaderID        BackendID
	BackendCount    int
	HealthyBackends int
	SchedulerStats  SchedulerStats
	BackendHealth   map[BackendID]BackendHealth
}
