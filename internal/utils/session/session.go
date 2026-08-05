package session

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// CurrentVersion is the session format version this package produces.
const CurrentVersion = 2

type SessionStatus int

const (
	Active SessionStatus = iota
	Paused
	Completed
	Archived
	Expired
)

func (s SessionStatus) String() string {
	names := [...]string{strconst.StrActive, "paused", strconst.StrCompleted, "archived", "expired"}
	if int(s) < len(names) {
		return names[s]
	}
	return strconst.StrUnknown
}

type MessageRole string

const (
	RoleUser       MessageRole = "user"
	RoleAssistant  MessageRole = strconst.StrAssistant
	RoleSystem     MessageRole = strconst.StrSystem
	RoleToolUse    MessageRole = "toolUse"
	RoleToolResult MessageRole = "toolResult"
)

// Session holds the complete state of an agent conversation.
type Session struct {
	Version      int               `json:"version"`
	ID           string            `json:"id"`
	ParentID     string            `json:"parent_id,omitempty"`
	Title        string            `json:"title"`
	WorkingDir   string            `json:"working_dir"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	MessageCount int               `json:"message_count"`
	TokenCount   int               `json:"token_count"`
	Model        string            `json:"model"`
	Status       SessionStatus     `json:"status"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Summary      string            `json:"summary,omitempty"`
	Messages     []Message         `json:"messages,omitempty"`
}

// Message represents a single turn in a conversation.
type Message struct {
	ID           string            `json:"id"`
	Role         MessageRole       `json:"role"`
	Content      string            `json:"content"`
	Timestamp    time.Time         `json:"timestamp"`
	TokenCount   int               `json:"token_count,omitempty"`
	ToolCalls    []ToolCall        `json:"tool_calls,omitempty"`
	ToolResultID string            `json:"tool_result_id,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ToolCall records a single tool invocation within a message.
type ToolCall struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Input      string  `json:"input"`
	Output     string  `json:"output"`
	Duration   float64 `json:"duration,omitempty"`
	Error      string  `json:"error,omitempty"`
	TokensUsed int     `json:"tokens_used,omitempty"`
}

// SessionOpts configures a new session created via NewSession.
type SessionOpts struct {
	Title       string
	WorkingDir  string
	Model       string
	MaxMessages int
	Metadata    map[string]string
}

// NewSession creates a session with sensible defaults.
func NewSession(opts SessionOpts) *Session {
	now := time.Now()
	s := &Session{
		Version:    CurrentVersion,
		ID:         generateID(),
		Title:      opts.Title,
		WorkingDir: opts.WorkingDir,
		CreatedAt:  now,
		UpdatedAt:  now,
		Model:      opts.Model,
		Status:     Active,
		Metadata:   opts.Metadata,
		Messages:   make([]Message, 0, 64),
	}
	if s.Title == "" {
		s.Title = "Untitled Session"
	}
	return s
}

// AddMessage appends a message and updates counters.
func (s *Session) AddMessage(msg Message) {
	if msg.ID == "" {
		msg.ID = generateID()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.TokenCount == 0 {
		msg.TokenCount = estimateTokens(msg.Content)
	}
	s.Messages = append(s.Messages, msg)
	s.MessageCount = len(s.Messages)
	s.TokenCount += msg.TokenCount
	s.UpdatedAt = time.Now()
}

func (s *Session) GetMessages() []Message {
	out := make([]Message, len(s.Messages))
	copy(out, s.Messages)
	return out
}

func (s *Session) LastMessage() *Message {
	if len(s.Messages) == 0 {
		return nil
	}
	return &s.Messages[len(s.Messages)-1]
}

func (s *Session) Duration() time.Duration {
	if s.UpdatedAt.IsZero() {
		return 0
	}
	return s.UpdatedAt.Sub(s.CreatedAt)
}

func (s *Session) IsExpired(maxAge time.Duration) bool {
	return maxAge > 0 && time.Since(s.UpdatedAt) > maxAge
}

// EstimateTokens recomputes the total token count from all messages.
func (s *Session) EstimateTokens() int {
	total := 0
	for i := range s.Messages {
		total += s.Messages[i].TokenCount
	}
	s.TokenCount = total
	return total
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	words := len(strings.Fields(text))
	chars := len(text)
	tw := words * 4 / 3
	tc := chars / 4
	if tw > tc {
		return tw
	}
	return tc
}
