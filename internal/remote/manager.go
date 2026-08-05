// SPDX-License-Identifier: MIT
package remote

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ManagerOption is a functional option for RemoteManager.
type ManagerOption func(*RemoteManager)

// WithManagerMaxConnections sets the maximum number of connections.
func WithManagerMaxConnections(n int) ManagerOption {
	return func(m *RemoteManager) { m.maxConnections = n }
}

// WithManagerHealthInterval sets the health check interval in seconds.
func WithManagerHealthInterval(seconds int) ManagerOption {
	return func(m *RemoteManager) { m.healthInterval = seconds }
}

// RemoteManager manages multiple concurrent remote sessions.
type RemoteManager struct {
	mu              sync.RWMutex
	sessions        map[string]*RemoteSession
	controls        map[string]*RemoteControl
	maxConnections  int
	healthInterval  int
	ctx             context.Context
	cancel          context.CancelFunc
	onSessionAdd    func(id string, session *RemoteSession)
	onSessionRemove func(id string)
}

// NewRemoteManager creates a new manager with functional options.
func NewRemoteManager(opts ...ManagerOption) *RemoteManager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &RemoteManager{
		sessions:       make(map[string]*RemoteSession),
		controls:       make(map[string]*RemoteControl),
		maxConnections: 50,
		healthInterval: 30,
		ctx:            ctx,
		cancel:         cancel,
	}
	for _, opt := range opts {
		opt(m)
	}
	go m.healthCheckLoop()
	return m
}

// Add creates a new session with the given config and options.
func (m *RemoteManager) Add(cfg *RemoteConfig, opts ...SessionOption) (*RemoteSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.maxConnections {
		return nil, fmt.Errorf("maximum connections (%d) reached", m.maxConnections)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	session := NewRemoteSession(cfg, opts...)
	m.sessions[session.ID()] = session
	m.controls[session.ID()] = NewRemoteControl(session)

	if m.onSessionAdd != nil {
		m.onSessionAdd(session.ID(), session)
	}

	return session, nil
}

// Connect adds a session and immediately connects it.
func (m *RemoteManager) Connect(ctx context.Context, cfg *RemoteConfig, opts ...SessionOption) (*RemoteSession, error) {
	session, err := m.Add(cfg, opts...)
	if err != nil {
		return nil, err
	}

	if err := session.Connect(ctx); err != nil {
		_ = m.Remove(session.ID())
		return nil, fmt.Errorf("connect: %w", err)
	}

	return session, nil
}

// Remove removes a session by ID and disconnects it.
func (m *RemoteManager) Remove(id string) error {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session %s not found", id)
	}
	delete(m.sessions, id)
	delete(m.controls, id)
	m.mu.Unlock()

	_ = session.Close()

	if m.onSessionRemove != nil {
		m.onSessionRemove(id)
	}
	return nil
}

// Get returns a session by ID.
func (m *RemoteManager) Get(id string) (*RemoteSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return session, nil
}

// Control returns a controller for the session with the given ID.
func (m *RemoteManager) Control(id string) (*RemoteControl, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ctrl, ok := m.controls[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return ctrl, nil
}

// List returns all session IDs and their states.
func (m *RemoteManager) List() map[string]SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]SessionState, len(m.sessions))
	for id, session := range m.sessions {
		out[id] = session.State()
	}
	return out
}

// ConnectAll connects all disconnected sessions.
func (m *RemoteManager) ConnectAll(ctx context.Context) map[string]error {
	m.mu.RLock()
	sessions := make(map[string]*RemoteSession, len(m.sessions))
	for id, s := range m.sessions {
		sessions[id] = s
	}
	m.mu.RUnlock()

	errors := make(map[string]error)
	for id, s := range sessions {
		if s.State() == StateDisconnected || s.State() == StateFailed {
			if err := s.Connect(ctx); err != nil {
				errors[id] = err
			}
		}
	}
	return errors
}

// DisconnectAll disconnects all connected sessions.
func (m *RemoteManager) DisconnectAll() map[string]error {
	m.mu.RLock()
	sessions := make(map[string]*RemoteSession, len(m.sessions))
	for id, s := range m.sessions {
		sessions[id] = s
	}
	m.mu.RUnlock()

	errors := make(map[string]error)
	for id, s := range sessions {
		if err := s.Disconnect(); err != nil {
			errors[id] = err
		}
	}
	return errors
}

// ReconnectAll reconnects all sessions.
func (m *RemoteManager) ReconnectAll(ctx context.Context) map[string]error {
	m.mu.RLock()
	sessions := make(map[string]*RemoteSession, len(m.sessions))
	for id, s := range m.sessions {
		sessions[id] = s
	}
	m.mu.RUnlock()

	errors := make(map[string]error)
	for id, s := range sessions {
		if err := s.Reconnect(ctx); err != nil {
			errors[id] = err
		}
	}
	return errors
}

// OnSessionAdd registers a callback for when a session is added.
func (m *RemoteManager) OnSessionAdd(fn func(id string, session *RemoteSession)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onSessionAdd = fn
}

// OnSessionRemove registers a callback for when a session is removed.
func (m *RemoteManager) OnSessionRemove(fn func(id string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onSessionRemove = fn
}

// ConnectedCount returns the number of currently connected sessions.
func (m *RemoteManager) ConnectedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, s := range m.sessions {
		if s.State() == StateConnected {
			count++
		}
	}
	return count
}

// TotalCount returns the total number of managed sessions.
func (m *RemoteManager) TotalCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// healthCheckLoop periodically checks session health.
func (m *RemoteManager) healthCheckLoop() {
	interval := time.Duration(m.healthInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.runHealthCheck()
		}
	}
}

// runHealthCheck sends heartbeats to all connected sessions.
func (m *RemoteManager) runHealthCheck() {
	m.mu.RLock()
	sessions := make(map[string]*RemoteSession, len(m.sessions))
	for id, s := range m.sessions {
		sessions[id] = s
	}
	m.mu.RUnlock()

	for id, s := range sessions {
		if s.State() != StateConnected {
			continue
		}

		heartbeat := NewHeartbeat(fmt.Sprintf("hb-%s-%d", id, time.Now().UnixNano()))
		_ = s.Send(heartbeat)

		lastActivity := s.LastActivity()
		if time.Since(lastActivity) > 3*time.Duration(m.healthInterval)*time.Second {
			if s.autoReconnect {
				go func() {
					_ = s.Reconnect(m.ctx)
				}()
			}
		}
	}
}

// Close shuts down all sessions and stops the manager.
func (m *RemoteManager) Close() error {
	m.cancel()

	m.mu.RLock()
	sessions := make(map[string]*RemoteSession, len(m.sessions))
	for id, s := range m.sessions {
		sessions[id] = s
	}
	m.mu.RUnlock()

	var firstErr error
	for _, s := range sessions {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	m.mu.Lock()
	m.sessions = make(map[string]*RemoteSession)
	m.controls = make(map[string]*RemoteControl)
	m.mu.Unlock()

	return firstErr
}

// ---- Utility Functions ----

// GenerateID creates a simple unique ID based on timestamp.
func GenerateID() string {
	return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}
