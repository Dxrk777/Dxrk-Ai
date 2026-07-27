// SPDX-License-Identifier: MIT
package dr

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Severity int

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

type Incident struct {
	ID          string
	Title       string
	Description string
	Severity    Severity
	DetectedAt  time.Time
	ResolvedAt  *time.Time
	ResolvedBy  string
	Notes       []string
	Affected    []string
}

type RecoveryPlan struct {
	ID          string
	Name        string
	Description string
	Steps       []RecoveryStep
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type RecoveryStep struct {
	Order       int
	Name        string
	Description string
	Command     string
	Timeout     time.Duration
	Critical    bool
}

type RecoveryResult struct {
	Step     RecoveryStep
	Success  bool
	Error    string
	Output   string
	Duration time.Duration
}

type Manager struct {
	mu        sync.Mutex
	incidents []Incident
	plans     []RecoveryPlan
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) RecordIncident(incident Incident) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incidents = append(m.incidents, incident)
}

func (m *Manager) ResolveIncident(id string, resolvedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.incidents {
		if m.incidents[i].ID == id && m.incidents[i].ResolvedAt == nil {
			now := time.Now()
			m.incidents[i].ResolvedAt = &now
			m.incidents[i].ResolvedBy = resolvedBy
			return nil
		}
	}
	return fmt.Errorf("incident %s not found or already resolved", id)
}

func (m *Manager) ListIncidents() []Incident {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Incident, len(m.incidents))
	copy(result, m.incidents)
	return result
}

func (m *Manager) ListOpenIncidents() []Incident {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Incident
	for _, inc := range m.incidents {
		if inc.ResolvedAt == nil {
			result = append(result, inc)
		}
	}
	return result
}

func (m *Manager) AddPlan(plan RecoveryPlan) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plans = append(m.plans, plan)
}

func (m *Manager) ExecutePlan(ctx context.Context, planID string, opts ...ExecuteOption) ([]RecoveryResult, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	m.mu.Lock()
	var plan *RecoveryPlan
	for i := range m.plans {
		if m.plans[i].ID == planID {
			plan = &m.plans[i]
			break
		}
	}
	m.mu.Unlock()

	if plan == nil {
		return nil, nil
	}

	results := make([]RecoveryResult, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		result := executeStep(ctx, step, cfg)
		results = append(results, result)
		if !result.Success && step.Critical {
			break
		}
	}
	return results, nil
}

func (m *Manager) CreateBackupPlan(name string, steps []RecoveryStep) RecoveryPlan {
	return RecoveryPlan{
		ID:        name,
		Name:      name,
		Steps:     steps,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
