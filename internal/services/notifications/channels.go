package notifications

import (
	"fmt"
	"io"
	"sync"
)

// InMemoryChannel stores notifications in memory for TUI/Web consumption.
type InMemoryChannel struct {
	notifications []Notification
	mu            sync.Mutex
}

// NewInMemoryChannel creates a new InMemoryChannel.
func NewInMemoryChannel() *InMemoryChannel {
	return &InMemoryChannel{}
}

// Name returns the channel name.
func (c *InMemoryChannel) Name() string {
	return "in_memory"
}

// Send stores the notification in memory.
func (c *InMemoryChannel) Send(n Notification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications = append(c.notifications, n)
	return nil
}

// SupportsType returns true for all notification types.
func (c *InMemoryChannel) SupportsType(_ NotificationType) bool {
	return true
}

// GetNotifications returns a copy of all stored notifications.
func (c *InMemoryChannel) GetNotifications() []Notification {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]Notification, len(c.notifications))
	copy(result, c.notifications)
	return result
}

// Clear removes all stored notifications.
func (c *InMemoryChannel) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications = c.notifications[:0]
}

// BufferedChannel sends notifications to a Go channel.
type BufferedChannel struct {
	ch   chan Notification
	done chan struct{}
}

// NewBufferedChannel creates a new BufferedChannel with the given capacity.
func NewBufferedChannel(capacity int) *BufferedChannel {
	if capacity <= 0 {
		capacity = 64
	}
	return &BufferedChannel{
		ch:   make(chan Notification, capacity),
		done: make(chan struct{}),
	}
}

// Name returns the channel name.
func (c *BufferedChannel) Name() string {
	return "buffered"
}

// Send sends the notification to the buffered channel.
// If the channel is full, the notification is dropped.
func (c *BufferedChannel) Send(n Notification) error {
	select {
	case c.ch <- n:
		return nil
	default:
		return fmt.Errorf("buffered channel full, notification dropped")
	}
}

// SupportsType returns true for all notification types.
func (c *BufferedChannel) SupportsType(_ NotificationType) bool {
	return true
}

// Receive returns the underlying channel for consumption.
func (c *BufferedChannel) Receive() <-chan Notification {
	return c.ch
}

// Close signals that no more notifications will be sent.
func (c *BufferedChannel) Close() {
	close(c.done)
}

// Done returns a channel that is closed when Close is called.
func (c *BufferedChannel) Done() <-chan struct{} {
	return c.done
}

// CallbackChannel sends notifications through a callback function.
type CallbackChannel struct {
	fn func(Notification)
}

// NewCallbackChannel creates a new CallbackChannel with the given function.
func NewCallbackChannel(fn func(Notification)) *CallbackChannel {
	return &CallbackChannel{fn: fn}
}

// Name returns the channel name.
func (c *CallbackChannel) Name() string {
	return "callback"
}

// Send invokes the callback function with the notification.
func (c *CallbackChannel) Send(n Notification) error {
	if c.fn == nil {
		return fmt.Errorf("callback function is nil")
	}
	c.fn(n)
	return nil
}

// SupportsType returns true for all notification types.
func (c *CallbackChannel) SupportsType(_ NotificationType) bool {
	return true
}

// ConsoleChannel outputs notifications to an io.Writer for CLI use.
type ConsoleChannel struct {
	writer   io.Writer
	colorize bool
}

// NewConsoleChannel creates a new ConsoleChannel.
func NewConsoleChannel(w io.Writer, colorize bool) *ConsoleChannel {
	return &ConsoleChannel{writer: w, colorize: colorize}
}

// Name returns the channel name.
func (c *ConsoleChannel) Name() string {
	return "console"
}

// Send writes the notification to the writer.
func (c *ConsoleChannel) Send(n Notification) error {
	var prefix string
	if c.colorize {
		prefix = colorPrefix(n.Type)
	} else {
		prefix = plainPrefix(n.Type)
	}

	line := fmt.Sprintf("%s [%s] %s: %s\n", prefix, n.Timestamp.Format("15:04:05"), n.Title, n.Message)
	_, err := c.writer.Write([]byte(line))
	return err
}

// SupportsType returns true for all notification types.
func (c *ConsoleChannel) SupportsType(_ NotificationType) bool {
	return true
}

func colorPrefix(t NotificationType) string {
	switch t {
	case TypeInfo:
		return "\033[36m[i]\033[0m"
	case TypeSuccess:
		return "\033[32m[+]\033[0m"
	case TypeWarning:
		return "\033[33m[!]\033[0m"
	case TypeError:
		return "\033[31m[x]\033[0m"
	case TypeProgress:
		return "\033[34m[>]\033[0m"
	default:
		return "[?]"
	}
}

func plainPrefix(t NotificationType) string {
	switch t {
	case TypeInfo:
		return "[i]"
	case TypeSuccess:
		return "[+]"
	case TypeWarning:
		return "[!]"
	case TypeError:
		return "[x]"
	case TypeProgress:
		return "[>]"
	default:
		return "[?]"
	}
}
