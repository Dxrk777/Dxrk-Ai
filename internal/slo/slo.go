package slo

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ObjectiveType int

const (
	ObjectiveLatency ObjectiveType = iota
	ObjectiveAvailability
	ObjectiveAccuracy
	ObjectiveThroughput
)

type Objective struct {
	Name        string
	Type        ObjectiveType
	Target      float64
	Window      time.Duration
	Current     float64
	ErrorBudget float64
	BurnRate    float64
	UpdatedAt   time.Time
}

type WindowSnapshot struct {
	Timestamp     time.Time
	ObjectiveName string
	Value         float64
	ErrorBudget   float64
	WithinSLO     bool
}

type Tracker struct {
	mu         sync.RWMutex
	objectives map[string]*Objective
	history    []WindowSnapshot
}

func NewTracker() *Tracker {
	return &Tracker{
		objectives: make(map[string]*Objective),
		history:    make([]WindowSnapshot, 0),
	}
}

func (t *Tracker) RegisterObjective(obj Objective) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.objectives[obj.Name]; exists {
		return fmt.Errorf("objective %q already exists", obj.Name)
	}

	obj.ErrorBudget = CalculateErrorBudget(obj.Target, obj.Current)
	obj.BurnRate = 0
	obj.UpdatedAt = time.Now()

	entry := obj
	t.objectives[obj.Name] = &entry
	return nil
}

func (t *Tracker) UpdateObjective(name string, value float64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	obj, exists := t.objectives[name]
	if !exists {
		return fmt.Errorf("objective %q not found", name)
	}

	prev := obj.Current
	obj.Current = value
	obj.ErrorBudget = CalculateErrorBudget(obj.Target, obj.Current)
	obj.UpdatedAt = time.Now()

	if obj.Window > 0 {
		obj.BurnRate = CalculateBurnRate(obj.Current, prev, obj.Window)
	}

	return nil
}

func (t *Tracker) GetObjective(name string) (*Objective, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	obj, exists := t.objectives[name]
	if !exists {
		return nil, fmt.Errorf("objective %q not found", name)
	}

	cp := *obj
	return &cp, nil
}

func (t *Tracker) ListObjectives() []Objective {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]Objective, 0, len(t.objectives))
	for _, obj := range t.objectives {
		result = append(result, *obj)
	}
	return result
}

func (t *Tracker) DeleteObjective(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.objectives[name]; !exists {
		return fmt.Errorf("objective %q not found", name)
	}

	delete(t.objectives, name)
	return nil
}

func (t *Tracker) Snapshot(ctx context.Context) (WindowSnapshot, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.objectives) == 0 {
		return WindowSnapshot{}, fmt.Errorf("no objectives registered")
	}

	var first string
	for name := range t.objectives {
		first = name
		break
	}

	obj := t.objectives[first]
	within := WithinSLO(obj.Current, obj.Target)

	snap := WindowSnapshot{
		Timestamp:     time.Now(),
		ObjectiveName: obj.Name,
		Value:         obj.Current,
		ErrorBudget:   obj.ErrorBudget,
		WithinSLO:     within,
	}

	t.history = append(t.history, snap)

	select {
	case <-ctx.Done():
		return WindowSnapshot{}, ctx.Err()
	default:
		return snap, nil
	}
}

func (t *Tracker) History(name string, limit int) []WindowSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var filtered []WindowSnapshot
	for _, snap := range t.history {
		if snap.ObjectiveName == name {
			filtered = append(filtered, snap)
		}
	}

	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered
}

func (t *Tracker) IsWithinSLO(name string) (bool, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	obj, exists := t.objectives[name]
	if !exists {
		return false, fmt.Errorf("objective %q not found", name)
	}

	return WithinSLO(obj.Current, obj.Target), nil
}
