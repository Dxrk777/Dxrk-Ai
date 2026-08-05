package notifications

import (
	"fmt"
	"sync"
	"time"
)

// Aggregator groups related notifications and prevents flooding.
type Aggregator struct {
	mu       sync.Mutex
	groups   map[string]*NotificationGroup
	interval time.Duration
}

// NotificationGroup holds related notifications sharing a key.
type NotificationGroup struct {
	Key           string
	Notifications []Notification
	First         time.Time
	Last          time.Time
	Count         int
	Suppressed    int
}

// NewAggregator creates a new Aggregator with the given flush interval.
func NewAggregator(interval time.Duration) *Aggregator {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Aggregator{
		groups:   make(map[string]*NotificationGroup),
		interval: interval,
	}
}

// Add adds a notification to its group. Returns true if the notification
// was added normally, false if it was suppressed due to flooding.
func (a *Aggregator) Add(n Notification) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := groupKey(n)
	group, exists := a.groups[key]
	if !exists {
		group = &NotificationGroup{
			Key:   key,
			First: n.Timestamp,
		}
		a.groups[key] = group
	}

	group.Last = n.Timestamp
	group.Count++

	// Suppress if too many notifications in the interval window.
	if group.Count > 1 && group.Last.Sub(group.First) < a.interval && group.Count > 10 {
		group.Suppressed++
		return false
	}

	group.Notifications = append(group.Notifications, n)
	return true
}

// Flush returns and clears all groups that have notifications.
func (a *Aggregator) Flush() []NotificationGroup {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make([]NotificationGroup, 0, len(a.groups))
	for key, group := range a.groups {
		if group.Count > 0 {
			// Copy the group before returning
			copied := NotificationGroup{
				Key:           group.Key,
				Notifications: make([]Notification, len(group.Notifications)),
				First:         group.First,
				Last:          group.Last,
				Count:         group.Count,
				Suppressed:    group.Suppressed,
			}
			copy(copied.Notifications, group.Notifications)
			result = append(result, copied)
		}
		delete(a.groups, key)
	}
	return result
}

// Summary returns a human-readable summary of a group.
func (a *Aggregator) Summary(groupKey string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	group, exists := a.groups[groupKey]
	if !exists {
		return "no notifications"
	}

	typeCounts := make(map[NotificationType]int)
	for _, n := range group.Notifications {
		typeCounts[n.Type]++
	}

	summary := fmt.Sprintf("Group %q: %d notifications", groupKey, group.Count)
	if group.Suppressed > 0 {
		summary += fmt.Sprintf(" (%d suppressed)", group.Suppressed)
	}
	summary += "\n"

	for t, count := range typeCounts {
		summary += fmt.Sprintf("  %s: %d\n", t.String(), count)
	}

	return summary
}

// ShouldSuppress returns true if the group has too many recent notifications.
func (a *Aggregator) ShouldSuppress(groupKey string, maxPerWindow int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	group, exists := a.groups[groupKey]
	if !exists {
		return false
	}

	if maxPerWindow <= 0 {
		maxPerWindow = 10
	}

	elapsed := time.Since(group.First)
	if elapsed > a.interval {
		// Window expired, reset the group
		group.Count = 0
		group.Suppressed = 0
		group.Notifications = group.Notifications[:0]
		group.First = time.Time{}
		return false
	}

	return group.Count >= maxPerWindow
}

func groupKey(n Notification) string {
	if n.Source != "" {
		return fmt.Sprintf("%s:%d", n.Source, n.Type)
	}
	return fmt.Sprintf("default:%d", n.Type)
}
