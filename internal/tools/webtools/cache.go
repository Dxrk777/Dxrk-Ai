package webtools

import (
	"net/url"
	"strings"
	"sync"
	"time"
)

// CacheEntry holds a cached fetch result.
type CacheEntry struct {
	URL       string       `json:"url"`
	Response  *FetchResult `json:"response"`
	FetchedAt time.Time    `json:"fetched_at"`
	ExpiresAt time.Time    `json:"expires_at"`
}

// WebCache provides an in-memory cache for fetch results.
type WebCache struct {
	entries map[string]*CacheEntry
	ttl     time.Duration
	maxSize int
	mu      sync.RWMutex
}

// NewWebCache creates a cache with the given TTL and max entry count.
func NewWebCache(ttl time.Duration, maxSize int) *WebCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if maxSize <= 0 {
		maxSize = 256
	}
	return &WebCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Get retrieves a cached result by URL. Returns nil if missing or expired.
func (c *WebCache) Get(rawURL string) (*CacheEntry, bool) {
	key := cacheKey(rawURL)
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if c.IsExpired(entry) {
		return nil, false
	}
	return entry, true
}

// Set stores a fetch result in the cache, respecting Cache-Control headers.
func (c *WebCache) Set(rawURL string, response *FetchResult) {
	key := cacheKey(rawURL)
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	ttl := c.ttl
	if maxAge := parseCacheControlMaxAge(response.Headers); maxAge > 0 {
		ttl = time.Duration(maxAge) * time.Second
	}

	c.entries[key] = &CacheEntry{
		URL:       rawURL,
		Response:  response,
		FetchedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Invalidate removes a specific URL from the cache.
func (c *WebCache) Invalidate(rawURL string) {
	key := cacheKey(rawURL)
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Clear removes all entries from the cache.
func (c *WebCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
}

// IsExpired checks whether a cache entry has passed its expiry time.
func (c *WebCache) IsExpired(entry *CacheEntry) bool {
	if entry == nil {
		return true
	}
	return time.Now().After(entry.ExpiresAt)
}

// Len returns the current number of cached entries.
func (c *WebCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func cacheKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Fragment = ""
	return strings.ToLower(u.String())
}

func (c *WebCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range c.entries {
		if first || v.FetchedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.FetchedAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

func parseCacheControlMaxAge(headers map[string]string) int {
	cc, ok := headers["Cache-Control"]
	if !ok {
		cc = headers["cache-control"]
	}
	if cc == "" {
		return 0
	}
	parts := strings.Split(cc, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			val := strings.TrimPrefix(part, "max-age=")
			val = strings.TrimSpace(val)
			n := 0
			for _, ch := range val {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				} else {
					break
				}
			}
			return n
		}
	}
	return 0
}
