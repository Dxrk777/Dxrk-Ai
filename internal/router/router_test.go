// SPDX-License-Identifier: MIT
package router

import (
	"context"
	"errors"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/query"
)

type mockProvider struct {
	name string
	fail bool
	cost int
}

func (m *mockProvider) Generate(ctx context.Context, msgs []query.Message, tools []query.ToolSchema) (query.Response, error) {
	if m.fail {
		return query.Response{}, errors.New(m.name + " failed")
	}
	return query.Response{
		Text: m.name + " response",
		Usage: query.Usage{
			InputTokens:  m.cost,
			OutputTokens: m.cost,
		},
	}, nil
}

func TestRouter_FirstAvailable(t *testing.T) {
	r := NewRouter([]ProviderEntry{
		{Name: "fail", Model: "gpt-4o-mini", Provider: &mockProvider{name: "fail", fail: true}},
		{Name: "ok", Model: "gpt-4o", Provider: &mockProvider{name: "ok"}},
	})
	resp, err := r.Generate(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if resp.Text != "ok response" {
		t.Fatalf("expected 'ok response', got: %q", resp.Text)
	}
}

func TestRouter_AllFail(t *testing.T) {
	r := NewRouter([]ProviderEntry{
		{Name: "a", Model: "gpt-4o-mini", Provider: &mockProvider{name: "a", fail: true}},
		{Name: "b", Model: "gpt-4o", Provider: &mockProvider{name: "b", fail: true}},
	})
	_, err := r.Generate(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRouter_RoundRobin(t *testing.T) {
	r := NewRouter([]ProviderEntry{
		{Name: "a", Model: "gpt-4o-mini", Provider: &mockProvider{name: "a"}},
		{Name: "b", Model: "gpt-4o", Provider: &mockProvider{name: "b"}},
	}, WithStrategy(StrategyRoundRobin))

	resp1, _ := r.Generate(context.Background(), nil, nil)
	resp2, _ := r.Generate(context.Background(), nil, nil)
	if resp1.Text == resp2.Text {
		t.Fatal("expected different providers for round-robin")
	}
}

func TestRouter_LowestCost(t *testing.T) {
	r := NewRouter([]ProviderEntry{
		{Name: "cheap", Model: "gpt-4o-mini", Provider: &mockProvider{name: "cheap", cost: 10}},
		{Name: "expensive", Model: "gpt-4o", Provider: &mockProvider{name: "expensive", cost: 100}},
	}, WithStrategy(StrategyLowestCost))

	resp, err := r.Generate(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if resp.Text != "cheap response" {
		t.Fatalf("expected cheapest provider, got: %q", resp.Text)
	}
}

func TestRouter_AddProvider(t *testing.T) {
	r := NewRouter([]ProviderEntry{
		{Name: "first", Model: "gpt-4o-mini", Provider: &mockProvider{name: "first"}},
	})
	r.AddProvider(ProviderEntry{
		Name: "second", Model: "gpt-4o", Provider: &mockProvider{name: "second"},
	})
	if len(r.Providers()) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(r.Providers()))
	}
}

func TestRouter_EmptyProviders(t *testing.T) {
	r := NewRouter(nil)
	_, err := r.Generate(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error with no providers")
	}
}

func TestCostTracker(t *testing.T) {
	ct := NewCostTracker()
	ct.Add("gpt-4o-mini", 1000, 500)
	if ct.Total() == 0 {
		t.Fatal("expected non-zero cost")
	}
	ct.Reset()
	if ct.Total() != 0 {
		t.Fatal("expected zero cost after reset")
	}
}

func TestCostTracker_UnknownModel(t *testing.T) {
	ct := NewCostTracker()
	ct.Add("unknown-model", 1000, 500)
	if ct.Total() != 0 {
		t.Fatal("expected zero cost for unknown model")
	}
}
