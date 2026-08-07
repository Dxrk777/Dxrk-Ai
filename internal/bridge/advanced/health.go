package advanced

import (
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

type HealthState int

const (
	HealthHealthy HealthState = iota
	HealthDegraded
	HealthUnhealthy
	HealthCircuitOpen
)

func (h HealthState) String() string {
	switch h {
	case HealthHealthy:
		return "healthy"
	case HealthDegraded:
		return "degraded"
	case HealthUnhealthy:
		return "unhealthy"
	case HealthCircuitOpen:
		return "circuit_open"
	default:
		return strconst.StrUnknown
	}
}

type HealthCheckFunc func() error

type HealthCheck struct {
	mu       sync.RWMutex
	name     string
	check    HealthCheckFunc
	interval time.Duration
	timeout  time.Duration
	lastRun  time.Time
	lastErr  error
	state    HealthState
	fails    int
	success  int
}

type HealthCheckOption func(*HealthCheck)

func WithHealthInterval(d time.Duration) HealthCheckOption {
	return func(h *HealthCheck) { h.interval = d }
}

func WithHealthTimeout(d time.Duration) HealthCheckOption {
	return func(h *HealthCheck) { h.timeout = d }
}

func NewHealthCheck(name string, check HealthCheckFunc, opts ...HealthCheckOption) *HealthCheck {
	h := &HealthCheck{
		name:     name,
		check:    check,
		interval: 30 * time.Second,
		timeout:  5 * time.Second,
		state:    HealthHealthy,
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

func (h *HealthCheck) Name() string       { return h.name }
func (h *HealthCheck) State() HealthState { h.mu.RLock(); defer h.mu.RUnlock(); return h.state }
func (h *HealthCheck) LastError() error   { h.mu.RLock(); defer h.mu.RUnlock(); return h.lastErr }
func (h *HealthCheck) LastRun() time.Time { h.mu.RLock(); defer h.mu.RUnlock(); return h.lastRun }

func (h *HealthCheck) Run() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastRun = time.Now()
	done := make(chan error, 1)
	go func() {
		done <- h.check()
	}()

	select {
	case err := <-done:
		h.lastErr = err
		if err != nil {
			h.fails++
			h.success = 0
			if h.fails >= 3 {
				h.state = HealthUnhealthy
			} else if h.fails >= 1 {
				h.state = HealthDegraded
			}
		} else {
			h.success++
			h.fails = 0
			if h.success >= 2 {
				h.state = HealthHealthy
			}
		}
		return err
	case <-time.After(h.timeout):
		h.lastErr = errTimeout
		h.fails++
		h.success = 0
		if h.fails >= 3 {
			h.state = HealthUnhealthy
		}
		return errTimeout
	}
}

type CircuitBreaker struct {
	mu           sync.RWMutex
	state        HealthState
	fails        int
	successes    int
	threshold    int
	resetTimeout time.Duration
	lastFail     time.Time
	halfOpen     bool
}

type CircuitBreakerOption func(*CircuitBreaker)

func WithCircuitThreshold(threshold int) CircuitBreakerOption {
	return func(cb *CircuitBreaker) { cb.threshold = threshold }
}

func WithCircuitResetTimeout(d time.Duration) CircuitBreakerOption {
	return func(cb *CircuitBreaker) { cb.resetTimeout = d }
}

func NewCircuitBreaker(opts ...CircuitBreakerOption) *CircuitBreaker {
	cb := &CircuitBreaker{
		state:        HealthHealthy,
		threshold:    5,
		resetTimeout: 30 * time.Second,
	}
	for _, o := range opts {
		o(cb)
	}
	return cb
}

func (cb *CircuitBreaker) State() HealthState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case HealthHealthy:
		return true
	case HealthDegraded:
		return true
	case HealthCircuitOpen:
		if time.Since(cb.lastFail) > cb.resetTimeout {
			cb.state = HealthDegraded
			cb.halfOpen = true
			return true
		}
		return false
	case HealthUnhealthy:
		if time.Since(cb.lastFail) > cb.resetTimeout {
			cb.state = HealthDegraded
			cb.halfOpen = true
			return true
		}
		return false
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.fails = 0
	cb.successes++
	if cb.halfOpen {
		if cb.successes >= 2 {
			cb.state = HealthHealthy
			cb.halfOpen = false
		}
	} else if cb.state == HealthDegraded {
		cb.state = HealthHealthy
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.fails++
	cb.successes = 0
	cb.lastFail = time.Now()

	if cb.halfOpen {
		cb.state = HealthCircuitOpen
		cb.halfOpen = false
		return
	}

	if cb.fails >= cb.threshold {
		cb.state = HealthCircuitOpen
	} else if cb.fails >= cb.threshold/2 {
		cb.state = HealthDegraded
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = HealthHealthy
	cb.fails = 0
	cb.successes = 0
	cb.halfOpen = false
}

func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.fails
}

type HealthManager struct {
	mu           sync.RWMutex
	checks       map[string]*HealthCheck
	breakers     map[string]*CircuitBreaker
	stateHandler func(name string, state HealthState)
}

func NewHealthManager() *HealthManager {
	return &HealthManager{
		checks:   make(map[string]*HealthCheck),
		breakers: make(map[string]*CircuitBreaker),
	}
}

func (m *HealthManager) SetStateHandler(fn func(string, HealthState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateHandler = fn
}

func (m *HealthManager) RegisterCheck(hc *HealthCheck) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checks[hc.name] = hc
}

func (m *HealthManager) RegisterBreaker(name string, cb *CircuitBreaker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.breakers[name] = cb
}

func (m *HealthManager) RunCheck(name string) error {
	m.mu.RLock()
	hc, ok := m.checks[name]
	m.mu.RUnlock()
	if !ok {
		return errNotFound
	}

	prev := hc.State()
	err := hc.Run()
	if hc.State() != prev {
		m.mu.RLock()
		sh := m.stateHandler
		m.mu.RUnlock()
		if sh != nil {
			sh(name, hc.State())
		}
	}
	return err
}

func (m *HealthManager) RunAll() map[string]HealthState {
	m.mu.RLock()
	names := make([]string, 0, len(m.checks))
	for name := range m.checks {
		names = append(names, name)
	}
	m.mu.RUnlock()

	results := make(map[string]HealthState, len(names))
	for _, name := range names {
		_ = m.RunCheck(name)
		m.mu.RLock()
		hc := m.checks[name]
		m.mu.RUnlock()
		results[name] = hc.State()
	}
	return results
}

func (m *HealthManager) OverallState() HealthState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	worst := HealthHealthy
	for _, hc := range m.checks {
		s := hc.State()
		if s > worst {
			worst = s
		}
	}
	return worst
}

func (m *HealthManager) Breaker(name string) (*CircuitBreaker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cb, ok := m.breakers[name]
	return cb, ok
}

var (
	errTimeout  = &timeoutError{}
	errNotFound = &notFoundError{}
)

type timeoutError struct{}

func (e *timeoutError) Error() string { return strconst.StrTimeout }

type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }
