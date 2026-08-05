package swarm

import (
	"context"
	"sync"
	"time"
)

type HealthMonitor struct {
	registry         *BackendRegistry
	mu               sync.RWMutex
	checks           map[BackendID]*healthCheck
	interval         time.Duration
	timeout          time.Duration
	failureThreshold int
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	callbacks        []HealthCallback
}

type healthCheck struct {
	backendID           BackendID
	lastCheck           time.Time
	consecutiveFailures int
	status              BackendStatus
}

type HealthCallback func(backendID BackendID, oldStatus, newStatus BackendStatus)

func NewHealthMonitor(registry *BackendRegistry, interval, timeout time.Duration, failureThreshold int) *HealthMonitor {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if failureThreshold <= 0 {
		failureThreshold = 3
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &HealthMonitor{
		registry:         registry,
		checks:           make(map[BackendID]*healthCheck),
		interval:         interval,
		timeout:          timeout,
		failureThreshold: failureThreshold,
		ctx:              ctx,
		cancel:           cancel,
		callbacks:        make([]HealthCallback, 0),
	}
}

func (m *HealthMonitor) Start() {
	m.wg.Add(1)
	go m.monitorLoop()
}

func (m *HealthMonitor) Stop() {
	m.cancel()
	m.wg.Wait()
}

func (m *HealthMonitor) RegisterCallback(cb HealthCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, cb)
}

func (m *HealthMonitor) monitorLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAll()
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *HealthMonitor) checkAll() {
	backends := m.registry.GetAll()
	for _, b := range backends {
		m.checkBackend(b)
	}
}

func (m *HealthMonitor) checkBackend(b *Backend) {
	m.mu.Lock()
	check, ok := m.checks[b.ID]
	if !ok {
		check = &healthCheck{
			backendID: b.ID,
			status:    b.Status,
		}
		m.checks[b.ID] = check
	}
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(m.ctx, m.timeout)
	defer cancel()

	err := m.pingBackend(ctx, b)

	m.mu.Lock()
	defer m.mu.Unlock()

	check.lastCheck = time.Now()
	oldStatus := check.status

	if err != nil {
		check.consecutiveFailures++
		if check.consecutiveFailures >= m.failureThreshold && check.status != StatusUnhealthy {
			check.status = StatusUnhealthy
			b.Status = StatusUnhealthy
			m.notifyCallbacks(b.ID, oldStatus, StatusUnhealthy)
		}
	} else {
		check.consecutiveFailures = 0
		if check.status != StatusHealthy {
			check.status = StatusHealthy
			b.Status = StatusHealthy
			m.notifyCallbacks(b.ID, oldStatus, StatusHealthy)
		}
	}
}

func (m *HealthMonitor) pingBackend(ctx context.Context, _ *Backend) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}

func (m *HealthMonitor) notifyCallbacks(backendID BackendID, oldStatus, newStatus BackendStatus) {
	for _, cb := range m.callbacks {
		go cb(backendID, oldStatus, newStatus)
	}
}

func (m *HealthMonitor) GetHealth(backendID BackendID) (BackendStatus, time.Time, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	check, ok := m.checks[backendID]
	if !ok {
		return StatusUnknown, time.Time{}, 0
	}
	return check.status, check.lastCheck, check.consecutiveFailures
}

func (m *HealthMonitor) GetAllHealth() map[BackendID]BackendHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[BackendID]BackendHealth)
	for id, check := range m.checks {
		result[id] = BackendHealth{
			BackendID:           id,
			Status:              check.status,
			LastCheck:           check.lastCheck,
			ConsecutiveFailures: check.consecutiveFailures,
		}
	}
	return result
}

type BackendHealth struct {
	BackendID           BackendID
	Status              BackendStatus
	LastCheck           time.Time
	ConsecutiveFailures int
}

func (m *HealthMonitor) ForceCheck(backendID BackendID) error {
	backends := m.registry.GetAll()
	for _, b := range backends {
		if b.ID == backendID {
			m.checkBackend(b)
			return nil
		}
	}
	return ErrBackendNotFound
}
