package teammemorysync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultSyncInterval = 5 * time.Minute
	defaultMaxItems     = 1000
	defaultConflict     = "newest"
)

type MemoryItem struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	TeamID    string    `json:"team_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Tags      []string  `json:"tags"`
	Synced    bool      `json:"synced"`
}

type SyncConfig struct {
	TeamID           string        `json:"team_id"`
	SyncInterval     time.Duration `json:"sync_interval"`
	MaxItems         int           `json:"max_items"`
	ConflictStrategy string        `json:"conflict_strategy"`
	StoragePath      string        `json:"storage_path"`
}

type syncState struct {
	lastSync     time.Time
	pendingCount int
	running      bool
}

type TeamMemoryService struct {
	config    SyncConfig
	items     map[string]*MemoryItem
	mu        sync.Mutex
	syncState syncState
}

type SyncResult struct {
	ItemsPushed int
	ItemsPulled int
	Conflicts   int
	Errors      []string
}

type ExportData struct {
	Version    string       `json:"version"`
	TeamID     string       `json:"team_id"`
	ExportedAt time.Time    `json:"exported_at"`
	Items      []MemoryItem `json:"items"`
}

func NewTeamMemoryService(config SyncConfig) *TeamMemoryService {
	if config.SyncInterval == 0 {
		config.SyncInterval = defaultSyncInterval
	}
	if config.MaxItems == 0 {
		config.MaxItems = defaultMaxItems
	}
	if config.ConflictStrategy == "" {
		config.ConflictStrategy = defaultConflict
	}
	if config.StoragePath == "" {
		config.StoragePath = filepath.Join(os.TempDir(), "teammemorysync")
	}
	return &TeamMemoryService{config: config, items: make(map[string]*MemoryItem)}
}

func (s *TeamMemoryService) AddMemory(item MemoryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.ID == "" {
		return fmt.Errorf("memory item ID is required")
	}
	if item.Key == "" {
		return fmt.Errorf("memory item key is required")
	}
	if item.TeamID == "" {
		item.TeamID = s.config.TeamID
	}
	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	item.Synced = false
	if _, exists := s.items[item.ID]; !exists && len(s.items) >= s.config.MaxItems {
		return fmt.Errorf("max items limit reached: %d", s.config.MaxItems)
	}
	cp := item
	s.items[item.ID] = &cp
	s.syncState.pendingCount++
	return nil
}

func (s *TeamMemoryService) GetMemory(id string) (*MemoryItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return nil, false
	}
	cp := *item
	return &cp, true
}

func (s *TeamMemoryService) SearchMemories(query string) []MemoryItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := strings.ToLower(query)
	results := make([]MemoryItem, 0)
	for _, item := range s.items {
		if strings.Contains(strings.ToLower(item.Key), q) ||
			strings.Contains(strings.ToLower(item.Content), q) ||
			tagsContain(item.Tags, q) {
			results = append(results, *item)
		}
	}
	return results
}

func tagsContain(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func (s *TeamMemoryService) Sync(ctx context.Context, localChanges []MemoryItem) (*SyncResult, error) {
	s.mu.Lock()
	if s.syncState.running {
		s.mu.Unlock()
		return nil, fmt.Errorf("sync already in progress")
	}
	s.syncState.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.syncState.running = false
		s.mu.Unlock()
	}()
	result := &SyncResult{}
	for _, change := range localChanges {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		existing, ok := s.items[change.ID]
		if !ok {
			if err := s.AddMemory(change); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("add %s: %v", change.ID, err))
				continue
			}
			result.ItemsPushed++
			continue
		}
		resolved, conflict := s.ResolveConflict(existing, &change)
		if conflict {
			result.Conflicts++
		}
		resolved.Synced = false
		resolved.UpdatedAt = time.Now()
		s.items[resolved.ID] = resolved
		result.ItemsPushed++
	}
	for _, remote := range s.simulateRemoteFetch() {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		existing, ok := s.items[remote.ID]
		if !ok {
			if err := s.AddMemory(remote); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("import remote %s: %v", remote.ID, err))
				continue
			}
			result.ItemsPulled++
			continue
		}
		resolved, conflict := s.ResolveConflict(existing, &remote)
		if conflict {
			result.Conflicts++
		}
		*existing = *resolved
		result.ItemsPulled++
	}
	s.mu.Lock()
	s.syncState.lastSync = time.Now()
	s.mu.Unlock()
	return result, nil
}

func (s *TeamMemoryService) simulateRemoteFetch() []MemoryItem {
	return nil
}

func (s *TeamMemoryService) ResolveConflict(local, remote *MemoryItem) (*MemoryItem, bool) {
	if local.UpdatedAt.Equal(remote.UpdatedAt) {
		return local, false
	}
	switch s.config.ConflictStrategy {
	case "newest":
		if remote.UpdatedAt.After(local.UpdatedAt) {
			merged := *remote
			merged.Tags = mergeTags(local.Tags, remote.Tags)
			return &merged, true
		}
		return local, false
	case "manual":
		merged := *local
		merged.Content = local.Content + "\n---\n" + remote.Content
		merged.Tags = mergeTags(local.Tags, remote.Tags)
		merged.UpdatedAt = time.Now()
		return &merged, true
	default:
		if remote.UpdatedAt.After(local.UpdatedAt) {
			return remote, true
		}
		return local, false
	}
}

func mergeTags(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	result := make([]string, 0, len(a)+len(b))
	for _, t := range a {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			result = append(result, t)
		}
	}
	for _, t := range b {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			result = append(result, t)
		}
	}
	return result
}

func (s *TeamMemoryService) GetPendingSync() []MemoryItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := make([]MemoryItem, 0)
	for _, item := range s.items {
		if !item.Synced {
			pending = append(pending, *item)
		}
	}
	return pending
}

func (s *TeamMemoryService) MarkSynced(ids []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		if item, ok := s.items[id]; ok {
			item.Synced = true
			s.syncState.pendingCount--
		}
	}
}

func (s *TeamMemoryService) ExportMemories() (*ExportData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]MemoryItem, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, *item)
	}
	return &ExportData{
		Version: "1.0", TeamID: s.config.TeamID,
		ExportedAt: time.Now(), Items: items,
	}, nil
}

func (s *TeamMemoryService) ImportMemories(data *ExportData) error {
	if data == nil {
		return fmt.Errorf("import data is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range data.Items {
		existing, ok := s.items[item.ID]
		if ok {
			if item.UpdatedAt.After(existing.UpdatedAt) {
				item.Synced = false
				s.items[item.ID] = &item
			}
		} else {
			if len(s.items) >= s.config.MaxItems {
				continue
			}
			item.Synced = false
			s.items[item.ID] = &item
		}
	}
	return nil
}

func (s *TeamMemoryService) SaveToLocal() error {
	s.mu.Lock()
	dir := s.config.StoragePath
	cfg := s.config
	snapshot := make(map[string]*MemoryItem, len(s.items))
	for k, v := range s.items {
		cp := *v
		snapshot[k] = &cp
	}
	s.mu.Unlock()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create storage dir: %w", err)
	}
	items := make([]MemoryItem, 0, len(snapshot))
	for _, item := range snapshot {
		items = append(items, *item)
	}
	data := ExportData{Version: "1.0", TeamID: cfg.TeamID, ExportedAt: time.Now(), Items: items}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memories: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memories.json"), raw, 0o644); err != nil {
		return fmt.Errorf("write memories: %w", err)
	}
	cfgRaw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), cfgRaw, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (s *TeamMemoryService) LoadFromLocal() error {
	s.mu.Lock()
	dir := s.config.StoragePath
	s.mu.Unlock()
	raw, err := os.ReadFile(filepath.Join(dir, "memories.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read memories: %w", err)
	}
	var data ExportData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("unmarshal memories: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range data.Items {
		existing, ok := s.items[item.ID]
		if ok {
			if item.UpdatedAt.After(existing.UpdatedAt) {
				s.items[item.ID] = &item
			}
		} else {
			s.items[item.ID] = &item
		}
	}
	cfgRaw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err == nil {
		var lc SyncConfig
		if json.Unmarshal(cfgRaw, &lc) == nil && lc.TeamID != "" {
			s.config.TeamID = lc.TeamID
		}
	}
	return nil
}
