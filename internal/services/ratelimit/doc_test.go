package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenBucketAllow(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:   5,
		DefaultBurst:  5,
		DefaultWindow: time.Second,
	})

	// Should allow 5 requests (burst).
	for i := 0; i < 5; i++ {
		if !l.Allow("test") {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// 6th should be denied.
	if l.Allow("test") {
		t.Fatal("request 6 should be denied")
	}
}

func TestTokenBucketRefill(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:   10,
		DefaultBurst:  2,
		DefaultWindow: time.Second,
	})

	// Consume burst.
	l.Allow("refill")
	l.Allow("refill")

	// Denied now.
	if l.Allow("refill") {
		t.Fatal("should be denied after burst")
	}

	// Wait for refill.
	time.Sleep(200 * time.Millisecond)

	if !l.Allow("refill") {
		t.Fatal("should be allowed after refill")
	}
}

func TestAllowWithCost(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:   10,
		DefaultBurst:  10,
		DefaultWindow: time.Second,
	})

	// Allow cost of 5.
	if !l.AllowWithCost("cost", 5) {
		t.Fatal("cost 5 should be allowed")
	}

	// Allow cost of 5 again.
	if !l.AllowWithCost("cost", 5) {
		t.Fatal("cost 5 should be allowed second time")
	}

	// Denied now.
	if l.AllowWithCost("cost", 1) {
		t.Fatal("should be denied after consuming 10")
	}
}

func TestBurstHandling(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:   1,
		DefaultBurst:  3,
		DefaultWindow: time.Second,
	})

	// Burst of 3.
	for i := 0; i < 3; i++ {
		if !l.Allow("burst") {
			t.Fatalf("burst request %d should be allowed", i)
		}
	}

	// Denied.
	if l.Allow("burst") {
		t.Fatal("should be denied after burst")
	}
}

func TestSlidingWindowTokenCounting(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:  100,
		DefaultBurst: 100,
	})
	tl := NewTokenLimiter(l)

	tl.SetTokenLimit("window", 100, time.Second)

	// Use 80 tokens.
	for i := 0; i < 80; i++ {
		if !tl.AllowTokens("window", 1) {
			t.Fatal("should allow 80 tokens")
		}
	}

	stats := tl.TokenUsage("window", time.Second)
	if stats.Used != 80 {
		t.Fatalf("expected 80 used, got %d", stats.Used)
	}

	// Use 20 more to hit limit.
	for i := 0; i < 20; i++ {
		if !tl.AllowTokens("window", 1) {
			t.Fatalf("should allow token %d", i)
		}
	}

	// Over limit now.
	if tl.AllowTokens("window", 1) {
		t.Fatal("should deny when over limit")
	}
}

func TestIsOverLimit(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:  100,
		DefaultBurst: 100,
	})
	tl := NewTokenLimiter(l)

	tl.SetTokenLimit("over", 10, time.Hour)

	for i := 0; i < 10; i++ {
		tl.AllowTokens("over", 1)
	}

	if !tl.IsOverLimit("over") {
		t.Fatal("should be at limit at exactly 10")
	}

	tl.AllowTokens("over", 1)

	if !tl.IsOverLimit("over") {
		t.Fatal("should be over limit at 11")
	}
}

func TestResetWindow(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:  100,
		DefaultBurst: 100,
	})
	tl := NewTokenLimiter(l)

	tl.SetTokenLimit("reset", 10, time.Hour)

	for i := 0; i < 10; i++ {
		tl.AllowTokens("reset", 1)
	}

	if tl.AllowTokens("reset", 1) {
		t.Fatal("should be denied")
	}

	tl.ResetWindow("reset")

	if !tl.AllowTokens("reset", 1) {
		t.Fatal("should be allowed after reset")
	}
}

func TestThrottleQueueOrdering(t *testing.T) {
	th := NewThrottle(1, 10)
	ctx := context.Background()

	var order []int
	var mu sync.Mutex

	// Fill the concurrency slot.
	th.Acquire(ctx)

	// Submit low priority first, then high priority.
	th.Submit(ctx, &ThrottledRequest{
		ID:       "low",
		Priority: 1,
		Callback: func() {
			mu.Lock()
			order = append(order, 1)
			mu.Unlock()
		},
	})
	th.Submit(ctx, &ThrottledRequest{
		ID:       "high",
		Priority: 10,
		Callback: func() {
			mu.Lock()
			order = append(order, 10)
			mu.Unlock()
		},
	})

	th.Release()
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if len(order) < 2 {
		t.Fatalf("expected 2 callbacks, got %d", len(order))
	}
	if order[0] != 10 {
		t.Fatalf("expected high priority first, got %d", order[0])
	}
	mu.Unlock()
}

func TestConcurrentAccess(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:   1000,
		DefaultBurst:  100,
		DefaultWindow: time.Second,
	})

	var wg sync.WaitGroup
	var allowed int64

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("concurrent") {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}
	wg.Wait()

	// Should allow at most 100 (burst).
	if allowed > 100 {
		t.Fatalf("expected at most 100 allowed, got %d", allowed)
	}
}

func TestContextCancellationInWait(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:   1,
		DefaultBurst:  1,
		DefaultWindow: time.Second,
	})

	// Consume the "_global_" token (what Wait uses internally).
	l.Allow("_global_")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := l.Wait(ctx)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestConfigureCustomLimits(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:  10,
		DefaultBurst: 10,
	})

	l.Configure("custom", BucketConfig{
		Rate:       3,
		Burst:      3,
		Window:     time.Second,
		RefillRate: 3,
	})

	// Custom bucket should allow only 3.
	for i := 0; i < 3; i++ {
		if !l.Allow("custom") {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	if l.Allow("custom") {
		t.Fatal("should be denied after burst of 3")
	}
}

func TestStatus(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:  5,
		DefaultBurst: 5,
	})

	l.Allow("status")
	l.Allow("status")

	s := l.Status("status")
	if s.Available < 2.9 || s.Available > 3.1 {
		t.Fatalf("expected ~3 available, got %f", s.Available)
	}
	if s.Max != 5 {
		t.Fatalf("expected max 5, got %f", s.Max)
	}
}

func TestCleanup(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:  10,
		DefaultBurst: 10,
	})

	l.Allow("old")
	l.Allow("new")

	// Manually age the "old" bucket.
	l.mu.Lock()
	l.buckets["old"].lastRefill = time.Now().Add(-10 * time.Minute)
	l.mu.Unlock()

	removed := l.Cleanup()
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
}

func TestReset(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:  10,
		DefaultBurst: 10,
	})

	l.Allow("reset")
	l.Allow("reset")

	l.Reset()

	s := l.Status("reset")
	if s.Available != 0 {
		t.Fatalf("expected 0 after reset, got %f", s.Available)
	}
}

func TestReserve(t *testing.T) {
	l := NewLimiter(LimiterConfig{
		DefaultRate:   5,
		DefaultBurst:  5,
		DefaultWindow: time.Second,
	})

	release := l.Reserve("reserve")

	s := l.Status("reserve")
	if s.Available < 3.9 || s.Available > 4.1 {
		t.Fatalf("expected ~4 after reserve, got %f", s.Available)
	}

	release()

	s = l.Status("reserve")
	// After release tokens are restored to burst max; refill may add a tiny amount.
	if s.Available < 4.9 || s.Available > 5.05 {
		t.Fatalf("expected ~5 after release, got %f", s.Available)
	}
}

func TestThrottleStats(t *testing.T) {
	th := NewThrottle(5, 20)

	stats := th.Stats()
	if stats.MaxConcurrent != 5 {
		t.Fatalf("expected max concurrent 5, got %d", stats.MaxConcurrent)
	}
	if stats.MaxQueue != 20 {
		t.Fatalf("expected max queue 20, got %d", stats.MaxQueue)
	}
}
