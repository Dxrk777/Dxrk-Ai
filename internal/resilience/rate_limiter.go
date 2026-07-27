// SPDX-License-Identifier: MIT
package resilience

import (
	"context"
	"sync"
	"time"
)

type RateLimiter struct {
	mu             sync.Mutex
	tokens         float64
	maxTokens      float64
	refillAmount   float64
	refillInterval time.Duration
	stopCh         chan struct{}
	closed         bool
}

func NewRateLimiter() *RateLimiter {
	return NewRateLimiterWithConfig(RateLimiterConfig{
		MaxTokens:      10,
		RefillInterval: time.Second,
		RefillAmount:   10,
	})
}

func NewRateLimiterWithConfig(config RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		tokens:         float64(config.MaxTokens),
		maxTokens:      float64(config.MaxTokens),
		refillAmount:   config.RefillAmount,
		refillInterval: config.RefillInterval,
		stopCh:         make(chan struct{}),
	}
	go rl.refillLoop()
	return rl
}

func (rl *RateLimiter) refillLoop() {
	ticker := time.NewTicker(rl.refillInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			rl.tokens += rl.refillAmount
			if rl.tokens > rl.maxTokens {
				rl.tokens = rl.maxTokens
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (rl *RateLimiter) Close() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.closed {
		return
	}
	rl.closed = true
	close(rl.stopCh)
}
