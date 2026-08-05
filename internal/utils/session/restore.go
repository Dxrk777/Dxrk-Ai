package session

import (
	"fmt"
	"strings"
	"time"
)

// ResumeContext holds everything needed to continue a session.
type ResumeContext struct {
	Session       *Session
	LastSummary   string
	PendingTools  []ToolCall
	ContextWindow int
	TokenBudget   int
}

type ResumeCriteria struct {
	MaxMessagesBack int
	PreferAfterTool bool
	MaxTokens       int
}

// RestoreSession loads and validates a session from storage.
func RestoreSession(id string, storage Storage) (*Session, error) {
	s, err := storage.Load(id)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	if s == nil || s.ID == "" {
		return nil, fmt.Errorf("session %q invalid", id)
	}
	if s.Version > CurrentVersion {
		return nil, fmt.Errorf("session version %d exceeds current version %d", s.Version, CurrentVersion)
	}
	return s, nil
}

// ResumeSession prepares a session for continuation.
func ResumeSession(s *Session) *ResumeContext {
	pending := collectPendingToolCalls(s)
	summary := buildIncrementalSummary(s)
	tokens := 0
	for _, msg := range s.Messages {
		tokens += msg.TokenCount
	}
	return &ResumeContext{Session: s, LastSummary: summary, PendingTools: pending,
		ContextWindow: len(s.Messages), TokenBudget: tokens}
}

// CreateSummary generates a text summary within a token budget.
func CreateSummary(s *Session, maxTokens int) (*string, error) {
	if s == nil {
		return nil, fmt.Errorf("session is nil")
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	used := 0
	kept := make([]Message, 0, len(s.Messages))
	for i := len(s.Messages) - 1; i >= 0; i-- {
		t := s.Messages[i].TokenCount
		if used+t > maxTokens {
			break
		}
		kept = append(kept, s.Messages[i])
		used += t
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	prefix := fmt.Sprintf("Session %q (%d messages, ~%d tokens).\n", s.Title, len(s.Messages), s.TokenCount)
	if len(kept) < len(s.Messages) {
		prefix = fmt.Sprintf("Session %q (%d of %d messages, ~%d tokens).\n", s.Title, len(kept), len(s.Messages), used)
	}
	result := prefix + buildMessageSummary(kept)
	return &result, nil
}

// FindResumePoint returns the best message index to resume from.
func FindResumePoint(s *Session, criteria ResumeCriteria) (int, error) {
	if s == nil || len(s.Messages) == 0 {
		return 0, fmt.Errorf("session is nil or empty")
	}
	maxBack := criteria.MaxMessagesBack
	if maxBack <= 0 || maxBack > len(s.Messages) {
		maxBack = len(s.Messages)
	}
	start := len(s.Messages) - maxBack
	if start < 0 {
		start = 0
	}
	if criteria.PreferAfterTool {
		for i := len(s.Messages) - 1; i >= start; i-- {
			if len(s.Messages[i].ToolCalls) > 0 || s.Messages[i].Role == RoleToolResult {
				idx := i + 1
				if idx > len(s.Messages) {
					idx = len(s.Messages)
				}
				return idx, nil
			}
		}
	}
	if criteria.MaxTokens > 0 {
		used := 0
		for i := len(s.Messages) - 1; i >= start; i-- {
			used += s.Messages[i].TokenCount
			if used > criteria.MaxTokens {
				return i + 1, nil
			}
		}
	}
	return start, nil
}

// AutoArchive returns true if the session should be archived based on age.
func AutoArchive(s *Session, maxAge time.Duration) bool {
	if s == nil || maxAge <= 0 {
		return false
	}
	if s.Status == Archived || s.Status == Expired || s.Status == Completed {
		return false
	}
	return time.Since(s.UpdatedAt) > maxAge
}

// CleanupExpired marks sessions older than maxAge as Expired.
func CleanupExpired(storage Storage, maxAge time.Duration) (int, error) {
	sessions, err := storage.List(ListOpts{Limit: 0})
	if err != nil {
		return 0, fmt.Errorf("list sessions: %w", err)
	}
	count := 0
	for _, summary := range sessions {
		s, err := storage.Load(summary.ID)
		if err != nil {
			continue
		}
		if s.IsExpired(maxAge) && (s.Status == Active || s.Status == Paused) {
			s.Status = Expired
			if err := storage.Save(s); err == nil {
				count++
			}
		} else if AutoArchive(s, maxAge) {
			s.Status = Archived
			if err := storage.Save(s); err == nil {
				count++
			}
		}
	}
	return count, nil
}

func collectPendingToolCalls(s *Session) []ToolCall {
	var pending []ToolCall
	for _, msg := range s.Messages {
		for _, tc := range msg.ToolCalls {
			if tc.Error != "" || tc.Output == "" {
				pending = append(pending, tc)
			}
		}
	}
	return pending
}

func buildIncrementalSummary(s *Session) string {
	if len(s.Messages) == 0 {
		return ""
	}
	last := s.Messages[len(s.Messages)-1]
	summary := fmt.Sprintf("Last message role: %s", last.Role)
	switch last.Role {
	case RoleAssistant:
		if last.Content != "" {
			summary = fmt.Sprintf("Last assistant: %s", truncate(last.Content, 200))
		}
	case RoleUser:
		summary = fmt.Sprintf("Awaiting response to: %s", truncate(last.Content, 200))
	}
	if len(last.ToolCalls) > 0 {
		summary += fmt.Sprintf(" (%d tool calls pending)", len(last.ToolCalls))
	}
	return summary
}

func buildMessageSummary(msgs []Message) string {
	var sb strings.Builder
	for _, msg := range msgs {
		fmt.Fprintf(&sb, "[%s] %s\n", msg.Role, truncate(msg.Content, 120))
	}
	return sb.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
