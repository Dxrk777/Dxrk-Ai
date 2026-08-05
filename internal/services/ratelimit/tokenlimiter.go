package ratelimit

import "time"

// TokenLimiter manages rate limits based on token usage per time window.
type TokenLimiter struct {
	limiter *Limiter
	windows map[string]*SlidingWindow
}

// SlidingWindow tracks token usage over a sliding time window.
type SlidingWindow struct {
	entries   []WindowEntry
	maxTokens int
	window    time.Duration
}

// WindowEntry represents a single token usage event.
type WindowEntry struct {
	Tokens    int
	Timestamp time.Time
}

// TokenUsageStats holds usage statistics for a key.
type TokenUsageStats struct {
	Key         string
	Used        int
	Max         int
	WindowStart time.Time
	WindowEnd   time.Time
	Percent     float64
}

// NewTokenLimiter creates a new token limiter.
func NewTokenLimiter(limiter *Limiter) *TokenLimiter {
	return &TokenLimiter{
		limiter: limiter,
		windows: make(map[string]*SlidingWindow),
	}
}

func (tl *TokenLimiter) getWindow(key string) *SlidingWindow {
	w, ok := tl.windows[key]
	if !ok {
		w = &SlidingWindow{
			maxTokens: 100000,
			window:    time.Hour,
		}
		tl.windows[key] = w
	}
	return w
}

// AllowTokens checks if consuming N tokens is within the rate limit.
func (tl *TokenLimiter) AllowTokens(key string, tokens int) bool {
	if tokens <= 0 {
		tokens = 1
	}

	w := tl.getWindow(key)
	w.cleanup(time.Now())

	if w.used()+tokens > w.maxTokens {
		return false
	}

	w.entries = append(w.entries, WindowEntry{
		Tokens:    tokens,
		Timestamp: time.Now(),
	})
	return true
}

// TokenUsage returns token usage stats for a key in a time window.
func (tl *TokenLimiter) TokenUsage(key string, window time.Duration) TokenUsageStats {
	w := tl.getWindow(key)
	now := time.Now()
	w.cleanup(now)

	used := 0
	var windowStart time.Time
	windowEnd := now
	cutoff := now.Add(-window)

	for i := range w.entries {
		if w.entries[i].Timestamp.After(cutoff) {
			used += w.entries[i].Tokens
			if windowStart.IsZero() || w.entries[i].Timestamp.Before(windowStart) {
				windowStart = w.entries[i].Timestamp
			}
		}
	}

	if windowStart.IsZero() {
		windowStart = cutoff
	}

	max := w.maxTokens
	pct := 0.0
	if max > 0 {
		pct = float64(used) / float64(max) * 100
	}

	return TokenUsageStats{
		Key:         key,
		Used:        used,
		Max:         max,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Percent:     pct,
	}
}

// SetTokenLimit sets a maximum tokens-per-window limit.
func (tl *TokenLimiter) SetTokenLimit(key string, maxTokens int, window time.Duration) {
	w := tl.getWindow(key)
	w.maxTokens = maxTokens
	w.window = window
}

// IsOverLimit checks if a key has reached or exceeded its token limit.
func (tl *TokenLimiter) IsOverLimit(key string) bool {
	w := tl.getWindow(key)
	w.cleanup(time.Now())
	return w.used() >= w.maxTokens
}

// ResetWindow resets the sliding window for a key.
func (tl *TokenLimiter) ResetWindow(key string) {
	delete(tl.windows, key)
}

// used returns the total tokens in the current window. Must be called after cleanup.
func (w *SlidingWindow) used() int {
	total := 0
	for i := range w.entries {
		total += w.entries[i].Tokens
	}
	return total
}

// cleanup removes entries outside the window.
func (w *SlidingWindow) cleanup(now time.Time) {
	cutoff := now.Add(-w.window)
	n := 0
	for i := range w.entries {
		if w.entries[i].Timestamp.After(cutoff) {
			w.entries[n] = w.entries[i]
			n++
		}
	}
	w.entries = w.entries[:n]
}
