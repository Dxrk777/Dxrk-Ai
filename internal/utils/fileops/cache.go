package fileops

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CacheEntry is a single cached file.
type CacheEntry struct {
	Path    string
	Content string
	Size    int64
	ModTime time.Time
	Expiry  time.Time
}

// CacheStats reports cache performance.
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Entries   int
	HitRate   float64
	SizeBytes int64
}

// FileCache is a thread-safe LRU file content cache with TTL expiration.
type FileCache struct {
	mu         sync.RWMutex
	entries    map[string]*CacheEntry
	order      []string // LRU order: oldest first
	maxEntries int
	ttl        time.Duration
	hits       int64
	misses     int64
	evictions  int64
	watchers   map[string][]func(string)
	stopCh     chan struct{}
}

// NewFileCache creates a cache that holds at most maxEntries entries for the
// given TTL. A background goroutine polls watched files every 5 seconds.
func NewFileCache(maxEntries int, ttl time.Duration) *FileCache {
	if maxEntries < 1 {
		maxEntries = 256
	}
	c := &FileCache{
		entries:    make(map[string]*CacheEntry),
		maxEntries: maxEntries,
		ttl:        ttl,
		watchers:   make(map[string][]func(string)),
		stopCh:     make(chan struct{}),
	}
	go c.watchLoop()
	return c
}

// Get returns cached content for path if present and not expired.
func (c *FileCache) Get(path string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[path]
	if !ok {
		c.misses++
		return "", false
	}
	if time.Now().After(e.Expiry) {
		c.removeLocked(path)
		c.misses++
		return "", false
	}
	// Refresh LRU position
	c.touchLocked(path)
	c.hits++
	return e.Content, true
}

// Set caches content for path.
func (c *FileCache) Set(path string, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	info, _ := os.Stat(path)
	var modTime time.Time
	var size int64
	if info != nil {
		modTime = info.ModTime()
		size = info.Size()
	}

	e := &CacheEntry{
		Path:    path,
		Content: content,
		Size:    size,
		ModTime: modTime,
		Expiry:  time.Now().Add(c.ttl),
	}

	if _, exists := c.entries[path]; exists {
		c.entries[path] = e
		c.touchLocked(path)
		return
	}

	for len(c.entries) >= c.maxEntries {
		c.evictOldestLocked()
	}

	c.entries[path] = e
	c.order = append(c.order, path)
}

// Invalidate removes a single entry from the cache.
func (c *FileCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(path)
}

// InvalidateAll clears the entire cache.
func (c *FileCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
	c.order = c.order[:0]
}

// InvalidatePattern removes entries whose path matches the glob pattern.
func (c *FileCache) InvalidatePattern(pattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	matched := make([]string, 0)
	for path := range c.entries {
		if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
			matched = append(matched, path)
		}
	}
	for _, p := range matched {
		c.removeLocked(p)
	}
}

// Watch registers a callback that fires when path is detected as modified.
func (c *FileCache) Watch(path string, callback func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.watchers[path] = append(c.watchers[path], callback)
}

// Stats returns current cache statistics.
func (c *FileCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var totalSize int64
	for _, e := range c.entries {
		totalSize += e.Size
	}
	total := c.hits + c.misses
	var rate float64
	if total > 0 {
		rate = float64(c.hits) / float64(total)
	}
	return CacheStats{
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
		Entries:   len(c.entries),
		HitRate:   rate,
		SizeBytes: totalSize,
	}
}

// Close stops the background watcher goroutine.
func (c *FileCache) Close() {
	close(c.stopCh)
}

// --- internal helpers ---

func (c *FileCache) touchLocked(path string) {
	for i, p := range c.order {
		if p == path {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, path)
			return
		}
	}
}

func (c *FileCache) removeLocked(path string) {
	delete(c.entries, path)
	for i, p := range c.order {
		if p == path {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

func (c *FileCache) evictOldestLocked() {
	if len(c.order) == 0 {
		return
	}
	oldest := c.order[0]
	c.order = c.order[1:]
	delete(c.entries, oldest)
	c.evictions++
}

func (c *FileCache) watchLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.pollWatchers()
		}
	}
}

func (c *FileCache) pollWatchers() {
	c.mu.RLock()
	paths := make([]string, 0, len(c.watchers))
	for p := range c.watchers {
		paths = append(paths, p)
	}
	cbs := make(map[string][]func(string))
	for p, fns := range c.watchers {
		fnsCopy := make([]func(string), len(fns))
		copy(fnsCopy, fns)
		cbs[p] = fnsCopy
	}
	c.mu.RUnlock()

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		c.mu.RLock()
		e, ok := c.entries[p]
		c.mu.RUnlock()
		if ok && info.ModTime().After(e.ModTime) {
			c.mu.Lock()
			c.removeLocked(p)
			c.mu.Unlock()
			for _, cb := range cbs[p] {
				cb(p)
			}
		}
	}
}

// GetOrLoad returns cached content or loads it using the provided function,
// caches the result, and returns it.
func (c *FileCache) GetOrLoad(path string, loader func(string) (string, error)) (string, error) {
	if content, ok := c.Get(path); ok {
		return content, nil
	}
	content, err := loader(path)
	if err != nil {
		return "", err
	}
	c.Set(path, content)
	return content, nil
}

// Paths returns all currently cached file paths.
func (c *FileCache) Paths() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.order))
	out = append(out, c.order...)
	return out
}

// Size returns the number of entries in the cache.
func (c *FileCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Contains reports whether path is currently cached and not expired.
func (c *FileCache) Contains(path string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[path]
	if !ok {
		return false
	}
	return !time.Now().After(e.Expiry)
}

// Keys returns all cached keys (alias for Paths, for map-like usage).
func (c *FileCache) Keys() []string {
	return c.Paths()
}

// InvalidatePrefix removes all entries whose path starts with prefix.
func (c *FileCache) InvalidatePrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var matched []string
	for path := range c.entries {
		if strings.HasPrefix(path, prefix) {
			matched = append(matched, path)
		}
	}
	for _, p := range matched {
		c.removeLocked(p)
	}
}
