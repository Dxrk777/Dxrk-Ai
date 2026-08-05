// SPDX-License-Identifier: MIT
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ---- Settings Store Interface ----

// SettingsStore is a pluggable key-value persistence backend.
type SettingsStore interface {
	Get(key string) (any, bool)
	Set(key string, value any) error
	Delete(key string) error
	List() map[string]any
	Save() error
	Load() error
	Priority() int
}

// ---- File Settings Store ----

// FileSettingsStore persists settings to a JSON file in the user's home directory.
type FileSettingsStore struct {
	mu       sync.RWMutex
	path     string
	data     map[string]any
	priority int
}

// NewFileSettingsStore creates a store backed by ~/.dxrk/settings.json.
func NewFileSettingsStore() *FileSettingsStore {
	home, _ := os.UserHomeDir()
	return &FileSettingsStore{
		path:     filepath.Join(home, ".dxrk", "settings.json"),
		data:     make(map[string]any),
		priority: 100,
	}
}

func (s *FileSettingsStore) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *FileSettingsStore) Set(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

func (s *FileSettingsStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *FileSettingsStore) List() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

func (s *FileSettingsStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	return os.WriteFile(s.path, data, 0o600) //nolint:gosec
}

func (s *FileSettingsStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			s.data = make(map[string]any)
			return nil
		}
		return fmt.Errorf("read settings: %w", err)
	}

	return json.Unmarshal(data, &s.data)
}

func (s *FileSettingsStore) Priority() int { return s.priority }

// ---- Project Settings Store ----

// ProjectSettingsStore persists settings to .dxrk/settings.json in the project root.
type ProjectSettingsStore struct {
	mu       sync.RWMutex
	path     string
	data     map[string]any
	priority int
}

// NewProjectSettingsStore creates a store for the current project directory.
func NewProjectSettingsStore() *ProjectSettingsStore {
	return &ProjectSettingsStore{
		path:     ".dxrk/settings.json",
		data:     make(map[string]any),
		priority: 200,
	}
}

func (s *ProjectSettingsStore) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *ProjectSettingsStore) Set(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

func (s *ProjectSettingsStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *ProjectSettingsStore) List() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

func (s *ProjectSettingsStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	return os.WriteFile(s.path, data, 0o600) //nolint:gosec
}

func (s *ProjectSettingsStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			s.data = make(map[string]any)
			return nil
		}
		return fmt.Errorf("read settings: %w", err)
	}

	return json.Unmarshal(data, &s.data)
}

func (s *ProjectSettingsStore) Priority() int { return s.priority }

// ---- Memory Settings Store ----

// MemorySettingsStore is an in-memory store for testing.
type MemorySettingsStore struct {
	mu       sync.RWMutex
	data     map[string]any
	priority int
}

// NewMemorySettingsStore creates an in-memory settings store.
func NewMemorySettingsStore(priority int) *MemorySettingsStore {
	return &MemorySettingsStore{
		data:     make(map[string]any),
		priority: priority,
	}
}

func (s *MemorySettingsStore) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *MemorySettingsStore) Set(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

func (s *MemorySettingsStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *MemorySettingsStore) List() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

func (s *MemorySettingsStore) Save() error { return nil }

func (s *MemorySettingsStore) Load() error { return nil }

func (s *MemorySettingsStore) Priority() int { return s.priority }

// ---- Settings Manager ----

// SettingsManager provides multi-store settings with priority-based access.
type SettingsManager struct {
	mu     sync.RWMutex
	stores []SettingsStore
}

// NewSettingsManager creates a SettingsManager. Stores are searched in
// descending priority order (highest priority first).
func NewSettingsManager(stores ...SettingsStore) *SettingsManager {
	// Sort by priority descending
	sorted := make([]SettingsStore, len(stores))
	copy(sorted, stores)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority() > sorted[j].Priority()
	})

	return &SettingsManager{stores: sorted}
}

// NewDefaultSettingsManager creates a SettingsManager with file + project stores.
func NewDefaultSettingsManager() *SettingsManager {
	return NewSettingsManager(
		NewProjectSettingsStore(),
		NewFileSettingsStore(),
	)
}

// Get retrieves a setting by searching stores in priority order.
func (m *SettingsManager) Get(key string) (any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, store := range m.stores {
		if val, ok := store.Get(key); ok {
			return val, nil
		}
	}
	return nil, fmt.Errorf("setting not found: %s", key)
}

// Set writes a setting to the highest-priority store.
func (m *SettingsManager) Set(key string, value any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.stores) == 0 {
		return fmt.Errorf("no settings stores configured")
	}

	// Write to highest-priority store
	return m.stores[0].Set(key, value)
}

// Delete removes a setting from all stores.
func (m *SettingsManager) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, store := range m.stores {
		if err := store.Delete(key); err != nil {
			return fmt.Errorf("delete from store: %w", err)
		}
	}
	return nil
}

// List returns all settings merged from all stores (highest priority wins).
func (m *SettingsManager) List() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]any)
	// Iterate from lowest to highest priority so higher wins
	for i := len(m.stores) - 1; i >= 0; i-- {
		for k, v := range m.stores[i].List() {
			out[k] = v
		}
	}
	return out
}

// Export serializes all settings to JSON.
func (m *SettingsManager) Export() ([]byte, error) {
	all := m.List()
	return json.MarshalIndent(all, "", "  ")
}

// Import loads settings from JSON data into the highest-priority store.
func (m *SettingsManager) Import(data []byte) error {
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse import data: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.stores) == 0 {
		return fmt.Errorf("no settings stores configured")
	}

	for k, v := range settings {
		if err := m.stores[0].Set(k, v); err != nil {
			return fmt.Errorf("import setting %s: %w", k, err)
		}
	}
	return nil
}

// Save persists all stores to disk.
func (m *SettingsManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, store := range m.stores {
		if err := store.Save(); err != nil {
			return fmt.Errorf("save store (priority %d): %w", store.Priority(), err)
		}
	}
	return nil
}

// Load loads all stores from disk.
func (m *SettingsManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, store := range m.stores {
		if err := store.Load(); err != nil {
			return fmt.Errorf("load store (priority %d): %w", store.Priority(), err)
		}
	}
	return nil
}

// Keys returns all setting keys sorted alphabetically.
func (m *SettingsManager) Keys() []string {
	all := m.List()
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Has checks if a setting exists in any store.
func (m *SettingsManager) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, store := range m.stores {
		if _, ok := store.Get(key); ok {
			return true
		}
	}
	return false
}

// GetWithDefault retrieves a setting, returning a default if not found.
func (m *SettingsManager) GetWithDefault(key string, defaultVal any) any {
	val, err := m.Get(key)
	if err != nil {
		return defaultVal
	}
	return val
}

// KeysByPrefix returns all keys matching a prefix.
func (m *SettingsManager) KeysByPrefix(prefix string) []string {
	all := m.List()
	var keys []string
	for k := range all {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}
