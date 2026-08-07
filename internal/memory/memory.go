// SPDX-License-Identifier: MIT
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/rag"
	"github.com/Dxrk777/Dxrk/internal/vault"
)

type MemoryType int

const (
	MemorySemantic MemoryType = iota
	MemoryEpisodic
	MemoryProcedural
)

type MemoryEntry struct {
	ID          string            `json:"id"`
	Type        MemoryType        `json:"type"`
	Content     string            `json:"content"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Embedding   []float64         `json:"embedding,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	AccessedAt  time.Time         `json:"accessed_at"`
	AccessCount int               `json:"access_count"`
	Importance  float64           `json:"importance"`
	ProjectID   string            `json:"project_id"`
	SessionID   string            `json:"session_id"`
}

type AgentMemory struct {
	mu         sync.RWMutex
	entries    map[string]*MemoryEntry
	byProject  map[string][]string
	bySession  map[string][]string
	byType     map[MemoryType][]string
	rag        *rag.RAG
	vault      *vault.Vault
	path       string
	maxEntries int
}

func NewAgentMemory(path string, maxEntries int, r *rag.RAG, v *vault.Vault) *AgentMemory {
	m := &AgentMemory{
		entries:    make(map[string]*MemoryEntry),
		byProject:  make(map[string][]string),
		bySession:  make(map[string][]string),
		byType:     make(map[MemoryType][]string),
		rag:        r,
		vault:      v,
		path:       path,
		maxEntries: maxEntries,
	}
	_ = m.load()
	return m
}

func (m *AgentMemory) Store(ctx context.Context, entry MemoryEntry) error {
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("mem-%d", time.Now().UnixNano())
	}
	entry.CreatedAt = time.Now()
	entry.AccessedAt = time.Now()

	if m.rag != nil && m.rag.IsEnabled() && len(entry.Content) > 0 {
		results, _ := m.rag.Query(entry.Content, 1)
		if len(results) > 0 {
			entry.Embedding = results[0].Record.Embedding
		}
	}

	m.mu.Lock()
	if len(m.entries) >= m.maxEntries {
		m.evictLocked()
	}
	m.entries[entry.ID] = &entry
	m.byProject[entry.ProjectID] = append(m.byProject[entry.ProjectID], entry.ID)
	m.bySession[entry.SessionID] = append(m.bySession[entry.SessionID], entry.ID)
	m.byType[entry.Type] = append(m.byType[entry.Type], entry.ID)
	m.mu.Unlock()

	return m.save()
}

func (m *AgentMemory) Retrieve(ctx context.Context, id string) (*MemoryEntry, bool) {
	m.mu.RLock()
	entry, ok := m.entries[id]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}

	m.mu.Lock()
	entry.AccessedAt = time.Now()
	entry.AccessCount++
	m.mu.Unlock()

	return entry, true
}

func (m *AgentMemory) Search(ctx context.Context, projectID, query string, memType MemoryType, limit int) []*MemoryEntry {
	if m.rag == nil || !m.rag.IsEnabled() {
		return m.searchLocal(projectID, query, memType, limit)
	}

	results, err := m.rag.Query(query, limit)
	if err != nil || len(results) == 0 {
		return m.searchLocal(projectID, query, memType, limit)
	}

	var entries []*MemoryEntry
	for _, r := range results {
		if e, ok := m.entries[r.Record.ID]; ok {
			if projectID == "" || e.ProjectID == projectID {
				if memType == 0 || e.Type == memType {
					entries = append(entries, e)
				}
			}
		}
	}
	return entries
}

func (m *AgentMemory) searchLocal(projectID, query string, memType MemoryType, limit int) []*MemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var candidates []*MemoryEntry
	for _, e := range m.entries {
		if projectID != "" && e.ProjectID != projectID {
			continue
		}
		if memType != 0 && e.Type != memType {
			continue
		}
		if query == "" || containsIgnoreCase(e.Content, query) {
			candidates = append(candidates, e)
		}
	}

	candidates = topByImportance(candidates, limit)
	return candidates
}

func (m *AgentMemory) GetByProject(projectID string) []*MemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byProject[projectID]
	entries := make([]*MemoryEntry, 0, len(ids))
	for _, id := range ids {
		if e, ok := m.entries[id]; ok {
			entries = append(entries, e)
		}
	}
	return entries
}

func (m *AgentMemory) GetBySession(sessionID string) []*MemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.bySession[sessionID]
	entries := make([]*MemoryEntry, 0, len(ids))
	for _, id := range ids {
		if e, ok := m.entries[id]; ok {
			entries = append(entries, e)
		}
	}
	return entries
}

func (m *AgentMemory) GetByType(memType MemoryType) []*MemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byType[memType]
	entries := make([]*MemoryEntry, 0, len(ids))
	for _, id := range ids {
		if e, ok := m.entries[id]; ok {
			entries = append(entries, e)
		}
	}
	return entries
}

func (m *AgentMemory) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[id]
	if !ok {
		return nil
	}

	delete(m.entries, id)
	m.removeFromIndex(m.byProject[entry.ProjectID], id)
	m.removeFromIndex(m.bySession[entry.SessionID], id)
	m.removeFromIndex(m.byType[entry.Type], id)

	return m.save()
}

func (m *AgentMemory) evictLocked() {
	var oldest *MemoryEntry
	for _, e := range m.entries {
		if oldest == nil || e.AccessedAt.Before(oldest.AccessedAt) {
			oldest = e
		}
	}
	if oldest != nil {
		delete(m.entries, oldest.ID)
		m.removeFromIndex(m.byProject[oldest.ProjectID], oldest.ID)
		m.removeFromIndex(m.bySession[oldest.SessionID], oldest.ID)
		m.removeFromIndex(m.byType[oldest.Type], oldest.ID)
	}
}

func (m *AgentMemory) removeFromIndex(slice []string, id string) {
	for i, v := range slice {
		if v == id {
			copy(slice[i:], slice[i+1:])
			slice[len(slice)-1] = ""
			break
		}
	}
}

func (m *AgentMemory) save() error {
	if m.path == "" {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := make([]MemoryEntry, 0, len(m.entries))
	for _, e := range m.entries {
		data = append(data, *e)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(m.path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(m.path, jsonData, 0o600)
}

func (m *AgentMemory) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var entries []MemoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	for i := range entries {
		e := entries[i]
		m.entries[e.ID] = &e
		m.byProject[e.ProjectID] = append(m.byProject[e.ProjectID], e.ID)
		m.bySession[e.SessionID] = append(m.bySession[e.SessionID], e.ID)
		m.byType[e.Type] = append(m.byType[e.Type], e.ID)
	}
	return nil
}

func (m *AgentMemory) Stats() MemoryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	byType := make(map[MemoryType]int)
	for _, e := range m.entries {
		byType[e.Type]++
	}

	return MemoryStats{
		TotalEntries: len(m.entries),
		ByProject:    len(m.byProject),
		BySession:    len(m.bySession),
		ByType:       byType,
	}
}

type MemoryStats struct {
	TotalEntries int                `json:"total_entries"`
	ByProject    int                `json:"by_project"`
	BySession    int                `json:"by_session"`
	ByType       map[MemoryType]int `json:"by_type"`
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexIgnoreCase(s, substr) >= 0)
}

func indexIgnoreCase(s, substr string) int {
	ls, lsub := len(s), len(substr)
	if lsub == 0 {
		return 0
	}
	for i := 0; i <= ls-lsub; i++ {
		match := true
		for j := 0; j < lsub; j++ {
			if lower(s[i+j]) != lower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

func topByImportance(entries []*MemoryEntry, limit int) []*MemoryEntry {
	if len(entries) <= limit {
		return entries
	}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Importance > entries[i].Importance {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	return entries[:limit]
}
