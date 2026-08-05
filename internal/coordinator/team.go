package coordinator

import (
	"fmt"
	"sync"
	"time"
)

// Message represents an inter-agent message.
type Message struct {
	From      string    `json:"from"`
	To        string    `json:"to"` // agent ID or "all" for broadcast
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Team represents a group of worker agents.
type Team struct {
	mu       sync.RWMutex
	Name     string    `json:"name"`
	Members  []*Worker `json:"members"`
	Messages []Message `json:"messages"`
}

// NewTeam creates a new team with the given name and member IDs.
func NewTeam(name string, memberIDs []string) *Team {
	members := make([]*Worker, 0, len(memberIDs))
	for _, id := range memberIDs {
		members = append(members, NewWorker(id))
	}
	return &Team{
		Name:     name,
		Members:  members,
		Messages: make([]Message, 0),
	}
}

// RouteMessage sends a message from one agent to another.
func (t *Team) RouteMessage(from, to, content string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Verify sender is a team member
	if !t.isMember(from) {
		return fmt.Errorf("agent %q is not a member of team %q", from, t.Name)
	}

	// Verify recipient is a team member (or "all")
	if to != "all" && !t.isMember(to) {
		return fmt.Errorf("agent %q is not a member of team %q", to, t.Name)
	}

	msg := Message{
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now(),
	}
	t.Messages = append(t.Messages, msg)
	return nil
}

// Broadcast sends a message from one agent to all other team members.
func (t *Team) Broadcast(from, content string) error {
	return t.RouteMessage(from, "all", content)
}

// GetMember returns a worker by ID.
func (t *Team) GetMember(id string) (*Worker, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, m := range t.Members {
		if m.ID == id {
			return m, nil
		}
	}
	return nil, fmt.Errorf("agent %q not found in team %q", id, t.Name)
}

// MemberIDs returns the IDs of all team members.
func (t *Team) MemberIDs() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ids := make([]string, 0, len(t.Members))
	for _, m := range t.Members {
		ids = append(ids, m.ID)
	}
	return ids
}

// MemberStatuses returns a map of member ID to status.
func (t *Team) MemberStatuses() map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	statuses := make(map[string]string, len(t.Members))
	for _, m := range t.Members {
		statuses[m.ID] = m.Status.String()
	}
	return statuses
}

// RecentMessages returns the last n messages.
func (t *Team) RecentMessages(n int) []Message {
	t.mu.RLock()
	defer t.mu.RUnlock()

	total := len(t.Messages)
	if total == 0 {
		return nil
	}

	start := total - n
	if start < 0 {
		start = 0
	}

	result := make([]Message, n)
	copy(result, t.Messages[start:])
	return result
}

// MessageCount returns the total number of messages.
func (t *Team) MessageCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.Messages)
}

func (t *Team) isMember(id string) bool {
	for _, m := range t.Members {
		if m.ID == id {
			return true
		}
	}
	return false
}
