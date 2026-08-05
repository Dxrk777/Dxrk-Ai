package notifications

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// Notifier sends notifications through registered channels.
type Notifier struct {
	mu         sync.RWMutex
	channels   []Channel
	history    []Notification
	maxHistory int
	config     NotifierConfig
	recentKeys map[string]time.Time
}

// NotifierConfig configures the notifier behavior.
type NotifierConfig struct {
	MaxHistory  int
	MaxRetries  int
	RetryDelay  time.Duration
	DedupWindow time.Duration
}

// Notification represents a single notification event.
type Notification struct {
	ID        string
	Type      NotificationType
	Title     string
	Message   string
	Details   map[string]any
	Priority  Priority
	Source    string
	Timestamp time.Time
	Read      bool
	Actions   []Action
}

// NotificationType categorizes the notification.
type NotificationType int

const (
	TypeInfo NotificationType = iota
	TypeSuccess
	TypeWarning
	TypeError
	TypeProgress
)

// String returns the string representation of the notification type.
func (t NotificationType) String() string {
	switch t {
	case TypeInfo:
		return "info"
	case TypeSuccess:
		return strconst.StrSuccess
	case TypeWarning:
		return "warning"
	case TypeError:
		return strconst.StrError
	case TypeProgress:
		return strconst.StrProgress
	default:
		return strconst.StrUnknown
	}
}

// Priority indicates notification urgency.
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityUrgent
)

// String returns the string representation of the priority.
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return strconst.StrNormal
	case PriorityHigh:
		return "high"
	case PriorityUrgent:
		return strconst.StrUrgent
	default:
		return strconst.StrUnknown
	}
}

// Action represents an actionable item on a notification.
type Action struct {
	ID    string
	Label string
	Style string // "primary", "secondary", "danger"
}

// Channel is a notification delivery channel.
type Channel interface {
	Name() string
	Send(n Notification) error
	SupportsType(t NotificationType) bool
}

func defaultNotifierConfig() NotifierConfig {
	return NotifierConfig{
		MaxHistory:  100,
		MaxRetries:  3,
		RetryDelay:  100 * time.Millisecond,
		DedupWindow: 5 * time.Second,
	}
}

// NewNotifier creates a new Notifier with the given config.
// Zero-value fields in cfg are filled with defaults.
func NewNotifier(cfg NotifierConfig) *Notifier {
	def := defaultNotifierConfig()
	if cfg.MaxHistory > 0 {
		def.MaxHistory = cfg.MaxHistory
	}
	if cfg.MaxRetries > 0 {
		def.MaxRetries = cfg.MaxRetries
	}
	if cfg.RetryDelay > 0 {
		def.RetryDelay = cfg.RetryDelay
	}
	if cfg.DedupWindow > 0 {
		def.DedupWindow = cfg.DedupWindow
	}
	return &Notifier{
		maxHistory: def.MaxHistory,
		config:     def,
		recentKeys: make(map[string]time.Time),
	}
}

// RegisterChannel adds a delivery channel.
func (n *Notifier) RegisterChannel(ch Channel) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.channels = append(n.channels, ch)
}

// Notify sends a notification through all matching channels.
func (n *Notifier) Notify(notif Notification) error {
	if notif.ID == "" {
		notif.ID = generateID()
	}
	if notif.Timestamp.IsZero() {
		notif.Timestamp = time.Now()
	}

	n.addToHistory(notif)

	n.mu.RLock()
	channels := make([]Channel, len(n.channels))
	copy(channels, n.channels)
	n.mu.RUnlock()

	var lastErr error
	for _, ch := range channels {
		if !ch.SupportsType(notif.Type) {
			continue
		}
		var err error
		for attempt := 0; attempt <= n.config.MaxRetries; attempt++ {
			err = ch.Send(notif)
			if err == nil {
				break
			}
			if attempt < n.config.MaxRetries {
				time.Sleep(n.config.RetryDelay)
			}
		}
		if err != nil {
			lastErr = fmt.Errorf("channel %s: %w", ch.Name(), err)
		}
	}
	return lastErr
}

// Info sends an informational notification.
func (n *Notifier) Info(title, message string) error {
	return n.Notify(Notification{
		Type:     TypeInfo,
		Title:    title,
		Message:  message,
		Priority: PriorityNormal,
	})
}

// Success sends a success notification.
func (n *Notifier) Success(title, message string) error {
	return n.Notify(Notification{
		Type:     TypeSuccess,
		Title:    title,
		Message:  message,
		Priority: PriorityNormal,
	})
}

// Warning sends a warning notification.
func (n *Notifier) Warning(title, message string) error {
	return n.Notify(Notification{
		Type:     TypeWarning,
		Title:    title,
		Message:  message,
		Priority: PriorityHigh,
	})
}

// Error sends an error notification.
func (n *Notifier) Error(title, message string) error {
	return n.Notify(Notification{
		Type:     TypeError,
		Title:    title,
		Message:  message,
		Priority: PriorityUrgent,
	})
}

// Progress sends a progress notification.
func (n *Notifier) Progress(title string, current, total int) error {
	pct := 0
	if total > 0 {
		pct = (current * 100) / total
	}
	return n.Notify(Notification{
		Type:     TypeProgress,
		Title:    title,
		Message:  fmt.Sprintf("%d/%d (%d%%)", current, total, pct),
		Priority: PriorityNormal,
		Details: map[string]any{
			"current": current,
			"total":   total,
			"percent": pct,
		},
	})
}

// History returns the most recent notifications, up to limit.
// If limit <= 0, all history is returned.
func (n *Notifier) History(limit int) []Notification {
	n.mu.RLock()
	defer n.mu.RUnlock()

	total := len(n.history)
	if limit <= 0 || limit > total {
		limit = total
	}

	result := make([]Notification, limit)
	copy(result, n.history[total-limit:])
	return result
}

// MarkRead marks a notification as read by ID. Returns true if found.
func (n *Notifier) MarkRead(id string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i := range n.history {
		if n.history[i].ID == id {
			n.history[i].Read = true
			return true
		}
	}
	return false
}

// UnreadCount returns the number of unread notifications.
func (n *Notifier) UnreadCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	count := 0
	for i := range n.history {
		if !n.history[i].Read {
			count++
		}
	}
	return count
}

// ClearHistory clears all notification history.
func (n *Notifier) ClearHistory() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.history = n.history[:0]
}

// Deduplicate returns true if this notification is a duplicate within the dedup window.
func (n *Notifier) Deduplicate(notif Notification) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	key := deduplicationKey(notif)
	now := time.Now()

	// Clean expired entries
	for k, t := range n.recentKeys {
		if now.Sub(t) > n.config.DedupWindow {
			delete(n.recentKeys, k)
		}
	}

	if _, exists := n.recentKeys[key]; exists {
		return true
	}
	n.recentKeys[key] = now
	return false
}

func (n *Notifier) addToHistory(notif Notification) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.history = append(n.history, notif)
	for len(n.history) > n.maxHistory {
		n.history = n.history[1:]
	}
}

func deduplicationKey(notif Notification) string {
	return fmt.Sprintf("%s:%s:%s", notif.Type, notif.Title, notif.Source)
}

func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
