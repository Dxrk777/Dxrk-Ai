// SPDX-License-Identifier: MIT
package mcp

import (
	"sort"
	"sync"
	"sync/atomic"
)

// MetricsExporter exposes rate limiter metrics.
type MetricsExporter interface {
	IncRateLimitedCalls(toolName string)
	SetTokensRemaining(count float64)
	Snapshot() map[string]any
}

// PrometheusMetrics implements MetricsExporter using atomic counters.
type PrometheusMetrics struct {
	rateLimitedCalls map[string]*atomic.Int64
	tokensRemaining  atomic.Int64
	mu               sync.RWMutex
}

// NewPrometheusMetrics creates a new PrometheusMetrics instance.
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		rateLimitedCalls: make(map[string]*atomic.Int64),
	}
}

// IncRateLimitedCalls increments the counter for a specific tool.
func (pm *PrometheusMetrics) IncRateLimitedCalls(toolName string) {
	pm.mu.RLock()
	counter, ok := pm.rateLimitedCalls[toolName]
	pm.mu.RUnlock()
	if ok {
		counter.Add(1)
		return
	}

	pm.mu.Lock()
	counter, ok = pm.rateLimitedCalls[toolName]
	if ok {
		pm.mu.Unlock()
		counter.Add(1)
		return
	}
	counter = new(atomic.Int64)
	counter.Add(1)
	pm.rateLimitedCalls[toolName] = counter
	pm.mu.Unlock()
}

// SetTokensRemaining stores the current token count.
func (pm *PrometheusMetrics) SetTokensRemaining(count float64) {
	pm.tokensRemaining.Store(int64(count))
}

// Snapshot returns all metrics as a map for serialization.
func (pm *PrometheusMetrics) Snapshot() map[string]any {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	snap := make(map[string]any)
	snap["rate_limiter_tokens_remaining"] = pm.tokensRemaining.Load()

	totalRateLimited := int64(0)
	rateLimited := make(map[string]int64)
	names := make([]string, 0, len(pm.rateLimitedCalls))
	for name := range pm.rateLimitedCalls {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		val := pm.rateLimitedCalls[name].Load()
		rateLimited[name] = val
		totalRateLimited += val
	}
	snap["rate_limited_total"] = totalRateLimited
	snap["rate_limited_by_tool"] = rateLimited

	return snap
}
