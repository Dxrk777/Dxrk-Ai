package tokencount

import "sync"

// TokenStatus represents the current state of a running counter.
type TokenStatus struct {
	Total           int
	Max             int
	Remaining       int
	Percent         float64
	Warning         bool
	NeedsCompaction bool
}

// RunningCounter tracks token usage in real-time with configurable limits.
// Safe for concurrent use.
type RunningCounter struct {
	mu               sync.Mutex
	estimator        *TokenEstimator
	totalTokens      int
	maxTokens        int
	warningThreshold float64 // 0.0-1.0, fraction of maxTokens to warn at
	messages         []Message
}

// NewRunningCounter returns a counter with the given max token limit.
// Warning threshold defaults to 0.8 (80%).
func NewRunningCounter(maxTokens int) *RunningCounter {
	return &RunningCounter{
		estimator:        NewEstimator(),
		maxTokens:        maxTokens,
		warningThreshold: 0.8,
		messages:         make([]Message, 0),
	}
}

// AddMessage adds a message and updates the running count.
func (rc *RunningCounter) AddMessage(msg Message) TokenStatus {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	tokens := msg.Tokens
	if tokens <= 0 {
		tokens = rc.estimator.EstimateTokens(msg.Content)
	}

	rc.messages = append(rc.messages, msg)
	rc.totalTokens += tokens

	return rc.statusLocked()
}

// RemoveLastMessage removes the most recent message and recalculates.
func (rc *RunningCounter) RemoveLastMessage() TokenStatus {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if len(rc.messages) == 0 {
		return rc.statusLocked()
	}

	last := rc.messages[len(rc.messages)-1]
	rc.messages = rc.messages[:len(rc.messages)-1]

	tokens := last.Tokens
	if tokens <= 0 {
		tokens = rc.estimator.EstimateTokens(last.Content)
	}
	rc.totalTokens -= tokens
	if rc.totalTokens < 0 {
		rc.totalTokens = 0
	}

	return rc.statusLocked()
}

// Status returns the current token budget status.
func (rc *RunningCounter) Status() TokenStatus {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.statusLocked()
}

func (rc *RunningCounter) statusLocked() TokenStatus {
	remaining := rc.maxTokens - rc.totalTokens
	if remaining < 0 {
		remaining = 0
	}

	var pct float64
	if rc.maxTokens > 0 {
		pct = float64(rc.totalTokens) / float64(rc.maxTokens) * 100.0
	}

	warning := pct >= rc.warningThreshold*100.0
	needsCompaction := float64(rc.totalTokens) >= float64(rc.maxTokens)*rc.warningThreshold

	return TokenStatus{
		Total:           rc.totalTokens,
		Max:             rc.maxTokens,
		Remaining:       remaining,
		Percent:         pct,
		Warning:         warning,
		NeedsCompaction: needsCompaction,
	}
}

// SetMaxTokens updates the max token limit.
func (rc *RunningCounter) SetMaxTokens(max int) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.maxTokens = max
}

// SetWarningThreshold sets when to start warning (0.0-1.0).
func (rc *RunningCounter) SetWarningThreshold(threshold float64) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 1 {
		threshold = 1
	}
	rc.warningThreshold = threshold
}

// Reset clears all tracked messages and counts.
func (rc *RunningCounter) Reset() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.messages = make([]Message, 0)
	rc.totalTokens = 0
}

// NeedsCompaction returns true if tokens exceed the warning threshold.
func (rc *RunningCounter) NeedsCompaction() bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return float64(rc.totalTokens) >= float64(rc.maxTokens)*rc.warningThreshold
}

// MessagesNeedingCompaction returns which messages can be safely compacted.
// Returns messages from oldest that would bring usage below threshold.
func (rc *RunningCounter) MessagesNeedingCompaction() []Message {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	targetTokens := int(float64(rc.maxTokens) * rc.warningThreshold * 0.7)
	if targetTokens < 0 {
		return nil
	}

	excess := rc.totalTokens - targetTokens
	if excess <= 0 {
		return nil
	}

	var candidates []Message
	accumulated := 0
	for _, msg := range rc.messages {
		tokens := msg.Tokens
		if tokens <= 0 {
			tokens = rc.estimator.EstimateTokens(msg.Content)
		}
		accumulated += tokens
		candidates = append(candidates, msg)
		if accumulated >= excess {
			break
		}
	}

	return candidates
}
