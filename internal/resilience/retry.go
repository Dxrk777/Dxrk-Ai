// SPDX-License-Identifier: MIT
package resilience

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

func Do(ctx context.Context, fn func(context.Context) error, config RetryConfig) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := backoffDuration(attempt, config)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		if err := fn(ctx); err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	return lastErr
}

func backoffDuration(attempt int, config RetryConfig) time.Duration {
	delay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt-1))
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}
	if config.Jitter {
		delay *= 0.5 + rand.Float64()*0.5 //nolint:gosec // jitter doesn't need crypto/rand
	}
	return time.Duration(delay)
}
