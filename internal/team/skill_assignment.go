package team

import "fmt"

func (m *Manager) AssignSkill(memberID string, skill string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	member, exists := m.members[memberID]
	if !exists {
		return fmt.Errorf("member with ID %s not found", memberID)
	}

	for _, s := range member.Skills {
		if s == skill {
			return nil
		}
	}

	member.Skills = append(member.Skills, skill)
	return nil
}

func (m *Manager) RemoveSkill(memberID string, skill string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	member, exists := m.members[memberID]
	if !exists {
		return fmt.Errorf("member with ID %s not found", memberID)
	}

	for i, s := range member.Skills {
		if s == skill {
			member.Skills = append(member.Skills[:i], member.Skills[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("skill %s not found for member %s", skill, memberID)
}

func (m *Manager) GetMembersBySkill(skill string) []Member {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Member
	for _, member := range m.members {
		for _, s := range member.Skills {
			if s == skill {
				result = append(result, *member)
				break
			}
		}
	}
	return result
}

func (m *Manager) GetSkillsByMember(memberID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	member, exists := m.members[memberID]
	if !exists {
		return nil, fmt.Errorf("member with ID %s not found", memberID)
	}

	cp := make([]string, len(member.Skills))
	copy(cp, member.Skills)
	return cp, nil
}
