package hooks

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	ErrExecutionTimeout   = errors.New("hooks: execution timeout")
	ErrMaxRetriesExceeded = errors.New("hooks: max retries exceeded")
	ErrCircuitOpen        = errors.New("hooks: circuit breaker open")
	ErrHookAborted        = errors.New("hooks: hook execution aborted")
)

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern for hook execution.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitBreakerState
	failures         int
	successes        int
	lastFailure      time.Time
	failureThreshold int
	successThreshold int
	timeout          time.Duration
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if successThreshold <= 0 {
		successThreshold = 2
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
	}
}

// Execute runs the given function with circuit breaker protection.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn(ctx)
	cb.recordResult(err)
	return err
}

func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) >= cb.timeout {
			cb.state = CircuitHalfOpen
			cb.successes = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return false
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		if cb.state == CircuitHalfOpen {
			cb.state = CircuitOpen
		} else if cb.failures >= cb.failureThreshold {
			cb.state = CircuitOpen
		}
	} else {
		cb.successes++
		if cb.state == CircuitHalfOpen && cb.successes >= cb.successThreshold {
			cb.state = CircuitClosed
			cb.failures = 0
		}
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
}

// HookExecutor executes hooks with timeout, retry, and circuit breaker.
type HookExecutor struct {
	timeout        time.Duration
	maxRetries     int
	retryDelay     time.Duration
	circuitBreaker *CircuitBreaker
}

// HookExecutorOption configures a HookExecutor.
type HookExecutorOption func(*HookExecutor)

// WithExecutorTimeout sets the execution timeout.
func WithExecutorTimeout(d time.Duration) HookExecutorOption {
	return func(e *HookExecutor) { e.timeout = d }
}

// WithExecutorRetries sets the max retries.
func WithExecutorRetries(n int) HookExecutorOption {
	return func(e *HookExecutor) { e.maxRetries = n }
}

// WithExecutorRetryDelay sets the retry delay.
func WithExecutorRetryDelay(d time.Duration) HookExecutorOption {
	return func(e *HookExecutor) { e.retryDelay = d }
}

// WithCircuitBreaker sets a custom circuit breaker.
func WithCircuitBreaker(cb *CircuitBreaker) HookExecutorOption {
	return func(e *HookExecutor) { e.circuitBreaker = cb }
}

// NewHookExecutor creates a new hook executor.
func NewHookExecutor(opts ...HookExecutorOption) *HookExecutor {
	e := &HookExecutor{
		timeout:        30 * time.Second,
		maxRetries:     3,
		retryDelay:     1 * time.Second,
		circuitBreaker: NewCircuitBreaker(5, 2, 30*time.Second),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute runs a hook command with the given context and configuration.
func (e *HookExecutor) Execute(ctx context.Context, config HookConfig, event HookEvent) HookResult {
	start := time.Now()
	result := HookResult{
		Success:  false,
		Duration: 0,
	}

	var lastErr error
	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				result.Error = ctx.Err().Error()
				result.Duration = time.Since(start)
				return result
			case <-time.After(e.retryDelay):
			}
		}

		execCtx, cancel := context.WithTimeout(ctx, e.timeout)
		err := e.executeOnce(execCtx, config, event, &result, attempt)
		cancel()

		if err == nil {
			result.Success = true
			result.Duration = time.Since(start)
			return result
		}

		lastErr = err
		if errors.Is(err, ErrExecutionTimeout) || errors.Is(err, ErrHookAborted) {
			break
		}
	}

	result.Success = false
	result.Error = lastErr.Error()
	result.Duration = time.Since(start)
	return result
}

func (e *HookExecutor) executeOnce(ctx context.Context, config HookConfig, _ HookEvent, result *HookResult, _ int) error {
	return e.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		cmd := exec.CommandContext(ctx, config.Command, config.Args...)
		cmd.Env = append(os.Environ(), config.Env...)

		stdout, stderr := &strings.Builder{}, &strings.Builder{}
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		err := cmd.Run()
		result.ExitCode = cmd.ProcessState.ExitCode()
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()

		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return ErrExecutionTimeout
			}
			return err
		}

		return nil
	})
}
