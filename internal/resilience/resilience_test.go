// SPDX-License-Identifier: MIT
package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		succeedOn    int
		config       RetryConfig
		wantErr      bool
		wantAttempts int
	}{
		{
			name:         "success on first attempt",
			succeedOn:    1,
			config:       NewRetryConfig(),
			wantErr:      false,
			wantAttempts: 1,
		},
		{
			name:      "success after retry",
			succeedOn: 2,
			config: RetryConfig{
				MaxAttempts:   3,
				InitialDelay:  time.Millisecond,
				MaxDelay:      time.Millisecond,
				BackoffFactor: 1.0,
			},
			wantErr:      false,
			wantAttempts: 2,
		},
		{
			name:      "success on last attempt",
			succeedOn: 3,
			config: RetryConfig{
				MaxAttempts:   3,
				InitialDelay:  time.Millisecond,
				MaxDelay:      time.Millisecond,
				BackoffFactor: 1.0,
			},
			wantErr:      false,
			wantAttempts: 3,
		},
		{
			name:      "failure after max attempts",
			succeedOn: 99,
			config: RetryConfig{
				MaxAttempts:   3,
				InitialDelay:  time.Millisecond,
				MaxDelay:      time.Millisecond,
				BackoffFactor: 1.0,
			},
			wantErr:      true,
			wantAttempts: 3,
		},
		{
			name:      "single attempt succeeds",
			succeedOn: 1,
			config: RetryConfig{
				MaxAttempts: 1,
			},
			wantErr:      false,
			wantAttempts: 1,
		},
		{
			name:      "single attempt fails",
			succeedOn: 99,
			config: RetryConfig{
				MaxAttempts: 1,
			},
			wantErr:      true,
			wantAttempts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var count int
			err := Do(context.Background(), func(_ context.Context) error {
				count++
				if count < tt.succeedOn {
					return errors.New("retry")
				}
				return nil
			}, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("Do() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if count != tt.wantAttempts {
				t.Errorf("Do() attempts = %d, want %d", count, tt.wantAttempts)
			}
		})
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var count atomic.Int32
	err := Do(ctx, func(_ context.Context) error {
		count.Add(1)
		return errors.New("fail")
	}, RetryConfig{
		MaxAttempts:   10,
		InitialDelay:  time.Hour,
		MaxDelay:      time.Hour,
		BackoffFactor: 1.0,
	})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if count.Load() != 1 {
		t.Errorf("expected 1 attempt, got %d", count.Load())
	}
}

func TestDo_BackoffCapsAtMaxDelay(t *testing.T) {
	t.Parallel()

	config := RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  time.Second,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 100,
	}

	var count int
	start := time.Now()
	err := Do(context.Background(), func(_ context.Context) error {
		count++
		if count < 5 {
			return errors.New("fail")
		}
		return nil
	}, config)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 attempts, got %d", count)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("backoff exceeded cap, took %v", elapsed)
	}
}

func TestCircuitBreaker_InitialState(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker()
	if got := cb.State(); got != StateClosed {
		t.Errorf("expected StateClosed, got %v", got)
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})

	for range 2 {
		cb.Call(context.Background(), func(_ context.Context) error {
			return errors.New("fail")
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after threshold, got %v", cb.State())
	}
}

func TestCircuitBreaker_RejectsWhenOpen(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour,
	})

	cb.Call(context.Background(), func(_ context.Context) error {
		return errors.New("fail")
	})

	err := cb.Call(context.Background(), func(_ context.Context) error {
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
	})

	cb.Call(context.Background(), func(_ context.Context) error {
		return errors.New("fail")
	})

	time.Sleep(60 * time.Millisecond)

	err := cb.Call(context.Background(), func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("expected probe to succeed, got %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after half-open success, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailReturnsToOpen(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
	})

	cb.Call(context.Background(), func(_ context.Context) error {
		return errors.New("fail")
	})

	time.Sleep(60 * time.Millisecond)

	cb.Call(context.Background(), func(_ context.Context) error {
		return errors.New("probe fail")
	})

	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after half-open failure, got %v", cb.State())
	}
}

func TestCircuitBreaker_ClosedResetsFailureCount(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})

	for range 2 {
		cb.Call(context.Background(), func(_ context.Context) error {
			return errors.New("fail")
		})
	}

	cb.Call(context.Background(), func(_ context.Context) error {
		return nil
	})

	cb.Call(context.Background(), func(_ context.Context) error {
		return errors.New("fail")
	})

	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed when under threshold, got %v", cb.State())
	}
}

func TestCircuitBreaker_RequiresSuccessThreshold(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreakerWithConfig(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 3,
		Timeout:          50 * time.Millisecond,
	})

	cb.Call(context.Background(), func(_ context.Context) error {
		return errors.New("fail")
	})

	time.Sleep(60 * time.Millisecond)

	for range 2 {
		err := cb.Call(context.Background(), func(_ context.Context) error {
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cb.State() != StateHalfOpen {
			t.Fatalf("expected StateHalfOpen after %d successes, got %v", 2, cb.State())
		}
	}

	err := cb.Call(context.Background(), func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after success threshold, got %v", cb.State())
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		max      int
		consume  int
		wantOk   int
		wantDeny int
	}{
		{
			name:     "all tokens available",
			max:      3,
			consume:  3,
			wantOk:   3,
			wantDeny: 0,
		},
		{
			name:     "exhaust tokens",
			max:      2,
			consume:  5,
			wantOk:   2,
			wantDeny: 3,
		},
		{
			name:     "zero tokens",
			max:      0,
			consume:  1,
			wantOk:   0,
			wantDeny: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiterWithConfig(RateLimiterConfig{
				MaxTokens:      tt.max,
				RefillInterval: time.Hour,
				RefillAmount:   0,
			})
			defer rl.Close()

			var okCount, denyCount int
			for range tt.consume {
				if rl.Allow() {
					okCount++
				} else {
					denyCount++
				}
			}

			if okCount != tt.wantOk {
				t.Errorf("allowed = %d, want %d", okCount, tt.wantOk)
			}
			if denyCount != tt.wantDeny {
				t.Errorf("denied = %d, want %d", denyCount, tt.wantDeny)
			}
		})
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiterWithConfig(RateLimiterConfig{
		MaxTokens:      1,
		RefillInterval: 50 * time.Millisecond,
		RefillAmount:   1,
	})
	defer rl.Close()

	rl.Allow()

	start := time.Now()
	err := rl.Wait(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed < 30*time.Millisecond {
		t.Errorf("Wait returned too fast: %v", elapsed)
	}
}

func TestRateLimiter_WaitContextCancel(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiterWithConfig(RateLimiterConfig{
		MaxTokens:      0,
		RefillInterval: time.Hour,
		RefillAmount:   0,
	})
	defer rl.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rl.Wait(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiterWithConfig(RateLimiterConfig{
		MaxTokens:      5,
		RefillInterval: 10 * time.Millisecond,
		RefillAmount:   3,
	})
	defer rl.Close()

	for range 5 {
		rl.Allow()
	}
	if rl.Allow() {
		t.Fatal("expected tokens exhausted")
	}

	time.Sleep(15 * time.Millisecond)

	if !rl.Allow() {
		t.Errorf("expected token after refill")
	}
}
