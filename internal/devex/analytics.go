// SPDX-License-Identifier: MIT
package devex

import (
	"context"
	"sync"
)

type Analytics struct {
	mu      sync.RWMutex
	storage map[string]int
}

func NewAnalytics(storage map[string]int) *Analytics {
	if storage == nil {
		storage = make(map[string]int)
	}
	return &Analytics{storage: storage}
}

func (a *Analytics) Increment(_ context.Context, feature string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.storage[feature]++
}

func (a *Analytics) Count(_ context.Context, feature string) int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.storage[feature]
}

func (a *Analytics) Snapshot() map[string]int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	snap := make(map[string]int, len(a.storage))
	for k, v := range a.storage {
		snap[k] = v
	}
	return snap
}

func (a *Analytics) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.storage = make(map[string]int)
}
