// SPDX-License-Identifier: MIT
package config

import (
	"fmt"
	"sync"
)

// ---- Feature Flags ----

// FeatureFlag represents a single feature flag.
type FeatureFlag struct {
	Name           string   `yaml:"name" json:"name"`
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	Description    string   `yaml:"description" json:"description"`
	RolloutPercent int      `yaml:"rollout_percent" json:"rollout_percent"`
	AllowedUsers   []string `yaml:"allowed_users,omitempty" json:"allowed_users,omitempty"`
}

// FeatureFlagManager manages feature flags with rollout control.
type FeatureFlagManager struct {
	mu     sync.RWMutex
	flags  map[string]*FeatureFlag
	config *ConfigManager
}

// NewFeatureFlagManager creates a flag manager backed by a ConfigManager.
func NewFeatureFlagManager(config *ConfigManager) *FeatureFlagManager {
	ffm := &FeatureFlagManager{
		flags:  make(map[string]*FeatureFlag),
		config: config,
	}
	ffm.LoadDefaults()
	return ffm
}

// LoadDefaults populates built-in feature flags.
func (m *FeatureFlagManager) LoadDefaults() {
	m.mu.Lock()
	defer m.mu.Unlock()

	defaults := []FeatureFlag{
		{
			Name:           "yolo_mode",
			Enabled:        false,
			Description:    "Skip confirmation prompts for all operations",
			RolloutPercent: 0,
		},
		{
			Name:           "auto_compact",
			Enabled:        true,
			Description:    "Automatically compact context when approaching limits",
			RolloutPercent: 100,
		},
		{
			Name:           "voice_input",
			Enabled:        false,
			Description:    "Enable voice input for commands",
			RolloutPercent: 0,
		},
		{
			Name:           "experimental_tools",
			Enabled:        false,
			Description:    "Enable experimental tool integrations",
			RolloutPercent: 10,
		},
		{
			Name:           "remote_sessions",
			Enabled:        false,
			Description:    "Enable remote session management",
			RolloutPercent: 25,
		},
	}

	for i := range defaults {
		m.flags[defaults[i].Name] = &defaults[i]
	}
}

// IsEnabled checks if a feature flag is enabled.
func (m *FeatureFlagManager) IsEnabled(flag string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	f, ok := m.flags[flag]
	if !ok {
		return false
	}
	return f.Enabled
}

// Enable activates a feature flag.
func (m *FeatureFlagManager) Enable(flag string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.flags[flag]
	if !ok {
		return fmt.Errorf("unknown feature flag: %s", flag)
	}
	f.Enabled = true
	f.RolloutPercent = 100
	return nil
}

// Disable deactivates a feature flag.
func (m *FeatureFlagManager) Disable(flag string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.flags[flag]
	if !ok {
		return fmt.Errorf("unknown feature flag: %s", flag)
	}
	f.Enabled = false
	f.RolloutPercent = 0
	return nil
}

// SetRollout sets the rollout percentage for a flag.
// The flag is enabled if percent > 0.
func (m *FeatureFlagManager) SetRollout(flag string, percent int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if percent < 0 || percent > 100 {
		return fmt.Errorf("rollout percent must be 0-100, got %d", percent)
	}

	f, ok := m.flags[flag]
	if !ok {
		return fmt.Errorf("unknown feature flag: %s", flag)
	}
	f.RolloutPercent = percent
	f.Enabled = percent > 0
	return nil
}

// GetAll returns all registered feature flags.
func (m *FeatureFlagManager) GetAll() []FeatureFlag {
	m.mu.RLock()
	defer m.mu.RUnlock()

	flags := make([]FeatureFlag, 0, len(m.flags))
	for _, f := range m.flags {
		flags = append(flags, *f)
	}
	return flags
}

// Get returns a single feature flag by name.
func (m *FeatureFlagManager) Get(name string) (*FeatureFlag, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	f, ok := m.flags[name]
	if !ok {
		return nil, false
	}
	cp := *f
	return &cp, true
}

// Register adds or updates a feature flag.
func (m *FeatureFlagManager) Register(f FeatureFlag) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flags[f.Name] = &f
}

// Remove deletes a feature flag.
func (m *FeatureFlagManager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.flags[name]; !ok {
		return fmt.Errorf("unknown feature flag: %s", name)
	}
	delete(m.flags, name)
	return nil
}

// IsEnabledForUser checks if a flag is enabled for a specific user
// considering rollout percentage and allowed users list.
func (m *FeatureFlagManager) IsEnabledForUser(flag, userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	f, ok := m.flags[flag]
	if !ok {
		return false
	}

	// If user is in allowed list, always enabled
	if len(f.AllowedUsers) > 0 {
		for _, u := range f.AllowedUsers {
			if u == userID {
				return true
			}
		}
	}

	// Check rollout percentage using deterministic hash
	if f.RolloutPercent <= 0 {
		return false
	}
	if f.RolloutPercent >= 100 {
		return f.Enabled
	}

	// Simple deterministic check: hash userID to 0-99
	hash := 0
	for _, c := range userID {
		hash = (hash*31 + int(c)) % 100
	}
	return f.Enabled && hash < f.RolloutPercent
}

// EnabledFlags returns the names of all enabled flags.
func (m *FeatureFlagManager) EnabledFlags() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var names []string
	for _, f := range m.flags {
		if f.Enabled {
			names = append(names, f.Name)
		}
	}
	return names
}
