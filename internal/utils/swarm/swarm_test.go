package swarm

import (
	"context"
	"testing"
	"time"
)

func TestBackendRegistry_Register(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	backend := &Backend{
		Name:         "test-backend",
		Address:      "localhost:8080",
		Capacity:     5,
		Capabilities: BackendCapabilities{"cpu": 4},
	}

	err := reg.Register(ctx, backend)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if backend.ID == "" {
		t.Error("Backend ID should be generated")
	}
	if backend.Status != StatusHealthy {
		t.Errorf("Expected StatusHealthy, got %v", backend.Status)
	}
}

func TestBackendRegistry_RegisterValidation(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		backend *Backend
		wantErr bool
	}{
		{"nil backend", nil, true},
		{"empty name", &Backend{Capacity: 1}, true},
		{"valid", &Backend{Name: "test", Capacity: 1}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := reg.Register(ctx, tc.backend)
			if (err != nil) != tc.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestBackendRegistry_Unregister(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	backend := &Backend{Name: "test", Capacity: 1}
	reg.Register(ctx, backend)

	err := reg.Unregister(ctx, backend.ID)
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	_, err = reg.Get(backend.ID)
	if err == nil {
		t.Error("Expected error for unregistered backend")
	}
}

func TestBackendRegistry_Get(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	backend := &Backend{Name: "test", Capacity: 1}
	reg.Register(ctx, backend)

	got, err := reg.Get(backend.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != backend.ID {
		t.Errorf("ID mismatch: %s vs %s", got.ID, backend.ID)
	}
}

func TestBackendRegistry_GetAll(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	reg.Register(ctx, &Backend{Name: "b1", Capacity: 1})
	reg.Register(ctx, &Backend{Name: "b2", Capacity: 1})

	all := reg.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(all))
	}
}

func TestBackendRegistry_GetHealthy(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	reg.Register(ctx, &Backend{Name: "healthy", Capacity: 1})
	b2 := &Backend{Name: "unhealthy", Capacity: 1}
	reg.Register(ctx, b2)
	b2.SetStatus(StatusUnhealthy)

	healthy := reg.GetHealthy()
	if len(healthy) != 1 {
		t.Errorf("Expected 1 healthy backend, got %d", len(healthy))
	}
	if healthy[0].Name != "healthy" {
		t.Errorf("Expected 'healthy', got %s", healthy[0].Name)
	}
}

func TestBackendRegistry_UpdateStatus(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	backend := &Backend{Name: "test", Capacity: 1}
	reg.Register(ctx, backend)

	err := reg.UpdateStatus(backend.ID, StatusDegraded)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, _ := reg.Get(backend.ID)
	if got.Status != StatusDegraded {
		t.Errorf("Expected StatusDegraded, got %v", got.Status)
	}
}

func TestBackendRegistry_Count(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	if reg.Count() != 0 {
		t.Errorf("Expected 0, got %d", reg.Count())
	}

	reg.Register(ctx, &Backend{Name: "b1", Capacity: 1})
	if reg.Count() != 1 {
		t.Errorf("Expected 1, got %d", reg.Count())
	}
}

func TestBackendRegistry_MaxBackends(t *testing.T) {
	config := DefaultSwarmConfig()
	config.MaxBackends = 1
	reg := NewBackendRegistry(config, nil)
	ctx := context.Background()

	reg.Register(ctx, &Backend{Name: "b1", Capacity: 1})
	err := reg.Register(ctx, &Backend{Name: "b2", Capacity: 1})
	if err == nil {
		t.Error("Expected error when max backends reached")
	}
}

func TestBackend_CanHandle(t *testing.T) {
	backend := &Backend{
		ID:           "test",
		Capacity:     2,
		Load:         0,
		Status:       StatusHealthy,
		Capabilities: BackendCapabilities{"cpu": 4, "memory": 8},
	}

	task := &Task{
		RequiredCapabilities: BackendCapabilities{"cpu": 2},
	}
	if !backend.CanHandle(task) {
		t.Error("Should handle task with sufficient capabilities")
	}

	task.RequiredCapabilities = BackendCapabilities{"cpu": 8}
	if backend.CanHandle(task) {
		t.Error("Should not handle task with insufficient capabilities")
	}

	backend.SetStatus(StatusUnhealthy)
	task.RequiredCapabilities = BackendCapabilities{"cpu": 2}
	if backend.CanHandle(task) {
		t.Error("Should not handle when unhealthy")
	}
}

func TestBackend_AvailableCapacity(t *testing.T) {
	backend := &Backend{Capacity: 10, Load: 3}
	if backend.AvailableCapacity() != 7 {
		t.Errorf("Expected 7, got %d", backend.AvailableCapacity())
	}
}

func TestBackend_IncrementDecrementLoad(t *testing.T) {
	backend := &Backend{Capacity: 2, Load: 0}

	ok := backend.IncrementLoad()
	if !ok || backend.Load != 1 {
		t.Errorf("IncrementLoad failed: ok=%v, load=%d", ok, backend.Load)
	}

	ok = backend.IncrementLoad()
	if !ok || backend.Load != 2 {
		t.Errorf("Second IncrementLoad failed: ok=%v, load=%d", ok, backend.Load)
	}

	ok = backend.IncrementLoad()
	if ok {
		t.Error("Should fail when at capacity")
	}

	backend.DecrementLoad()
	if backend.Load != 1 {
		t.Errorf("Expected load 1 after decrement, got %d", backend.Load)
	}
}

func TestTask_Assign(t *testing.T) {
	task := &Task{ID: "task-1"}
	backendID := BackendID("backend-1")

	task.Assign(backendID)
	if task.AssignedBackend != backendID {
		t.Errorf("AssignedBackend = %s, want %s", task.AssignedBackend, backendID)
	}
	if task.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
}

func TestTask_CanRetry(t *testing.T) {
	task := &Task{MaxRetries: 3, Retries: 0}
	if !task.CanRetry() {
		t.Error("Should be able to retry")
	}

	task.Retries = 3
	if task.CanRetry() {
		t.Error("Should not be able to retry after max retries")
	}
}

func TestTask_IncrementRetry(t *testing.T) {
	task := &Task{AssignedBackend: "b1", Retries: 0}
	task.IncrementRetry()

	if task.Retries != 1 {
		t.Errorf("Retries = %d, want 1", task.Retries)
	}
	if task.AssignedBackend != "" {
		t.Error("AssignedBackend should be cleared")
	}
	if !task.StartedAt.IsZero() {
		t.Error("StartedAt should be cleared")
	}
}

func TestBackendStatus_String(t *testing.T) {
	tests := []struct {
		status BackendStatus
		expect string
	}{
		{StatusUnknown, "unknown"},
		{StatusStarting, "starting"},
		{StatusHealthy, "healthy"},
		{StatusDegraded, "degraded"},
		{StatusUnhealthy, "unhealthy"},
		{StatusStopping, "stopping"},
		{StatusStopped, "stopped"},
	}

	for _, tc := range tests {
		t.Run(tc.expect, func(t *testing.T) {
			if tc.status.String() != tc.expect {
				t.Errorf("String() = %s, want %s", tc.status.String(), tc.expect)
			}
		})
	}
}

func TestSwarmEventType_String(t *testing.T) {
	if EventBackendRegistered.String() != "backend_registered" {
		t.Error("EventBackendRegistered string mismatch")
	}
	if EventLeaderElected.String() != "leader_elected" {
		t.Error("EventLeaderElected string mismatch")
	}
}

func TestDefaultSwarmConfig(t *testing.T) {
	config := DefaultSwarmConfig()
	if config.HeartbeatInterval <= 0 {
		t.Error("HeartbeatInterval should be positive")
	}
	if config.TaskTimeout <= 0 {
		t.Error("TaskTimeout should be positive")
	}
	if config.MaxRetries < 0 {
		t.Error("MaxRetries should be non-negative")
	}
}

func TestSwarmConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *SwarmConfig
		wantErr bool
	}{
		{"valid", DefaultSwarmConfig(), false},
		{"zero heartbeat", &SwarmConfig{HeartbeatInterval: 0}, true},
		{"negative timeout", &SwarmConfig{TaskTimeout: -1}, true},
		{"invalid backends", &SwarmConfig{MinBackends: 5, MaxBackends: 1}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestTaskScheduler(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	reg.Register(ctx, &Backend{Name: "worker1", Capacity: 5})
	reg.Register(ctx, &Backend{Name: "worker2", Capacity: 5})

	scheduler := NewTaskScheduler(reg, SchedulerConfig{
		MaxConcurrentTasks: 2,
		QueueSize:          10,
		TaskTimeout:        time.Second,
	})
	scheduler.Start()
	defer scheduler.Stop()

	task := &Task{
		Type:                 "test",
		RequiredCapabilities: BackendCapabilities{"cpu": 1},
	}

	err := scheduler.Submit(task)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	select {
	case result := <-scheduler.Results():
		if result.TaskID != task.ID {
			t.Errorf("Result TaskID mismatch: %s vs %s", result.TaskID, task.ID)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for result")
	}
}

func TestLeaderElection(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	reg.Register(ctx, &Backend{Name: "leader1", Capacity: 5})
	reg.Register(ctx, &Backend{Name: "leader2", Capacity: 5})

	events := NewEventBus(ctx)
	events.Start()
	defer events.Stop()

	config := DefaultSwarmConfig()
	config.LeaderElectionEnabled = true
	config.LeaseDuration = 100 * time.Millisecond

	election := NewLeaderElection(config, reg, events)
	election.Start()
	defer election.Stop()

	time.Sleep(200 * time.Millisecond)

	leader, err := election.GetLeader()
	if err != nil {
		t.Fatalf("GetLeader failed: %v", err)
	}
	if leader == nil {
		t.Error("Leader should be elected")
	}
}

func TestEventBus(t *testing.T) {
	ctx := context.Background()
	bus := NewEventBus(ctx)
	bus.Start()
	defer bus.Stop()

	received := make(chan SwarmEvent, 10)
	unsub := bus.Subscribe(EventBackendRegistered, func(e SwarmEvent) {
		received <- e
	})
	defer unsub()

	bus.Publish(SwarmEvent{
		Type:      EventBackendRegistered,
		BackendID: "test-backend",
		Timestamp: time.Now(),
	})

	select {
	case e := <-received:
		if e.BackendID != "test-backend" {
			t.Errorf("BackendID = %s, want test-backend", e.BackendID)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for event")
	}
}

func TestEventBus_SubscribeAll(t *testing.T) {
	ctx := context.Background()
	bus := NewEventBus(ctx)
	bus.Start()
	defer bus.Stop()

	received := make(chan SwarmEvent, 10)
	bus.Subscribe(EventBackendRegistered, func(SwarmEvent) {})
	bus.Subscribe(EventTaskCompleted, func(SwarmEvent) {})
	unsub := bus.SubscribeAll(func(e SwarmEvent) {
		received <- e
	})
	defer unsub()

	bus.Publish(SwarmEvent{Type: EventBackendRegistered, BackendID: "b1"})
	bus.Publish(SwarmEvent{Type: EventTaskCompleted, BackendID: "b2"})

	seen := make(map[string]bool)
	timeout := time.After(time.Second)
	for len(seen) < 2 {
		select {
		case e := <-received:
			seen[string(e.BackendID)] = true
		case <-timeout:
			t.Fatalf("Timeout waiting for events, seen: %v", seen)
		}
	}
}

func TestHealthMonitor(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	backend := &Backend{Name: "test", Capacity: 1}
	reg.Register(ctx, backend)

	monitor := NewHealthMonitor(reg, 50*time.Millisecond, 10*time.Millisecond, 2)
	monitor.Start()
	defer monitor.Stop()

	time.Sleep(150 * time.Millisecond)

	status, _, failures := monitor.GetHealth(backend.ID)
	if status != StatusHealthy {
		t.Errorf("Expected healthy, got %v", status)
	}
	if failures != 0 {
		t.Errorf("Expected 0 failures, got %d", failures)
	}
}

func TestSwarmCoordinator(t *testing.T) {
	reg := NewBackendRegistry(DefaultSwarmConfig(), nil)
	ctx := context.Background()

	reg.Register(ctx, &Backend{Name: "worker1", Capacity: 5})

	coord := NewSwarmCoordinator(reg, CoordinatorConfig{
		ElectionTimeout:    100 * time.Millisecond,
		HeartbeatInterval:  50 * time.Millisecond,
		EnableWorkStealing: true,
	})
	coord.Start()
	defer coord.Stop()

	time.Sleep(200 * time.Millisecond)

	backends := coord.GetBackends()
	if len(backends) != 1 {
		t.Errorf("Expected 1 backend, got %d", len(backends))
	}

	stats := coord.Stats()
	if stats.BackendCount != 1 {
		t.Errorf("Stats.BackendCount = %d, want 1", stats.BackendCount)
	}
}

func TestGenerateIDs(t *testing.T) {
	taskID := GenerateTaskID()
	if len(string(taskID)) < 10 {
		t.Errorf("TaskID too short: %s", taskID)
	}

	backendID := GenerateBackendID()
	if len(string(backendID)) < 10 {
		t.Errorf("BackendID too short: %s", backendID)
	}
}

func TestBackend_MarshalJSON(t *testing.T) {
	backend := &Backend{
		ID:       "test",
		Name:     "test-backend",
		Status:   StatusHealthy,
		Capacity: 5,
	}

	data, err := backend.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("JSON should not be empty")
	}
}
