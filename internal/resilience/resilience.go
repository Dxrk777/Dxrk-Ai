// SPDX-License-Identifier: MIT
package resilience

import "time"

type Options struct {
	Retry     RetryConfig
	CB        CircuitBreakerConfig
	RateLimit RateLimiterConfig
}

type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	Jitter        bool
}

type CircuitBreakerConfig struct {
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
}

type RateLimiterConfig struct {
	MaxTokens      int
	RefillInterval time.Duration
	RefillAmount   float64
}

func NewRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	}
}
