package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter implements token bucket rate limiting with configurable windows.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*Bucket
	config  LimiterConfig
}

// LimiterConfig holds the default configuration for the rate limiter.
type LimiterConfig struct {
	// DefaultRate is the default requests per second.
	DefaultRate float64
	// DefaultBurst is the default burst size.
	DefaultBurst int
	// DefaultWindow is the default time window.
	DefaultWindow time.Duration
	// MaxConcurrent is the max concurrent requests (0 = unlimited).
	MaxConcurrent int
}

// BucketConfig configures a single rate limit bucket.
type BucketConfig struct {
	Rate       float64
	Burst      int
	Window     time.Duration
	RefillRate float64
}

// Bucket represents a token bucket.
type Bucket struct {
	tokens     float64
	lastRefill time.Time
	config     BucketConfig
	waiting    int
	mu         sync.Mutex
}

// BucketStatus represents the current state of a bucket.
type BucketStatus struct {
	Key             string
	Available       float64
	Max             float64
	WaitingRequests int
	LastAccess      time.Time
}

// NewLimiter creates a new rate limiter with the given configuration.
func NewLimiter(cfg LimiterConfig) *Limiter {
	if cfg.DefaultRate <= 0 {
		cfg.DefaultRate = 10
	}
	if cfg.DefaultBurst <= 0 {
		cfg.DefaultBurst = int(cfg.DefaultRate)
		if cfg.DefaultBurst < 1 {
			cfg.DefaultBurst = 1
		}
	}
	if cfg.DefaultWindow <= 0 {
		cfg.DefaultWindow = time.Second
	}
	return &Limiter{
		buckets: make(map[string]*Bucket),
		config:  cfg,
	}
}

func (l *Limiter) getOrCreateBucket(key string) *Bucket {
	b, ok := l.buckets[key]
	if !ok {
		refillRate := l.config.DefaultRate
		burst := l.config.DefaultBurst
		b = &Bucket{
			tokens:     float64(burst),
			lastRefill: time.Now(),
			config: BucketConfig{
				Rate:       refillRate,
				Burst:      burst,
				Window:     l.config.DefaultWindow,
				RefillRate: refillRate,
			},
		}
		l.buckets[key] = b
	}
	return b
}

// Allow checks if a request is allowed under the rate limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	b := l.getOrCreateBucket(key)
	l.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// AllowWithCost checks if a request with token cost is allowed.
func (l *Limiter) AllowWithCost(key string, cost int) bool {
	if cost <= 0 {
		cost = 1
	}

	l.mu.Lock()
	b := l.getOrCreateBucket(key)
	l.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()
	if b.tokens >= float64(cost) {
		b.tokens -= float64(cost)
		return true
	}
	return false
}

// Wait blocks until a request is allowed or the context is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	key := "_global_"
	l.mu.Lock()
	b := l.getOrCreateBucket(key)
	l.mu.Unlock()

	b.mu.Lock()
	b.waiting++
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.waiting--
		b.mu.Unlock()
	}()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Check context cancellation first to avoid select randomness.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b.mu.Lock()
		b.refill()
		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}
		b.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Reserve reserves a slot and returns a release function.
func (l *Limiter) Reserve(key string) func() {
	l.mu.Lock()
	b := l.getOrCreateBucket(key)
	l.mu.Unlock()

	b.mu.Lock()
	b.refill()
	if b.tokens >= 1 {
		b.tokens--
		b.mu.Unlock()
		return func() {
			b.mu.Lock()
			b.tokens++
			if b.tokens > float64(b.config.Burst) {
				b.tokens = float64(b.config.Burst)
			}
			b.mu.Unlock()
		}
	}
	b.waiting++
	b.mu.Unlock()

	// Spin wait for a token.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				b.mu.Lock()
				b.refill()
				if b.tokens >= 1 {
					b.tokens--
					b.mu.Unlock()
					close(done)
					return
				}
				b.mu.Unlock()
			}
		}
	}()

	<-done

	return func() {
		b.mu.Lock()
		b.tokens++
		if b.tokens > float64(b.config.Burst) {
			b.tokens = float64(b.config.Burst)
		}
		b.mu.Unlock()
	}
}

// Configure sets custom limits for a specific key.
func (l *Limiter) Configure(key string, cfg BucketConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if cfg.Burst <= 0 {
		cfg.Burst = 1
	}
	if cfg.Rate <= 0 {
		cfg.Rate = 1
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Second
	}
	if cfg.RefillRate <= 0 {
		cfg.RefillRate = cfg.Rate
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &Bucket{
			tokens:     float64(cfg.Burst),
			lastRefill: time.Now(),
			config:     cfg,
		}
		l.buckets[key] = b
	} else {
		b.mu.Lock()
		b.config = cfg
		if b.tokens > float64(cfg.Burst) {
			b.tokens = float64(cfg.Burst)
		}
		b.mu.Unlock()
	}
}

// Status returns the current state of a bucket.
func (l *Limiter) Status(key string) BucketStatus {
	l.mu.Lock()
	b, ok := l.buckets[key]
	l.mu.Unlock()

	if !ok {
		return BucketStatus{
			Key:       key,
			Available: 0,
			Max:       0,
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()
	return BucketStatus{
		Key:             key,
		Available:       b.tokens,
		Max:             float64(b.config.Burst),
		WaitingRequests: b.waiting,
		LastAccess:      b.lastRefill,
	}
}

// Reset clears all buckets.
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buckets = make(map[string]*Bucket)
}

// Cleanup removes expired buckets. Returns the number of buckets removed.
func (l *Limiter) Cleanup() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute)
	removed := 0
	for key, b := range l.buckets {
		b.mu.Lock()
		if b.lastRefill.Before(cutoff) {
			delete(l.buckets, key)
			removed++
		}
		b.mu.Unlock()
	}
	return removed
}

// refill replenishes tokens based on elapsed time. Must be called with b.mu held.
func (b *Bucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}
	b.tokens += elapsed * b.config.RefillRate
	if b.tokens > float64(b.config.Burst) {
		b.tokens = float64(b.config.Burst)
	}
	b.lastRefill = now
}
