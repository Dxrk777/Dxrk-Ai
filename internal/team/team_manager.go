package team

import (
	"fmt"
	"sync"
	"time"
)

type Manager struct {
	mu      sync.RWMutex
	members map[string]*Member
}

func NewManager() *Manager {
	return &Manager{
		members: make(map[string]*Member),
	}
}

func (m *Manager) AddMember(member Member) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.members[member.ID]; exists {
		return fmt.Errorf("member with ID %s already exists", member.ID)
	}

	cp := member
	m.members[member.ID] = &cp
	return nil
}

func (m *Manager) RemoveMember(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.members[id]; !exists {
		return fmt.Errorf("member with ID %s not found", id)
	}

	delete(m.members, id)
	return nil
}

func (m *Manager) GetMember(id string) (*Member, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	member, exists := m.members[id]
	if !exists {
		return nil, fmt.Errorf("member with ID %s not found", id)
	}

	cp := *member
	return &cp, nil
}

func (m *Manager) UpdateMember(member Member) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.members[member.ID]; !exists {
		return fmt.Errorf("member with ID %s not found", member.ID)
	}

	cp := member
	m.members[member.ID] = &cp
	return nil
}

func (m *Manager) ListMembers() []Member {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Member, 0, len(m.members))
	for _, member := range m.members {
		result = append(result, *member)
	}
	return result
}

func (m *Manager) ListMembersByRole(role Role) []Member {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Member
	for _, member := range m.members {
		if member.Role == role {
			result = append(result, *member)
		}
	}
	return result
}

func (m *Manager) SetRole(id string, role Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	member, exists := m.members[id]
	if !exists {
		return fmt.Errorf("member with ID %s not found", id)
	}

	member.Role = role
	return nil
}

func (m *Manager) HasPermission(id string, permission Permission) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	member, exists := m.members[id]
	if !exists {
		return false
	}

	for _, p := range member.Role.Permissions() {
		if p == permission {
			return true
		}
	}
	return false
}

func (m *Manager) GetActiveMembers() []Member {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Member
	for _, member := range m.members {
		if member.Active {
			result = append(result, *member)
		}
	}
	return result
}

func (m *Manager) RecordActivity(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if member, exists := m.members[id]; exists {
		member.LastActiveAt = time.Now()
	}
}
