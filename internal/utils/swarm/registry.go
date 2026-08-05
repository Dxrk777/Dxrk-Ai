package swarm

import (
	"context"
	"errors"
	"sync"
	"time"
)

type BackendRegistry struct {
	backends map[BackendID]*Backend
	mu       sync.RWMutex
	config   *SwarmConfig
	events   *EventBus
}

func NewBackendRegistry(config *SwarmConfig, events *EventBus) *BackendRegistry {
	if config == nil {
		config = DefaultSwarmConfig()
	}
	return &BackendRegistry{
		backends: make(map[BackendID]*Backend),
		config:   config,
		events:   events,
	}
}

func (r *BackendRegistry) Register(ctx context.Context, backend *Backend) error {
	if backend == nil {
		return errors.New("backend cannot be nil")
	}
	if backend.ID == "" {
		backend.ID = GenerateBackendID()
	}
	if backend.Name == "" {
		return errors.New("backend name is required")
	}
	if backend.Capacity <= 0 {
		backend.Capacity = 1
	}
	if backend.Capabilities == nil {
		backend.Capabilities = make(BackendCapabilities)
	}
	if backend.Metadata == nil {
		backend.Metadata = make(map[string]string)
	}

	backend.RegisteredAt = time.Now()
	backend.LastHeartbeat = time.Now()
	backend.Status = StatusStarting

	r.mu.Lock()
	if len(r.backends) >= r.config.MaxBackends {
		r.mu.Unlock()
		return errors.New("maximum backends reached")
	}
	r.backends[backend.ID] = backend
	r.mu.Unlock()

	backend.SetStatus(StatusHealthy)

	r.emit(EventBackendRegistered, backend.ID, map[string]interface{}{
		"name":         backend.Name,
		"address":      backend.Address,
		"capacity":     backend.Capacity,
		"capabilities": backend.Capabilities,
	})

	return nil
}

func (r *BackendRegistry) Unregister(ctx context.Context, backendID BackendID) error {
	r.mu.Lock()
	backend, exists := r.backends[backendID]
	if !exists {
		r.mu.Unlock()
		return ErrBackendNotFound
	}
	backend.SetStatus(StatusStopping)
	delete(r.backends, backendID)
	r.mu.Unlock()

	r.emit(EventBackendUnregistered, backendID, map[string]interface{}{
		"name": backend.Name,
	})

	return nil
}

func (r *BackendRegistry) Get(backendID BackendID) (*Backend, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backend, exists := r.backends[backendID]
	if !exists {
		return nil, ErrBackendNotFound
	}
	return backend, nil
}

func (r *BackendRegistry) GetAll() []*Backend {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backends := make([]*Backend, 0, len(r.backends))
	for _, b := range r.backends {
		backends = append(backends, b)
	}
	return backends
}

func (r *BackendRegistry) GetHealthy() []*Backend {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backends := make([]*Backend, 0)
	for _, b := range r.backends {
		if b.Status == StatusHealthy || b.Status == StatusDegraded {
			backends = append(backends, b)
		}
	}
	return backends
}

func (r *BackendRegistry) GetByCapability(capability string, minAmount int) []*Backend {
	r.mu.RLock()
	defer r.mu.RUnlock()
	backends := make([]*Backend, 0)
	for _, b := range r.backends {
		if b.Status != StatusHealthy && b.Status != StatusDegraded {
			continue
		}
		if amount, ok := b.Capabilities[capability]; ok && amount >= minAmount {
			backends = append(backends, b)
		}
	}
	return backends
}

func (r *BackendRegistry) UpdateHeartbeat(backendID BackendID) error {
	r.mu.RLock()
	backend, exists := r.backends[backendID]
	r.mu.RUnlock()
	if !exists {
		return ErrBackendNotFound
	}
	backend.UpdateHeartbeat()
	r.emit(EventBackendHeartbeat, backendID, nil)
	return nil
}

func (r *BackendRegistry) UpdateStatus(backendID BackendID, status BackendStatus) error {
	r.mu.RLock()
	backend, exists := r.backends[backendID]
	r.mu.RUnlock()
	if !exists {
		return ErrBackendNotFound
	}
	oldStatus := backend.Status
	backend.SetStatus(status)
	if oldStatus != status {
		r.emit(EventBackendStatusChanged, backendID, map[string]interface{}{
			"old_status": oldStatus.String(),
			"new_status": status.String(),
		})
	}
	return nil
}

func (r *BackendRegistry) UpdateCapacity(backendID BackendID, capacity int) error {
	if capacity <= 0 {
		return errors.New("capacity must be positive")
	}
	r.mu.RLock()
	backend, exists := r.backends[backendID]
	r.mu.RUnlock()
	if !exists {
		return ErrBackendNotFound
	}
	backend.mu.Lock()
	backend.Capacity = capacity
	backend.mu.Unlock()
	return nil
}

func (r *BackendRegistry) UpdateCapabilities(backendID BackendID, capabilities BackendCapabilities) error {
	r.mu.RLock()
	backend, exists := r.backends[backendID]
	r.mu.RUnlock()
	if !exists {
		return ErrBackendNotFound
	}
	backend.mu.Lock()
	backend.Capabilities = capabilities
	backend.mu.Unlock()
	return nil
}

func (r *BackendRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.backends)
}

func (r *BackendRegistry) HealthyCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, b := range r.backends {
		if b.Status == StatusHealthy || b.Status == StatusDegraded {
			count++
		}
	}
	return count
}

func (r *BackendRegistry) emit(eventType SwarmEventType, backendID BackendID, data map[string]interface{}) {
	if r.events != nil {
		r.events.Publish(SwarmEvent{
			Type:      eventType,
			Timestamp: time.Now(),
			BackendID: backendID,
			Data:      data,
		})
	}
}
