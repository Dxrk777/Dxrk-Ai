// SPDX-License-Identifier: MIT
package router

import (
	"container/heap"
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/query"
)

type CacheEntry struct {
	Response    queryResponse
	CreatedAt   time.Time
	ExpiresAt   time.Time
	AccessCount int
	lastAccess  time.Time
	index       int
}

type queryResponse struct {
	Text         string
	InputTokens  int
	OutputTokens int
}

type cacheHeap []*CacheEntry

func (h cacheHeap) Len() int           { return len(h) }
func (h cacheHeap) Less(i, j int) bool { return h[i].lastAccess.Before(h[j].lastAccess) }
func (h cacheHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *cacheHeap) Push(x any) {
	n := len(*h)
	entry := x.(*CacheEntry)
	entry.index = n
	*h = append(*h, entry)
}
func (h *cacheHeap) Pop() any {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil
	entry.index = -1
	*h = old[:n-1]
	return entry
}

type SemanticCache struct {
	mu                sync.RWMutex
	entries           map[string]*CacheEntry
	lru               cacheHeap
	maxSize           int
	ttl               time.Duration
	semanticEnabled   bool
	semanticThreshold float64
	embeddings        map[string][]float64
	keyFn             func(string) string
}

type CacheOption func(*SemanticCache)

func NewSemanticCache(opts ...CacheOption) *SemanticCache {
	c := &SemanticCache{
		entries:           make(map[string]*CacheEntry),
		maxSize:           1000,
		ttl:               5 * time.Minute,
		semanticEnabled:   false,
		semanticThreshold: 0.95,
		embeddings:        make(map[string][]float64),
		keyFn:             defaultKeyFn,
	}
	heap.Init(&c.lru)
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithMaxSize(size int) CacheOption {
	return func(c *SemanticCache) { c.maxSize = size }
}

func WithTTL(ttl time.Duration) CacheOption {
	return func(c *SemanticCache) { c.ttl = ttl }
}

func WithSemanticMatching(enabled bool, threshold float64) CacheOption {
	return func(c *SemanticCache) {
		c.semanticEnabled = enabled
		if threshold > 0 {
			c.semanticThreshold = threshold
		}
	}
}

func WithCustomKeyFn(fn func(string) string) CacheOption {
	return func(c *SemanticCache) { c.keyFn = fn }
}

func (c *SemanticCache) Get(messages string) (queryResponse, bool) { //nolint:revive
	c.mu.RLock()
	defer c.mu.RUnlock()

	hash := c.keyFn(messages)
	entry, ok := c.entries[hash]
	if !ok {
		if !c.semanticEnabled {
			return queryResponse{}, false
		}
		return c.semanticGet(messages)
	}

	if time.Now().After(entry.ExpiresAt) {
		return queryResponse{}, false
	}

	entry.AccessCount++
	entry.lastAccess = time.Now()
	return entry.Response, true
}

func (c *SemanticCache) Set(messages string, resp queryResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	hash := c.keyFn(messages)
	if existing, ok := c.entries[hash]; ok {
		existing.Response = resp
		existing.ExpiresAt = time.Now().Add(c.ttl)
		existing.lastAccess = time.Now()
		return
	}

	entry := &CacheEntry{
		Response:   resp,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(c.ttl),
		lastAccess: time.Now(),
	}

	if len(c.entries) >= c.maxSize {
		c.evictLocked()
	}

	c.entries[hash] = entry
	heap.Push(&c.lru, entry)

	if c.semanticEnabled {
		c.embeddings[hash] = simpleEmbed(messages)
	}
}

func (c *SemanticCache) Invalidate(messages string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	hash := c.keyFn(messages)
	if entry, ok := c.entries[hash]; ok {
		heap.Remove(&c.lru, entry.index)
		delete(c.entries, hash)
		delete(c.embeddings, hash)
	}
}

func (c *SemanticCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.lru = make(cacheHeap, 0)
	c.embeddings = make(map[string][]float64)
	heap.Init(&c.lru)
}

func (c *SemanticCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hits := 0
	for _, e := range c.entries {
		if e.AccessCount > 0 {
			hits++
		}
	}

	return CacheStats{
		Size:    len(c.entries),
		MaxSize: c.maxSize,
		Hits:    hits,
		TTL:     c.ttl,
	}
}

func (c *SemanticCache) evictLocked() {
	for len(c.lru) > 0 && len(c.entries) >= c.maxSize {
		entry := heap.Pop(&c.lru).(*CacheEntry)
		for hash, e := range c.entries {
			if e == entry {
				delete(c.entries, hash)
				delete(c.embeddings, hash)
				break
			}
		}
	}
}

func (c *SemanticCache) semanticGet(messages string) (queryResponse, bool) {
	queryEmb := simpleEmbed(messages)
	var bestScore float64
	var bestResp queryResponse

	for hash, entry := range c.entries {
		if time.Now().After(entry.ExpiresAt) {
			continue
		}
		emb, ok := c.embeddings[hash]
		if !ok {
			continue
		}
		score := cosineSim(queryEmb, emb)
		if score > bestScore {
			bestScore = score
			bestResp = entry.Response
		}
	}

	if bestScore >= c.semanticThreshold {
		return bestResp, true
	}
	return queryResponse{}, false
}

type CacheStats struct {
	Size    int           `json:"size"`
	MaxSize int           `json:"max_size"`
	Hits    int           `json:"hits"`
	TTL     time.Duration `json:"ttl"`
}

func defaultKeyFn(messages string) string {
	h := sha256.Sum256([]byte(messages))
	return fmt.Sprintf("%x", h[:16])
}

func simpleEmbed(text string) []float64 {
	words := strings.Fields(text)
	if len(words) == 0 {
		return make([]float64, 128)
	}

	vec := make([]float64, 128)
	for _, w := range words {
		h := sha256.Sum256([]byte(w))
		idx := int(h[0]) % 128
		val := float64(int(h[1])%100) / 50.0
		if h[2]%2 == 0 {
			val = -val
		}
		vec[idx] += val
	}

	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	if norm > 0 {
		inv := 1.0 / math.Sqrt(norm)
		for i := range vec {
			vec[i] *= inv
		}
	}
	return vec
}

func cosineSim(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

type CachingRouter struct {
	*Router
	cache *SemanticCache
}

func NewCachingRouter(r *Router, cache *SemanticCache) *CachingRouter {
	return &CachingRouter{Router: r, cache: cache}
}

func (cr *CachingRouter) CachedGenerate(ctx context.Context, messages []query.Message, tools []query.ToolSchema) (query.Response, error) {
	prompt := joinMessages(messages)

	if resp, ok := cr.cache.Get(prompt); ok {
		return query.Response{
			Text: resp.Text,
			Usage: query.Usage{
				InputTokens:  resp.InputTokens,
				OutputTokens: resp.OutputTokens,
			},
		}, nil
	}

	resp, err := cr.Generate(ctx, messages, tools)
	if err != nil {
		return resp, err
	}

	cr.cache.Set(prompt, queryResponse{
		Text:         resp.Text,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	})
	return resp, nil
}

func joinMessages(msgs []query.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(string(m.Role))
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}
