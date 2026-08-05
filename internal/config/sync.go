// SPDX-License-Identifier: MIT
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ---- Sync Types ----

// SyncConfig holds settings synchronization configuration.
type SyncConfig struct {
	Endpoint string        `yaml:"endpoint" json:"endpoint"`
	APIKey   string        `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	DeviceID string        `yaml:"device_id" json:"device_id"`
	Interval time.Duration `yaml:"interval" json:"interval"`
	LastSync time.Time     `yaml:"last_sync" json:"last_sync"`
}

// SettingChange represents a single setting mutation for sync.
type SettingChange struct {
	Key       string    `json:"key"`
	Value     any       `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	DeviceID  string    `json:"device_id"`
	Operation string    `json:"operation"` // "set" or "delete"
}

// SyncStatus reports the current synchronization state.
type SyncStatus struct {
	Connected   bool      `json:"connected"`
	LastSync    time.Time `json:"last_sync"`
	PendingPush int       `json:"pending_push"`
	Error       string    `json:"error,omitempty"`
}

// ---- Conflict Resolution ----

// ConflictResolution determines how conflicting changes are resolved.
type ConflictResolution int

const (
	// ConflictLastWriteWins resolves by latest timestamp.
	ConflictLastWriteWins ConflictResolution = iota
	// ConflictLocalWins always prefers local changes.
	ConflictLocalWins
	// ConflictRemoteWins always prefers remote changes.
	ConflictRemoteWins
	// ConflictManual requires user intervention.
	ConflictManual
)

// ---- Settings Syncer ----

// SettingsSyncer provides bidirectional settings synchronization.
type SettingsSyncer struct {
	mu       sync.Mutex
	config   SyncConfig
	storage  SettingsStore
	client   *http.Client
	resolver ConflictResolution
	status   SyncStatus
	queue    []SettingChange
}

// NewSettingsSyncer creates a syncer with the given config and storage.
func NewSettingsSyncer(config SyncConfig, storage SettingsStore) *SettingsSyncer {
	if config.Interval == 0 {
		config.Interval = 5 * time.Minute
	}
	return &SettingsSyncer{
		config:   config,
		storage:  storage,
		client:   &http.Client{Timeout: 30 * time.Second},
		resolver: ConflictLastWriteWins,
	}
}

// SetResolver sets the conflict resolution strategy.
func (s *SettingsSyncer) SetResolver(r ConflictResolution) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resolver = r
}

// Sync performs a full bidirectional sync.
func (s *SettingsSyncer) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status.Error = ""

	// Collect local changes since last sync
	localChanges := s.collectLocalChanges()

	// Push local changes
	if len(localChanges) > 0 {
		if err := s.pushLocked(localChanges); err != nil {
			s.status.Error = fmt.Sprintf("push: %v", err)
			return fmt.Errorf("push changes: %w", err)
		}
	}

	// Pull remote changes
	remoteChanges, err := s.pullLocked()
	if err != nil {
		s.status.Error = fmt.Sprintf("pull: %v", err)
		return fmt.Errorf("pull changes: %w", err)
	}

	// Resolve conflicts and apply
	if len(remoteChanges) > 0 {
		merged := s.resolveConflictsLocked(localChanges, remoteChanges)
		if err := s.applyChanges(merged); err != nil {
			s.status.Error = fmt.Sprintf("apply: %v", err)
			return fmt.Errorf("apply merged changes: %w", err)
		}
	}

	s.status.LastSync = time.Now()
	s.config.LastSync = s.status.LastSync
	s.status.Connected = true
	return nil
}

// Push sends local changes to the remote endpoint.
func (s *SettingsSyncer) Push(localChanges []SettingChange) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pushLocked(localChanges)
}

func (s *SettingsSyncer) pushLocked(changes []SettingChange) error {
	if s.config.Endpoint == "" {
		return fmt.Errorf("no sync endpoint configured")
	}

	body, err := json.Marshal(changes)
	if err != nil {
		return fmt.Errorf("marshal changes: %w", err)
	}

	url := s.config.Endpoint + "/api/v1/sync/push"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-ID", s.config.DeviceID)
	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("push request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("push failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Pull fetches remote changes since the last sync.
func (s *SettingsSyncer) Pull(remoteChanges []SettingChange) ([]SettingChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pullLocked()
}

func (s *SettingsSyncer) pullLocked() ([]SettingChange, error) {
	if s.config.Endpoint == "" {
		return nil, nil
	}

	url := fmt.Sprintf("%s/api/v1/sync/pull?device_id=%s&since=%s",
		s.config.Endpoint,
		s.config.DeviceID,
		s.config.LastSync.Format(time.RFC3339),
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pull request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pull failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var changes []SettingChange
	if err := json.NewDecoder(resp.Body).Decode(&changes); err != nil {
		return nil, fmt.Errorf("decode changes: %w", err)
	}

	return changes, nil
}

// ResolveConflicts merges local and remote changes, resolving conflicts.
func (s *SettingsSyncer) ResolveConflicts(local, remote []SettingChange) []SettingChange {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resolveConflictsLocked(local, remote)
}

func (s *SettingsSyncer) resolveConflictsLocked(local, remote []SettingChange) []SettingChange {
	// Index local changes by key
	localMap := make(map[string]SettingChange)
	for _, c := range local {
		localMap[c.Key] = c
	}

	// Index remote changes by key
	remoteMap := make(map[string]SettingChange)
	for _, c := range remote {
		remoteMap[c.Key] = c
	}

	// Merge: remote wins on conflict unless strategy says otherwise
	merged := make(map[string]SettingChange)
	for key, rc := range remoteMap {
		lc, hasLocal := localMap[key]
		if hasLocal {
			switch s.resolver {
			case ConflictLocalWins:
				merged[key] = lc
			case ConflictRemoteWins:
				merged[key] = rc
			case ConflictLastWriteWins:
				if lc.Timestamp.After(rc.Timestamp) {
					merged[key] = lc
				} else {
					merged[key] = rc
				}
			case ConflictManual:
				// Keep both, remote first; caller must resolve
				merged[key] = rc
				merged["__conflict__"+key] = lc
			}
		} else {
			merged[key] = rc
		}
	}

	// Add local-only changes
	for key, lc := range localMap {
		if _, inRemote := remoteMap[key]; !inRemote {
			merged[key] = lc
		}
	}

	// Sort by timestamp for deterministic ordering
	result := make([]SettingChange, 0, len(merged))
	for _, v := range merged {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result
}

// GetSyncStatus returns the current sync status.
func (s *SettingsSyncer) GetSyncStatus() SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.PendingPush = len(s.queue)
	return s.status
}

// collectLocalChanges reads settings modified since last sync.
func (s *SettingsSyncer) collectLocalChanges() []SettingChange {
	all := s.storage.List()
	changes := make([]SettingChange, 0, len(all))

	for key, val := range all {
		changes = append(changes, SettingChange{
			Key:       key,
			Value:     val,
			Timestamp: time.Now(),
			DeviceID:  s.config.DeviceID,
			Operation: "set",
		})
	}

	return changes
}

// applyChanges applies merged changes to the local store.
func (s *SettingsSyncer) applyChanges(changes []SettingChange) error {
	for _, c := range changes {
		// Skip conflict markers (handled separately in manual mode)
		if len(c.Key) > 12 && c.Key[:12] == "__conflict__" {
			continue
		}

		switch c.Operation {
		case "delete":
			if err := s.storage.Delete(c.Key); err != nil {
				return fmt.Errorf("delete %s: %w", c.Key, err)
			}
		default:
			if err := s.storage.Set(c.Key, c.Value); err != nil {
				return fmt.Errorf("set %s: %w", c.Key, err)
			}
		}
	}
	return nil
}

// QueueChange adds a change to the sync queue for later push.
func (s *SettingsSyncer) QueueChange(key string, value any, operation string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.queue = append(s.queue, SettingChange{
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
		DeviceID:  s.config.DeviceID,
		Operation: operation,
	})
}

// FlushQueue pushes all queued changes.
func (s *SettingsSyncer) FlushQueue() error {
	s.mu.Lock()
	queue := s.queue
	s.queue = nil
	s.mu.Unlock()

	if len(queue) == 0 {
		return nil
	}
	return s.Push(queue)
}
