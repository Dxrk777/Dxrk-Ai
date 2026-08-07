// SPDX-License-Identifier: MIT
package router

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"

	"github.com/Dxrk777/Dxrk/internal/query"
)

type Capability int

const (
	CapToolCall Capability = iota
	CapVision
	CapStreaming
)

type Strategy int

const (
	StrategyFirstAvailable Strategy = iota
	StrategyLowestCost
	StrategyRoundRobin
)

type ProviderEntry struct {
	Name         string
	Model        string
	Provider     query.Provider
	Capabilities []Capability
}

type Router struct {
	providers   []ProviderEntry
	strategy    Strategy
	costTracker *CostTracker
	mu          sync.Mutex
	rrIndex     int
	logger      func(format string, args ...any)
}

type RouterOption func(*Router)

func WithStrategy(s Strategy) RouterOption {
	return func(r *Router) { r.strategy = s }
}

func WithCostTracker(ct *CostTracker) RouterOption {
	return func(r *Router) { r.costTracker = ct }
}

func WithLogger(logFn func(format string, args ...any)) RouterOption {
	return func(r *Router) { r.logger = logFn }
}

func NewRouter(providers []ProviderEntry, opts ...RouterOption) *Router {
	r := &Router{
		providers:   providers,
		strategy:    StrategyFirstAvailable,
		costTracker: NewCostTracker(),
		logger:      func(string, ...any) {},
	}
	if len(providers) > 0 {
		r.rrIndex = rand.IntN(len(providers)) //nolint:gosec
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Router) Generate(ctx context.Context, messages []query.Message, tools []query.ToolSchema) (query.Response, error) {
	selected := r.selectProviders(len(r.providers))

	var lastErr error
	for _, idx := range selected {
		entry := r.providers[idx]

		resp, err := entry.Provider.Generate(ctx, messages, tools)
		if err == nil {
			if r.costTracker != nil {
				r.costTracker.Add(entry.Model, resp.Usage.InputTokens, resp.Usage.OutputTokens)
			}
			r.logger("[router] %s/%s succeeded (%d in + %d out tokens)",
				entry.Name, entry.Model, resp.Usage.InputTokens, resp.Usage.OutputTokens)
			return resp, nil
		}

		r.logger("[router] %s/%s failed: %v", entry.Name, entry.Model, err)
		lastErr = err
	}

	return query.Response{}, fmt.Errorf("all providers failed: %w", lastErr)
}

func (r *Router) Providers() []ProviderEntry {
	return r.providers
}

func (r *Router) AddProvider(entry ProviderEntry) {
	r.mu.Lock()
	r.providers = append(r.providers, entry)
	r.mu.Unlock()
}

func (r *Router) selectProviders(_ int) []int {
	r.mu.Lock()
	defer r.mu.Unlock()

	total := len(r.providers)
	if total == 0 {
		return nil
	}

	switch r.strategy {
	case StrategyRoundRobin:
		idx := r.rrIndex % total
		r.rrIndex = (idx + 1) % total
		return []int{idx}

	case StrategyLowestCost:
		return r.sortByCostLocked()

	default:
		indices := make([]int, total)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}
}

func (r *Router) sortByCostLocked() []int {
	type scored struct {
		idx   int
		cost  float64
		entry ProviderEntry
	}

	scoredList := make([]scored, len(r.providers))
	for i, p := range r.providers {
		cost := 0.0
		if cfg, ok := DefaultCosts[p.Model]; ok {
			cost = cfg.InputPricePer1K
		}
		scoredList[i] = scored{idx: i, cost: cost, entry: p}
	}

	for i := 0; i < len(scoredList); i++ {
		for j := i + 1; j < len(scoredList); j++ {
			if scoredList[j].cost < scoredList[i].cost {
				scoredList[i], scoredList[j] = scoredList[j], scoredList[i]
			}
		}
	}

	indices := make([]int, len(scoredList))
	for i, s := range scoredList {
		indices[i] = s.idx
	}
	return indices
}

func (r *Router) CostTracker() *CostTracker {
	return r.costTracker
}
