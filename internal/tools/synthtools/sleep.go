package synthtools

import (
	"context"
	"fmt"
	"time"
)

// SleepTool provides controlled execution pausing.
type SleepTool struct {
	maxDuration    time.Duration
	allowInterrupt bool
}

// NewSleepTool creates a SleepTool with the given maximum duration.
// The interrupt flag controls whether context cancellation can abort the sleep.
func NewSleepTool(maxDuration time.Duration, allowInterrupt bool) *SleepTool {
	if maxDuration <= 0 {
		maxDuration = 5 * time.Minute
	}
	return &SleepTool{
		maxDuration:    maxDuration,
		allowInterrupt: allowInterrupt,
	}
}

// Sleep pauses execution for the given duration.
// Returns early if the context is cancelled and interrupt is allowed.
func (s *SleepTool) Sleep(ctx context.Context, duration time.Duration) error {
	if duration < 0 {
		return fmt.Errorf("sleep duration must be non-negative")
	}
	if duration > s.maxDuration {
		return fmt.Errorf("sleep duration %v exceeds maximum %v", duration, s.maxDuration)
	}
	if duration == 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	if s.allowInterrupt {
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SleepUntil pauses execution until the specified time.
func (s *SleepTool) SleepUntil(ctx context.Context, until time.Time) error {
	d := time.Until(until)
	if d <= 0 {
		return nil
	}
	return s.Sleep(ctx, d)
}

// WithProgress pauses execution and calls the callback periodically
// with the elapsed time since the sleep started.
func (s *SleepTool) WithProgress(ctx context.Context, duration time.Duration, callback func(elapsed time.Duration)) error {
	if duration < 0 {
		return fmt.Errorf("sleep duration must be non-negative")
	}
	if duration > s.maxDuration {
		return fmt.Errorf("sleep duration %v exceeds maximum %v", duration, s.maxDuration)
	}
	if duration == 0 {
		return nil
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(duration)
	defer timer.Stop()

	start := time.Now()
	for {
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if callback != nil {
				callback(time.Since(start))
			}
		}
	}
}

// BenchmarkSleep measures system sleep accuracy by sleeping for 10ms
// and returning the actual elapsed time.
func BenchmarkSleep() time.Duration {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	return time.Since(start)
}
