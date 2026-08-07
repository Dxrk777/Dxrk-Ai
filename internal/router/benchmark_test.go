// SPDX-License-Identifier: MIT
package router

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/query"
)

type benchmarkProvider struct {
	name     string
	delay    int
	tokensIn int
}

func (m *benchmarkProvider) Generate(ctx context.Context, msgs []query.Message, tools []query.ToolSchema) (query.Response, error) {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return query.Response{}, ctx.Err()
		default:
		}
	}
	return query.Response{
		Text: m.name + " response",
		Usage: query.Usage{
			InputTokens:  m.tokensIn,
			OutputTokens: m.tokensIn / 2,
		},
	}, nil
}

func BenchmarkRouter_FirstAvailable_2Providers(b *testing.B) {
	r := NewRouter([]ProviderEntry{
		{Name: "fast", Model: "gpt-4o-mini", Provider: &benchmarkProvider{name: "fast", tokensIn: 500}},
		{Name: "slow", Model: "gpt-4o", Provider: &benchmarkProvider{name: "slow", tokensIn: 5000}},
	})
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := r.Generate(ctx, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRouter_FirstAvailable_10Providers(b *testing.B) {
	entries := make([]ProviderEntry, 10)
	for i := range entries {
		entries[i] = ProviderEntry{
			Name:  fmt.Sprintf("p%d", i),
			Model: "gpt-4o-mini",
			Provider: &benchmarkProvider{
				name:     fmt.Sprintf("p%d", i),
				tokensIn: 100 * (i + 1),
			},
		}
	}
	r := NewRouter(entries)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Generate(ctx, nil, nil)
	}
}

func BenchmarkRouter_LowestCost_10Providers(b *testing.B) {
	entries := make([]ProviderEntry, 10)
	for i := range entries {
		var model string
		switch i % 4 {
		case 0:
			model = "gpt-4o-mini"
		case 1:
			model = "gpt-4o"
		case 2:
			model = "claude-3-haiku-20240307"
		case 3:
			model = "ollama/llama3.1:8b"
		}
		entries[i] = ProviderEntry{
			Name:  fmt.Sprintf("p%d", i),
			Model: model,
			Provider: &benchmarkProvider{
				name:     fmt.Sprintf("p%d", i),
				tokensIn: 100 * (i + 1),
			},
		}
	}
	r := NewRouter(entries, WithStrategy(StrategyLowestCost))
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Generate(ctx, nil, nil)
	}
}

func BenchmarkRouter_RoundRobin_10Providers(b *testing.B) {
	entries := make([]ProviderEntry, 10)
	for i := range entries {
		entries[i] = ProviderEntry{
			Name:  fmt.Sprintf("p%d", i),
			Model: "gpt-4o-mini",
			Provider: &benchmarkProvider{
				name:     fmt.Sprintf("p%d", i),
				tokensIn: 100,
			},
		}
	}
	r := NewRouter(entries, WithStrategy(StrategyRoundRobin))
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Generate(ctx, nil, nil)
	}
}

func BenchmarkRouter_Failover(b *testing.B) {
	entries := make([]ProviderEntry, 5)
	for i := range entries {
		fail := i < 3
		entries[i] = ProviderEntry{
			Name:  fmt.Sprintf("p%d", i),
			Model: "gpt-4o-mini",
			Provider: &benchmarkProvider{
				name:     fmt.Sprintf("p%d", i),
				tokensIn: 100,
			},
		}
		entries[i].Provider = &mockProvider{
			name: fmt.Sprintf("p%d", i),
			fail: fail,
			cost: 100,
		}
	}
	r := NewRouter(entries)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Generate(ctx, nil, nil)
	}
}

func BenchmarkCostTracker_Add(b *testing.B) {
	ct := NewCostTracker()
	models := make([]string, len(DefaultCosts))
	i := 0
	for m := range DefaultCosts {
		models[i] = m
		i++
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct.Add(models[rand.Intn(len(models))], 1000, 500) //nolint:gosec
	}
}

func BenchmarkCostTracker_AddConcurrent(b *testing.B) {
	ct := NewCostTracker()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ct.Add("gpt-4o-mini", 1000, 500)
		}
	})
}
