package messages

import (
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrWindowFull is returned when the context window cannot accept
	// more messages even after compaction.
	ErrWindowFull = errors.New("context window full")

	// ErrNoMessages is returned when compaction is requested on an
	// empty window.
	ErrNoMessages = errors.New("no messages to compact")
)

// CompactStrategy defines how messages are selected for removal.
type CompactStrategy int

const (
	// CompactOldest drops the oldest non-system messages first.
	CompactOldest CompactStrategy = iota
	// CompactToolResults drops tool_result messages first.
	CompactToolResults
	// CompactByImportance scores messages and drops the lowest-scored.
	CompactByImportance
	// CompactRecursive applies compaction repeatedly until budget is met.
	CompactRecursive
)

// MessageScore holds a message and its compaction score.
// Higher score means more important to keep.
type MessageScore struct {
	Message Message
	Score   float64
	Reason  string
}

// ScoreMessages ranks messages by importance for compaction decisions.
// Scoring factors: recency (newer = higher), role priority (user > system >
// assistant > tool), tool error status (errors kept longer), and content
// size (large tool results penalized).
func ScoreMessages(msgs []Message) []MessageScore {
	if len(msgs) == 0 {
		return nil
	}

	scores := make([]MessageScore, len(msgs))
	now := msgs[len(msgs)-1].Timestamp

	roleBase := map[Role]float64{
		RoleUser:       100,
		RoleSystem:     90,
		RoleAssistant:  70,
		RoleToolUse:    50,
		RoleToolResult: 30,
	}

	for i, m := range msgs {
		score := roleBase[m.Role]
		if score == 0 {
			score = 40
		}

		if !now.IsZero() && !m.Timestamp.IsZero() {
			age := now.Sub(m.Timestamp).Minutes()
			recencyBonus := 50.0 / (1.0 + age/30.0)
			score += recencyBonus
		}

		tokenSize := m.EstimateTokens()
		if tokenSize > 500 {
			score -= float64(tokenSize-500) / 100.0
		}

		if m.Role == RoleToolResult {
			for _, c := range m.Contents {
				if c.Type == ContentToolResult && c.ToolResult != nil && c.ToolResult.IsError {
					score += 20
				}
			}
		}

		if i == 0 || i == len(msgs)-1 {
			score += 30
		}

		scores[i] = MessageScore{
			Message: m,
			Score:   score,
			Reason:  fmt.Sprintf("role=%s tokens=%d", m.Role, tokenSize),
		}
	}

	return scores
}

// ContextWindow manages a bounded collection of messages within a token budget.
type ContextWindow struct {
	Messages     []Message
	TokenCount   int
	MaxTokens    int
	SystemPrompt string
	Truncated    bool
}

// NewContextWindow creates a window with the given maximum token budget.
func NewContextWindow(maxTokens int) *ContextWindow {
	return &ContextWindow{
		Messages:  make([]Message, 0, 64),
		MaxTokens: maxTokens,
	}
}

// SetSystemPrompt sets the system prompt, counting its tokens toward the budget.
func (w *ContextWindow) SetSystemPrompt(prompt string) {
	w.SystemPrompt = prompt
	w.recountTokens()
}

// AddMessage adds a message to the window. If the window would overflow,
// older messages are automatically dropped.
func (w *ContextWindow) AddMessage(msg Message) error {
	tokens := msg.EstimateTokens()
	systemTokens := EstimateTokens(w.SystemPrompt)

	if tokens+systemTokens > w.MaxTokens {
		return fmt.Errorf("%w: message (%d tokens) exceeds window budget (%d tokens)", ErrWindowFull, tokens, w.MaxTokens)
	}

	w.Messages = append(w.Messages, msg)
	w.TokenCount += tokens

	for w.TokenCount+systemTokens > w.MaxTokens && len(w.Messages) > 1 {
		w.dropOldest()
		w.Truncated = true
	}

	return nil
}

func (w *ContextWindow) dropOldest() {
	for i, m := range w.Messages {
		if m.Role != RoleSystem {
			w.TokenCount -= m.EstimateTokens()
			w.Messages = append(w.Messages[:i], w.Messages[i+1:]...)
			return
		}
	}
	if len(w.Messages) > 0 {
		w.TokenCount -= w.Messages[0].EstimateTokens()
		w.Messages = w.Messages[1:]
	}
}

// GetMessages returns the current window contents.
func (w *ContextWindow) GetMessages() []Message {
	out := make([]Message, len(w.Messages))
	copy(out, w.Messages)
	return out
}

// RemainingTokens returns how many tokens are left in the budget.
func (w *ContextWindow) RemainingTokens() int {
	systemTokens := EstimateTokens(w.SystemPrompt)
	remaining := w.MaxTokens - w.TokenCount - systemTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// NeedsCompaction returns true if the window is above 80% capacity.
func (w *ContextWindow) NeedsCompaction() bool {
	systemTokens := EstimateTokens(w.SystemPrompt)
	used := w.TokenCount + systemTokens
	return float64(used) >= float64(w.MaxTokens)*0.8
}

// Compact reduces the window contents using the specified strategy.
func (w *ContextWindow) Compact(strategy CompactStrategy) error {
	if len(w.Messages) == 0 {
		return ErrNoMessages
	}

	switch strategy {
	case CompactOldest:
		w.compactOldest()
	case CompactToolResults:
		w.compactToolResults()
	case CompactByImportance:
		w.compactByImportance()
	case CompactRecursive:
		w.compactRecursive()
	default:
		return fmt.Errorf("unknown compact strategy: %d", strategy)
	}

	w.recountTokens()
	return nil
}

func (w *ContextWindow) compactOldest() {
	systemTokens := EstimateTokens(w.SystemPrompt)
	target := w.MaxTokens / 2

	for w.TokenCount+systemTokens > target && len(w.Messages) > 2 {
		w.dropOldest()
		w.Truncated = true
	}
}

func (w *ContextWindow) compactToolResults() {
	systemTokens := EstimateTokens(w.SystemPrompt)
	target := w.MaxTokens / 2

	for i := 0; i < len(w.Messages) && w.TokenCount+systemTokens > target; i++ {
		m := w.Messages[i]
		if m.Role == RoleToolResult {
			w.TokenCount -= m.EstimateTokens()
			w.Messages = append(w.Messages[:i], w.Messages[i+1:]...)
			w.Truncated = true
			i--
		}
	}
}

func (w *ContextWindow) compactByImportance() {
	systemTokens := EstimateTokens(w.SystemPrompt)
	target := w.MaxTokens / 2

	scores := ScoreMessages(w.Messages)
	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].Score < scores[j].Score
	})

	for _, s := range scores {
		if w.TokenCount+systemTokens <= target {
			break
		}
		for i, m := range w.Messages {
			if m.ID == s.Message.ID || m.Timestamp.Equal(s.Message.Timestamp) && m.Role == s.Message.Role {
				w.TokenCount -= m.EstimateTokens()
				w.Messages = append(w.Messages[:i], w.Messages[i+1:]...)
				w.Truncated = true
				break
			}
		}
	}
}

func (w *ContextWindow) compactRecursive() {
	maxIters := 10
	for i := 0; i < maxIters && w.NeedsCompaction(); i++ {
		scores := ScoreMessages(w.Messages)
		if len(scores) == 0 {
			break
		}

		minIdx := 0
		minScore := scores[0].Score
		for j, s := range scores {
			if s.Score < minScore {
				minScore = s.Score
				minIdx = j
			}
		}

		target := scores[minIdx].Message
		for i, m := range w.Messages {
			if m.Timestamp.Equal(target.Timestamp) && m.Role == target.Role {
				w.TokenCount -= m.EstimateTokens()
				w.Messages = append(w.Messages[:i], w.Messages[i+1:]...)
				w.Truncated = true
				break
			}
		}
	}
}

func (w *ContextWindow) recountTokens() {
	w.TokenCount = 0
	for _, m := range w.Messages {
		w.TokenCount += m.EstimateTokens()
	}
}
