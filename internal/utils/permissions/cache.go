// SPDX-License-Identifier: MIT
package permissions

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ---- Cache Entry ----

// CacheEntry is a single cached permission decision.
type CacheEntry struct {
	Key       string    `json:"key"`
	Action    Action    `json:"action"`
	RuleID    string    `json:"rule_id,omitempty"`
	Expiry    time.Time `json:"expiry"`
	Timestamp time.Time `json:"timestamp"`
}

// IsExpired reports whether the entry has expired.
func (ce *CacheEntry) IsExpired() bool {
	return !ce.Expiry.IsZero() && time.Now().After(ce.Expiry)
}

// ---- Permission Cache ----

// PermissionCache is a thread-safe LRU permission cache with TTL.
type PermissionCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	order   []string
	maxSize int
	ttl     time.Duration
}

// NewPermissionCache creates a cache with the given TTL and max entries.
// Use ttl=0 for session-only entries (no expiry). MaxSize of 0 defaults to 1024.
func NewPermissionCache(ttl time.Duration, maxSize int) *PermissionCache {
	if maxSize <= 0 {
		maxSize = 1024
	}
	return &PermissionCache{
		entries: make(map[string]*CacheEntry, maxSize),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a cache entry by key. Returns nil, false if missing or expired.
func (pc *PermissionCache) Get(key string) (*CacheEntry, bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	entry, ok := pc.entries[key]
	if !ok {
		return nil, false
	}
	if entry.IsExpired() {
		pc.removeLocked(key)
		return nil, false
	}
	// Move to end (most recently used).
	pc.touchLocked(key)
	return entry, true
}

// Set stores a cache entry. If the cache is full, the least recently used
// entry is evicted.
func (pc *PermissionCache) Set(key string, entry CacheEntry) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if existing, ok := pc.entries[key]; ok {
		pc.touchLocked(key)
		entry.Timestamp = time.Now()
		if entry.Expiry.IsZero() && !existing.Expiry.IsZero() {
			entry.Expiry = existing.Expiry
		}
		pc.entries[key] = &entry
		return
	}

	for len(pc.order) >= pc.maxSize {
		pc.evictOldestLocked()
	}

	entry.Timestamp = time.Now()
	if entry.Expiry.IsZero() && pc.ttl > 0 {
		entry.Expiry = time.Now().Add(pc.ttl)
	}
	pc.entries[key] = &entry
	pc.order = append(pc.order, key)
}

// SetWithTTL stores a cache entry with a custom TTL, overriding the default.
func (pc *PermissionCache) SetWithTTL(key string, entry CacheEntry, ttl time.Duration) {
	entry.Expiry = time.Now().Add(ttl)
	pc.Set(key, entry)
}

// Invalidate removes a single cache entry.
func (pc *PermissionCache) Invalidate(key string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.removeLocked(key)
}

// InvalidateAll clears the entire cache.
func (pc *PermissionCache) InvalidateAll() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.entries = make(map[string]*CacheEntry, pc.maxSize)
	pc.order = pc.order[:0]
}

// Size returns the number of entries in the cache.
func (pc *PermissionCache) Size() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.entries)
}

// Purge removes all expired entries.
func (pc *PermissionCache) Purge() int {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	removed := 0
	var remaining []string
	for _, key := range pc.order {
		entry := pc.entries[key]
		if entry.IsExpired() {
			delete(pc.entries, key)
			removed++
		} else {
			remaining = append(remaining, key)
		}
	}
	pc.order = remaining
	return removed
}

// ---- Internal Helpers ----

func (pc *PermissionCache) touchLocked(key string) {
	for i, k := range pc.order {
		if k == key {
			pc.order = append(pc.order[:i], pc.order[i+1:]...)
			pc.order = append(pc.order, key)
			return
		}
	}
}

func (pc *PermissionCache) removeLocked(key string) {
	if _, ok := pc.entries[key]; !ok {
		return
	}
	delete(pc.entries, key)
	for i, k := range pc.order {
		if k == key {
			pc.order = append(pc.order[:i], pc.order[i+1:]...)
			return
		}
	}
}

func (pc *PermissionCache) evictOldestLocked() {
	if len(pc.order) == 0 {
		return
	}
	oldest := pc.order[0]
	delete(pc.entries, oldest)
	pc.order = pc.order[1:]
}

// ---- Cache Key Construction ----

// CacheKey builds a cache key from tool, resource, and optional context hash.
func CacheKey(tool, resource, extra string) string {
	h := sha256.New()
	h.Write([]byte(tool))
	h.Write([]byte{0})
	h.Write([]byte(resource))
	if extra != "" {
		h.Write([]byte{0})
		h.Write([]byte(extra))
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:16])
}

// ---- Disk Persistence ----

type cacheSnapshot struct {
	Entries []CacheEntry `json:"entries"`
}

// PersistToDisk writes the cache to a JSON file.
func (pc *PermissionCache) PersistToDisk(path string) error {
	pc.mu.RLock()
	snapshot := make([]CacheEntry, 0, len(pc.entries))
	for _, entry := range pc.entries {
		if !entry.IsExpired() {
			snapshot = append(snapshot, *entry)
		}
	}
	pc.mu.RUnlock()

	data, err := json.MarshalIndent(&cacheSnapshot{Entries: snapshot}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	return os.WriteFile(path, data, 0o644) //nolint:gosec
}

// LoadFromDisk loads a cache from a JSON file.
func (pc *PermissionCache) LoadFromDisk(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("read cache file: %w", err)
	}

	var snapshot cacheSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("unmarshal cache: %w", err)
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	for _, entry := range snapshot.Entries {
		if entry.IsExpired() {
			continue
		}
		if len(pc.order) >= pc.maxSize {
			pc.evictOldestLocked()
		}
		e := entry
		pc.entries[entry.Key] = &e
		pc.order = append(pc.order, entry.Key)
	}
	return nil
}
