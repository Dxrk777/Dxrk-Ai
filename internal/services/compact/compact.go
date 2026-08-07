package compact

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

type CompactLevel int

const (
	CompactFull CompactLevel = iota
	CompactMicro
	CompactSessionMemory
)

type Message struct {
	Role      string
	Content   string
	Timestamp time.Time
	Tokens    int
}

func (m *Message) EstimateTokens() int {
	if m.Tokens > 0 {
		return m.Tokens
	}
	n := len(m.Content)
	if n == 0 {
		return 0
	}
	return n / 4
}

type CompactResult struct {
	Level           CompactLevel
	OriginalTokens  int
	CompactedTokens int
	MessagesKept    int
	MessagesDropped int
	Duration        time.Duration
	Summary         string
}

type AutoCompactConfig struct {
	Enabled              bool
	ThresholdTokens      int
	MaxTokens            int
	MicroCompactEnabled  bool
	SessionMemoryEnabled bool
}

func defaultConfig() AutoCompactConfig {
	return AutoCompactConfig{
		Enabled:              true,
		ThresholdTokens:      180000,
		MaxTokens:            200000,
		MicroCompactEnabled:  true,
		SessionMemoryEnabled: true,
	}
}

type CompactService struct {
	config              AutoCompactConfig
	mu                  sync.Mutex
	sessionCompactCount int
	lastMicroCompact    time.Time
}

func NewCompactService(config AutoCompactConfig) *CompactService {
	cfg := defaultConfig()
	if config.ThresholdTokens > 0 {
		cfg.ThresholdTokens = config.ThresholdTokens
	}
	if config.MaxTokens > 0 {
		cfg.MaxTokens = config.MaxTokens
	}
	cfg.Enabled = config.Enabled
	cfg.MicroCompactEnabled = config.MicroCompactEnabled
	cfg.SessionMemoryEnabled = config.SessionMemoryEnabled
	return &CompactService{config: cfg}
}

func (s *CompactService) ShouldCompact(currentTokens int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return currentTokens > s.config.ThresholdTokens
}

func (s *CompactService) Compact(ctx context.Context, messages []Message, level CompactLevel) (*CompactResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	switch level {
	case CompactFull:
		return s.FullCompact(ctx, messages)
	case CompactMicro:
		return s.MicroCompact(ctx, messages, "time_based")
	case CompactSessionMemory:
		return s.SessionMemoryCompact(ctx, messages, "")
	default:
		return nil, fmt.Errorf("unknown compact level: %d", level)
	}
}

func (s *CompactService) FullCompact(ctx context.Context, messages []Message) (*CompactResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	start := time.Now()

	if len(messages) == 0 {
		return &CompactResult{
			Level:           CompactFull,
			OriginalTokens:  0,
			CompactedTokens: 0,
			MessagesKept:    0,
			MessagesDropped: 0,
			Duration:        time.Since(start),
			Summary:         "",
		}, nil
	}

	originalTokens := 0
	for i := range messages {
		originalTokens += messages[i].EstimateTokens()
	}

	first := messages[0]
	recentCount := len(messages) / 4
	if recentCount < 2 {
		recentCount = 2
	}
	if recentCount > len(messages)-1 {
		recentCount = len(messages) - 1
	}

	kept := make([]Message, 0, 1+recentCount)
	kept = append(kept, first)
	kept = append(kept, messages[len(messages)-recentCount:]...)

	compactedTokens := 0
	for i := range kept {
		compactedTokens += kept[i].EstimateTokens()
	}

	summaryParts := make([]string, 0, len(kept))
	for i := range kept {
		summaryParts = append(summaryParts, fmt.Sprintf("[%s] %s", kept[i].Role, truncate(kept[i].Content, 120)))
	}

	return &CompactResult{
		Level:           CompactFull,
		OriginalTokens:  originalTokens,
		CompactedTokens: compactedTokens,
		MessagesKept:    len(kept),
		MessagesDropped: len(messages) - len(kept),
		Duration:        time.Since(start),
		Summary:         strings.Join(summaryParts, "\n"),
	}, nil
}

func (s *CompactService) MicroCompact(ctx context.Context, messages []Message, strategy string) (*CompactResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	start := time.Now()

	if !s.config.MicroCompactEnabled {
		return &CompactResult{
			Level:           CompactMicro,
			OriginalTokens:  0,
			CompactedTokens: 0,
			MessagesKept:    len(messages),
			MessagesDropped: 0,
			Duration:        time.Since(start),
			Summary:         "micro compact disabled",
		}, nil
	}

	originalTokens := 0
	for i := range messages {
		originalTokens += messages[i].EstimateTokens()
	}

	var result []Message
	switch strategy {
	case "time_based":
		result = s.applyTimeBasedMicroCompact(messages)
	case "content_based":
		result = s.applyContentBasedMicroCompact(messages)
	default:
		result = messages
	}

	compactedTokens := 0
	for i := range result {
		compactedTokens += result[i].EstimateTokens()
	}

	s.mu.Lock()
	s.lastMicroCompact = time.Now()
	s.mu.Unlock()

	return &CompactResult{
		Level:           CompactMicro,
		OriginalTokens:  originalTokens,
		CompactedTokens: compactedTokens,
		MessagesKept:    len(result),
		MessagesDropped: len(messages) - len(result),
		Duration:        time.Since(start),
		Summary:         fmt.Sprintf("micro compact (%s): kept %d, dropped %d", strategy, len(result), len(messages)-len(result)),
	}, nil
}

func (s *CompactService) applyTimeBasedMicroCompact(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}

	gapThreshold := 60 * time.Minute
	now := time.Now()
	lastAssistantIdx := -1

	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == strconst.StrAssistant {
			lastAssistantIdx = i
			break
		}
	}

	if lastAssistantIdx < 0 {
		return messages
	}

	gap := now.Sub(messages[lastAssistantIdx].Timestamp)
	if gap < gapThreshold {
		return messages
	}

	keepRecent := 5
	result := make([]Message, 0, len(messages))
	for i, msg := range messages {
		if msg.Role == "user" && i < len(messages)-keepRecent {
			cleared := msg
			cleared.Content = "[Old tool result content cleared]"
			result = append(result, cleared)
		} else {
			result = append(result, msg)
		}
	}

	return result
}

func (s *CompactService) applyContentBasedMicroCompact(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}

	result := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "user" && len(msg.Content) > 10000 {
			trimmed := msg
			trimmed.Content = truncate(trimmed.Content, 500) + "\n...[content truncated for compaction]"
			result = append(result, trimmed)
		} else {
			result = append(result, msg)
		}
	}

	return result
}

func (s *CompactService) SessionMemoryCompact(ctx context.Context, messages []Message, sessionNotes string) (*CompactResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	start := time.Now()

	if !s.config.SessionMemoryEnabled {
		return &CompactResult{
			Level:           CompactSessionMemory,
			OriginalTokens:  0,
			CompactedTokens: 0,
			MessagesKept:    len(messages),
			MessagesDropped: 0,
			Duration:        time.Since(start),
			Summary:         "session memory compact disabled",
		}, nil
	}

	originalTokens := 0
	for i := range messages {
		originalTokens += messages[i].EstimateTokens()
	}

	if len(messages) == 0 {
		return &CompactResult{
			Level:           CompactSessionMemory,
			OriginalTokens:  0,
			CompactedTokens: 0,
			MessagesKept:    0,
			MessagesDropped: 0,
			Duration:        time.Since(start),
			Summary:         "no messages to compact",
		}, nil
	}

	minTokens := 10000
	minTextMessages := 5
	maxTokens := 40000

	startIdx := len(messages)
	totalTokens := 0
	textMsgCount := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := messages[i].EstimateTokens()
		totalTokens += msgTokens
		if messages[i].Content != "" {
			textMsgCount++
		}
		startIdx = i

		if totalTokens >= maxTokens {
			break
		}
		if totalTokens >= minTokens && textMsgCount >= minTextMessages {
			break
		}
	}

	messagesToKeep := messages[startIdx:]
	compactedTokens := 0
	for i := range messagesToKeep {
		compactedTokens += messagesToKeep[i].EstimateTokens()
	}

	summaryParts := []string{
		fmt.Sprintf("Session memory compact: %d messages, %d tokens", len(messagesToKeep), compactedTokens),
	}
	if sessionNotes != "" {
		summaryParts = append(summaryParts, fmt.Sprintf("Session notes: %s", truncate(sessionNotes, 200)))
	}

	s.mu.Lock()
	s.sessionCompactCount++
	s.mu.Unlock()

	return &CompactResult{
		Level:           CompactSessionMemory,
		OriginalTokens:  originalTokens,
		CompactedTokens: compactedTokens,
		MessagesKept:    len(messagesToKeep),
		MessagesDropped: len(messages) - len(messagesToKeep),
		Duration:        time.Since(start),
		Summary:         strings.Join(summaryParts, "\n"),
	}, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
