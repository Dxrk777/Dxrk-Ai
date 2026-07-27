// SPDX-License-Identifier: MIT
package query

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

// RetryProvider wraps a Provider with exponential backoff retry.
// When maxRetries is exceeded, it can fall back to a secondary provider.
type RetryProvider struct {
	primary    Provider
	fallback   Provider
	maxRetries int
	baseDelay  time.Duration
}

// RetryOption configures the RetryProvider.
type RetryOption func(*RetryProvider)

// WithFallback sets the fallback provider used when all retries are exhausted.
func WithFallback(p Provider) RetryOption {
	return func(rp *RetryProvider) { rp.fallback = p }
}

// WithMaxRetries sets the maximum number of retries (default 3).
func WithMaxRetries(n int) RetryOption {
	return func(rp *RetryProvider) { rp.maxRetries = n }
}

// WithBaseDelay sets the initial backoff delay (default 1s).
func WithBaseDelay(d time.Duration) RetryOption {
	return func(rp *RetryProvider) { rp.baseDelay = d }
}

// NewRetryProvider creates a RetryProvider that wraps a primary provider.
func NewRetryProvider(primary Provider, opts ...RetryOption) *RetryProvider {
	rp := &RetryProvider{
		primary:    primary,
		maxRetries: 3,
		baseDelay:  time.Second,
	}
	for _, opt := range opts {
		opt(rp)
	}
	return rp
}

// Generate calls the primary provider with retry on failure.
// Falls back to the secondary provider if all retries are exhausted.
func (rp *RetryProvider) Generate(ctx context.Context, messages []Message, tools []ToolSchema) (Response, error) {
	var lastErr error

	for attempt := 0; attempt <= rp.maxRetries; attempt++ {
		if attempt > 0 {
			delay := rp.baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
			jitter := time.Duration(rand.Int64N(int64(delay / 2))) //nolint:gosec
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			case <-time.After(delay + jitter):
			}
		}

		resp, err := rp.primary.Generate(ctx, messages, tools)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}

	if rp.fallback != nil {
		resp, err := rp.fallback.Generate(ctx, messages, tools)
		if err != nil {
			return Response{}, fmt.Errorf("retry+fallback: primary: %w, fallback: %w", lastErr, err)
		}
		return resp, nil
	}

	return Response{}, fmt.Errorf("retry exhausted: %w", lastErr)
}
