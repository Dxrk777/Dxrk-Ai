// SPDX-License-Identifier: MIT
package remote

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// MessageType identifies the kind of remote message.
type MessageType string

const (
	MsgRequest      MessageType = "request"
	MsgResponse     MessageType = "response"
	MsgStream       MessageType = "stream"
	MsgEvent        MessageType = "event"
	MsgHeartbeat    MessageType = "heartbeat"
	MsgHeartbeatAck MessageType = "heartbeat_ack"
	MsgError        MessageType = strconst.StrError
	MsgClose        MessageType = "close"
)

// MessagePriority indicates message urgency.
type MessagePriority int

const (
	PriorityLow    MessagePriority = 0
	PriorityNormal MessagePriority = 5
	PriorityHigh   MessagePriority = 10
)

// RemoteMessage is the envelope for all messages exchanged over a remote connection.
type RemoteMessage struct {
	ID        string            `json:"id"`
	Type      MessageType       `json:"type"`
	Method    string            `json:"method,omitempty"`
	Priority  MessagePriority   `json:"priority,omitempty"`
	Payload   json.RawMessage   `json:"payload,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Error     *RemoteError      `json:"error,omitempty"`
}

// RemoteError represents an error in a remote message.
type RemoteError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *RemoteError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// NewRequest creates a new request message.
func NewRequest(id, method string, payload any) (*RemoteMessage, error) {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		raw = data
	}
	return &RemoteMessage{
		ID:        id,
		Type:      MsgRequest,
		Method:    method,
		Payload:   raw,
		Metadata:  make(map[string]string),
		Timestamp: time.Now(),
	}, nil
}

// NewResponse creates a new response message.
func NewResponse(id string, payload any) (*RemoteMessage, error) {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		raw = data
	}
	return &RemoteMessage{
		ID:        id,
		Type:      MsgResponse,
		Payload:   raw,
		Metadata:  make(map[string]string),
		Timestamp: time.Now(),
	}, nil
}

// NewStreamMessage creates a streaming message.
func NewStreamMessage(id string, payload any) (*RemoteMessage, error) {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		raw = data
	}
	return &RemoteMessage{
		ID:        id,
		Type:      MsgStream,
		Payload:   raw,
		Metadata:  make(map[string]string),
		Timestamp: time.Now(),
	}, nil
}

// NewEventMessage creates an event message.
func NewEventMessage(id, event string, payload any) (*RemoteMessage, error) {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		raw = data
	}
	return &RemoteMessage{
		ID:        id,
		Type:      MsgEvent,
		Method:    event,
		Payload:   raw,
		Metadata:  make(map[string]string),
		Timestamp: time.Now(),
	}, nil
}

// NewHeartbeat creates a heartbeat message.
func NewHeartbeat(id string) *RemoteMessage {
	return &RemoteMessage{
		ID:        id,
		Type:      MsgHeartbeat,
		Metadata:  make(map[string]string),
		Timestamp: time.Now(),
	}
}

// NewHeartbeatAck creates a heartbeat acknowledgment.
func NewHeartbeatAck(id string) *RemoteMessage {
	return &RemoteMessage{
		ID:        id,
		Type:      MsgHeartbeatAck,
		Metadata:  make(map[string]string),
		Timestamp: time.Now(),
	}
}

// NewErrorMessage creates an error message.
func NewErrorMessage(id string, code int, message, detail string) *RemoteMessage {
	return &RemoteMessage{
		ID:   id,
		Type: MsgError,
		Error: &RemoteError{
			Code:    code,
			Message: message,
			Detail:  detail,
		},
		Metadata:  make(map[string]string),
		Timestamp: time.Now(),
	}
}

// NewCloseMessage creates a close message.
func NewCloseMessage(id string) *RemoteMessage {
	return &RemoteMessage{
		ID:        id,
		Type:      MsgClose,
		Metadata:  make(map[string]string),
		Timestamp: time.Now(),
	}
}

// Encode serializes the message to JSON bytes.
func (m *RemoteMessage) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage deserializes a message from JSON bytes.
func DecodeMessage(data []byte) (*RemoteMessage, error) {
	var msg RemoteMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("decode message: %w", err)
	}
	return &msg, nil
}

// SetMetadata sets a key-value pair in the message metadata.
func (m *RemoteMessage) SetMetadata(key, value string) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]string)
	}
	m.Metadata[key] = value
}

// GetMetadata retrieves a value from the message metadata.
func (m *RemoteMessage) GetMetadata(key string) (string, bool) {
	if m.Metadata == nil {
		return "", false
	}
	val, ok := m.Metadata[key]
	return val, ok
}

// UnmarshalPayload decodes the payload into the provided value.
func (m *RemoteMessage) UnmarshalPayload(v any) error {
	if len(m.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(m.Payload, v)
}

// IsRequest returns true if the message is a request.
func (m *RemoteMessage) IsRequest() bool {
	return m.Type == MsgRequest
}

// IsResponse returns true if the message is a response.
func (m *RemoteMessage) IsResponse() bool {
	return m.Type == MsgResponse
}

// IsStream returns true if the message is a stream message.
func (m *RemoteMessage) IsStream() bool {
	return m.Type == MsgStream
}

// IsEvent returns true if the message is an event.
func (m *RemoteMessage) IsEvent() bool {
	return m.Type == MsgEvent
}

// IsError returns true if the message is an error.
func (m *RemoteMessage) IsError() bool {
	return m.Type == MsgError
}

// IsClose returns true if the message is a close message.
func (m *RemoteMessage) IsClose() bool {
	return m.Type == MsgClose
}

// IsHeartbeat returns true if the message is a heartbeat.
func (m *RemoteMessage) IsHeartbeat() bool {
	return m.Type == MsgHeartbeat || m.Type == MsgHeartbeatAck
}

// ---- Message Protocol Handler ----

// MessageHandler processes incoming remote messages.
type MessageHandler interface {
	HandleMessage(msg *RemoteMessage) (*RemoteMessage, error)
}

// MessageHandlerFunc is a function adapter for MessageHandler.
type MessageHandlerFunc func(msg *RemoteMessage) (*RemoteMessage, error)

func (f MessageHandlerFunc) HandleMessage(msg *RemoteMessage) (*RemoteMessage, error) {
	return f(msg)
}

// MessageRouter routes messages to handlers by method name.
type MessageRouter struct {
	routes   map[string]MessageHandler
	fallback MessageHandler
}

// NewMessageRouter creates a new message router.
func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		routes: make(map[string]MessageHandler),
	}
}

// Register adds a handler for a given method.
func (r *MessageRouter) Register(method string, handler MessageHandler) {
	r.routes[method] = handler
}

// RegisterFunc registers a function adapter for a given method.
func (r *MessageRouter) RegisterFunc(method string, handler func(msg *RemoteMessage) (*RemoteMessage, error)) {
	r.routes[method] = MessageHandlerFunc(handler)
}

// SetFallback sets the fallback handler for unmatched methods.
func (r *MessageRouter) SetFallback(handler MessageHandler) {
	r.fallback = handler
}

// Route dispatches a message to the appropriate handler.
func (r *MessageRouter) Route(msg *RemoteMessage) (*RemoteMessage, error) {
	if msg.IsRequest() || msg.IsEvent() {
		if handler, ok := r.routes[msg.Method]; ok {
			return handler.HandleMessage(msg)
		}
		if r.fallback != nil {
			return r.fallback.HandleMessage(msg)
		}
		return nil, fmt.Errorf("no handler for method: %s", msg.Method)
	}
	return nil, fmt.Errorf("cannot route message of type: %s", msg.Type)
}
