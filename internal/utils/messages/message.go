package messages

import (
	"fmt"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// Role represents the sender of a message.
type Role int

const (
	RoleUser Role = iota
	RoleAssistant
	RoleSystem
	RoleToolUse
	RoleToolResult
)

func (r Role) String() string {
	switch r {
	case RoleUser:
		return "user"
	case RoleAssistant:
		return strconst.StrAssistant
	case RoleSystem:
		return strconst.StrSystem
	case RoleToolUse:
		return strconst.StrToolUse
	case RoleToolResult:
		return strconst.StrToolResult
	default:
		return strconst.StrUnknown
	}
}

// ParseRole converts a string to a Role.
func ParseRole(s string) Role {
	switch strings.ToLower(s) {
	case "user":
		return RoleUser
	case strconst.StrAssistant:
		return RoleAssistant
	case strconst.StrSystem:
		return RoleSystem
	case strconst.StrToolUse:
		return RoleToolUse
	case strconst.StrToolResult:
		return RoleToolResult
	default:
		return RoleUser
	}
}

// ContentType identifies the kind of payload in a Content block.
type ContentType int

const (
	ContentText ContentType = iota
	ContentImage
	ContentToolUse
	ContentToolResult
)

func (ct ContentType) String() string {
	switch ct {
	case ContentText:
		return "text"
	case ContentImage:
		return "image"
	case ContentToolUse:
		return strconst.StrToolUse
	case ContentToolResult:
		return strconst.StrToolResult
	default:
		return strconst.StrUnknown
	}
}

// ImageData holds an image payload.
type ImageData struct {
	Source    string // "base64" or "url"
	MediaType string // e.g. "image/png", "image/jpeg"
	Data      string // base64 data or URL
}

// ToolUseData represents an invocation of a tool by the assistant.
type ToolUseData struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResultData represents the result returned by a tool.
type ToolResultData struct {
	ToolUseID string
	Content   string
	IsError   bool
	Duration  time.Duration
}

// Content is a single block within a message.
type Content struct {
	Type       ContentType
	Text       string
	Image      *ImageData
	ToolUse    *ToolUseData
	ToolResult *ToolResultData
}

// Message is a single conversation turn.
type Message struct {
	ID         string
	Role       Role
	Contents   []Content
	Timestamp  time.Time
	TokenCount int
	Model      string
	StopReason string
	Metadata   map[string]any
}

// EstimateTokens returns the token count for this message.
// If TokenCount is set it is returned directly; otherwise a
// rough estimate (characters / 4) is computed from all text content.
func (m Message) EstimateTokens() int {
	if m.TokenCount > 0 {
		return m.TokenCount
	}
	total := 0
	for _, c := range m.Contents {
		switch c.Type {
		case ContentText:
			total += EstimateTokens(c.Text)
		case ContentToolUse:
			if c.ToolUse != nil {
				total += EstimateTokens(c.ToolUse.Name)
				for k, v := range c.ToolUse.Input {
					total += EstimateTokens(k)
					total += EstimateTokens(fmt.Sprintf("%v", v))
				}
			}
		case ContentToolResult:
			if c.ToolResult != nil {
				total += EstimateTokens(c.ToolResult.Content)
			}
		}
	}
	if total == 0 {
		return 1
	}
	return total
}

// HasToolUse returns true if the message contains a tool_use content block
// with the given name. If name is empty, it checks for any tool use.
func (m Message) HasToolUse(name string) bool {
	for _, c := range m.Contents {
		if c.Type == ContentToolUse && c.ToolUse != nil {
			if name == "" || c.ToolUse.Name == name {
				return true
			}
		}
	}
	return false
}

// TextContent returns the concatenated text of all text content blocks.
func (m Message) TextContent() string {
	var sb strings.Builder
	for _, c := range m.Contents {
		if c.Type == ContentText && c.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

// EstimateTokens returns a rough token count for a string.
// The heuristic is len(s)/4, with a minimum of 1 for non-empty strings.
func EstimateTokens(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	tokens := n / 4
	if tokens == 0 {
		return 1
	}
	return tokens
}

// MessageBuilder provides a fluent API for constructing messages.
type MessageBuilder struct {
	msg Message
}

// NewMessage starts building a message with the given role.
func NewMessage(role Role) *MessageBuilder {
	return &MessageBuilder{
		msg: Message{
			Role:      role,
			Timestamp: time.Now(),
			Contents:  make([]Content, 0, 4),
			Metadata:  make(map[string]any),
		},
	}
}

// WithID sets the message ID.
func (b *MessageBuilder) WithID(id string) *MessageBuilder {
	b.msg.ID = id
	return b
}

// WithTimestamp sets the message timestamp.
func (b *MessageBuilder) WithTimestamp(ts time.Time) *MessageBuilder {
	b.msg.Timestamp = ts
	return b
}

// WithModel sets the model that generated this message.
func (b *MessageBuilder) WithModel(model string) *MessageBuilder {
	b.msg.Model = model
	return b
}

// WithTokenCount sets an explicit token count.
func (b *MessageBuilder) WithTokenCount(n int) *MessageBuilder {
	b.msg.TokenCount = n
	return b
}

// WithStopReason sets the stop reason for assistant messages.
func (b *MessageBuilder) WithStopReason(reason string) *MessageBuilder {
	b.msg.StopReason = reason
	return b
}

// WithMetadata adds a key-value pair to the message metadata.
func (b *MessageBuilder) WithMetadata(key string, value any) *MessageBuilder {
	b.msg.Metadata[key] = value
	return b
}

// Text appends a text content block.
func (b *MessageBuilder) Text(text string) *MessageBuilder {
	b.msg.Contents = append(b.msg.Contents, Content{
		Type: ContentText,
		Text: text,
	})
	return b
}

// Image appends an image content block.
func (b *MessageBuilder) Image(mediaType, data string) *MessageBuilder {
	b.msg.Contents = append(b.msg.Contents, Content{
		Type: ContentImage,
		Image: &ImageData{
			Source:    "base64",
			MediaType: mediaType,
			Data:      data,
		},
	})
	return b
}

// ImageURL appends an image content block sourced from a URL.
func (b *MessageBuilder) ImageURL(url, mediaType string) *MessageBuilder {
	b.msg.Contents = append(b.msg.Contents, Content{
		Type: ContentImage,
		Image: &ImageData{
			Source:    "url",
			MediaType: mediaType,
			Data:      url,
		},
	})
	return b
}

// ToolUse appends a tool_use content block.
func (b *MessageBuilder) ToolUse(id, name string, input map[string]any) *MessageBuilder {
	if input == nil {
		input = make(map[string]any)
	}
	b.msg.Contents = append(b.msg.Contents, Content{
		Type: ContentToolUse,
		ToolUse: &ToolUseData{
			ID:    id,
			Name:  name,
			Input: input,
		},
	})
	return b
}

// ToolResult appends a tool_result content block.
func (b *MessageBuilder) ToolResult(toolUseID, content string, isError bool) *MessageBuilder {
	b.msg.Contents = append(b.msg.Contents, Content{
		Type: ContentToolResult,
		ToolResult: &ToolResultData{
			ToolUseID: toolUseID,
			Content:   content,
			IsError:   isError,
		},
	})
	return b
}

// Build returns the constructed Message.
func (b *MessageBuilder) Build() Message {
	return b.msg
}
