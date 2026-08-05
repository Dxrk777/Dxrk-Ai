package image

import (
	"sync"
	"time"
)

type CacheEntry struct {
	Data        []byte
	Format      Format
	Width       int
	Height      int
	CreatedAt   time.Time
	AccessedAt  time.Time
	AccessCount int
}

type ImageCache struct {
	entries     map[string]*CacheEntry
	mu          sync.RWMutex
	maxSize     int
	maxAge      time.Duration
	currentSize int
}

func NewImageCache(maxSize int, maxAge time.Duration) *ImageCache {
	if maxSize <= 0 {
		maxSize = 100
	}
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	return &ImageCache{
		entries: make(map[string]*CacheEntry),
		maxSize: maxSize,
		maxAge:  maxAge,
	}
}

func (c *ImageCache) Get(key string) ([]byte, Format, int, int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, Unknown, 0, 0, false
	}

	if time.Since(entry.CreatedAt) > c.maxAge {
		return nil, Unknown, 0, 0, false
	}

	entry.AccessedAt = time.Now()
	entry.AccessCount++
	return entry.Data, entry.Format, entry.Width, entry.Height, true
}

func (c *ImageCache) Set(key string, data []byte, format Format, width, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	c.entries[key] = &CacheEntry{
		Data:        data,
		Format:      format,
		Width:       width,
		Height:      height,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 1,
	}
	c.currentSize += len(data)
}

func (c *ImageCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return false
	}
	c.currentSize -= len(entry.Data)
	delete(c.entries, key)
	return true
}

func (c *ImageCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
	c.currentSize = 0
}

func (c *ImageCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time
	for key, entry := range c.entries {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}
	if oldestKey != "" {
		c.currentSize -= len(c.entries[oldestKey].Data)
		delete(c.entries, oldestKey)
	}
}

func (c *ImageCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		EntryCount: len(c.entries),
		TotalSize:  c.currentSize,
		MaxSize:    c.maxSize,
		MaxAge:     c.maxAge,
	}
}

type CacheStats struct {
	EntryCount int
	TotalSize  int
	MaxSize    int
	MaxAge     time.Duration
}

func (c *ImageCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.entries))
	for k := range c.entries {
		keys = append(keys, k)
	}
	return keys
}

func (c *ImageCache) PruneExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	count := 0
	for key, entry := range c.entries {
		if now.Sub(entry.CreatedAt) > c.maxAge {
			c.currentSize -= len(entry.Data)
			delete(c.entries, key)
			count++
		}
	}
	return count
}
