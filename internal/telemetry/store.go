// SPDX-License-Identifier: MIT
// Package telemetry provides opt-in local usage tracking for tools.
// Data is stored in ~/.config/dxrk/telemetry/ and never sent anywhere.
package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config holds telemetry settings.
type Config struct {
	Enabled bool   `json:"enabled"`
	Dir     string `json:"dir"`
}

// DefaultConfig returns telemetry config in ~/.config/dxrk/telemetry/.
func DefaultConfig(homeDir string) Config {
	return Config{
		Enabled: false,
		Dir:     filepath.Join(homeDir, ".config", "dxrk", "telemetry"),
	}
}

// Event is a single telemetry event.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Tool      string    `json:"tool,omitempty"`
	Duration  string    `json:"duration,omitempty"`
	Success   bool      `json:"success"`
}

// Store records telemetry events to a local JSON file.
type Store struct {
	mu     sync.Mutex
	config Config
	events []Event
}

// NewStore creates a telemetry store. If config.Enabled is false, all methods are no-ops.
func NewStore(cfg Config) *Store {
	return &Store{config: cfg}
}

// Record adds a telemetry event.
func (s *Store) Record(action, tool string, success bool, duration time.Duration) {
	if !s.config.Enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, Event{
		Timestamp: time.Now(),
		Action:    action,
		Tool:      tool,
		Duration:  duration.Round(time.Millisecond).String(),
		Success:   success,
	})
}

// Flush writes all buffered events to disk.
func (s *Store) Flush() error {
	if !s.config.Enabled {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.events) == 0 {
		return nil
	}

	if err := os.MkdirAll(s.config.Dir, 0o750); err != nil {
		return fmt.Errorf("create telemetry dir: %w", err)
	}

	timestamp := time.Now().UnixMilli()
	path := filepath.Join(s.config.Dir, fmt.Sprintf("events_%d.json", timestamp))

	data, err := json.Marshal(s.events)
	if err != nil {
		return fmt.Errorf("marshal events: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write telemetry: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename telemetry: %w", err)
	}

	s.events = s.events[:0]
	return nil
}

// Enable turns on telemetry and persists the choice.
func (s *Store) Enable() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.Enabled = true
	return s.writeConfig()
}

// Disable turns off telemetry and persists the choice.
func (s *Store) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.Enabled = false
	_ = s.writeConfig()
}

// IsEnabled returns whether telemetry is active.
func (s *Store) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.Enabled
}

func (s *Store) writeConfig() error {
	cfgPath := filepath.Join(s.config.Dir, "config.json")
	if err := os.MkdirAll(s.config.Dir, 0o750); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s.config, "", "  ")
	return os.WriteFile(cfgPath, data, 0o600)
}

// ToolCallCounter wraps a tools.Registry to record telemetry automatically.
type ToolCallCounter struct {
	store *Store
}

// NewToolCallCounter creates a counter that records tool calls.
func NewToolCallCounter(store *Store) *ToolCallCounter {
	return &ToolCallCounter{store: store}
}

// RecordCall records a tool execution event.
func (c *ToolCallCounter) RecordCall(toolName string, success bool, duration time.Duration) {
	c.store.Record("tool_call", toolName, success, duration)
}

// Flush writes buffered telemetry to disk.
func (c *ToolCallCounter) Flush() error {
	return c.store.Flush()
}
