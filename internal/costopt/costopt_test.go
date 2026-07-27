// SPDX-License-Identifier: MIT
package costopt

import (
	"context"
	"testing"

	"github.com/Dxrk777/Dxrk-Ai/internal/router"
)

func TestNewCostOptimizer(t *testing.T) {
	r := router.NewRouter(nil)
	c := router.NewSemanticCache()
	co := NewCostOptimizer(r, c, BudgetConfig{
		DailyLimitUSD:   10,
		MonthlyLimitUSD: 100,
		AlertThreshold:  0.8,
	}, "")
	if co == nil {
		t.Fatal("expected non-nil CostOptimizer")
	}
}

func TestRecordUsage(t *testing.T) {
	r := router.NewRouter(nil)
	co := NewCostOptimizer(r, router.NewSemanticCache(), BudgetConfig{}, "")
	co.RecordUsage(context.Background(), "gpt-4o", 1000, 500)

	st := co.GetBudgetStatus()
	if st.DailySpent <= 0 {
		t.Fatal("expected positive daily spend after recording usage")
	}
}

func TestCurrentSpend(t *testing.T) {
	r := router.NewRouter(nil)
	co := NewCostOptimizer(r, router.NewSemanticCache(), BudgetConfig{}, "")
	co.RecordUsage(context.Background(), "gpt-4o", 1000, 500)
	co.RecordUsage(context.Background(), "claude-sonnet-4", 2000, 1000)

	st := co.GetBudgetStatus()
	if st.DailySpent <= 0 {
		t.Fatal("expected positive daily spend")
	}
	if st.MonthlySpent != st.DailySpent {
		t.Fatal("monthly spend should equal daily spend when only one day")
	}
}

func TestRecordUsageUnknownModel(t *testing.T) {
	r := router.NewRouter(nil)
	co := NewCostOptimizer(r, router.NewSemanticCache(), BudgetConfig{}, "")
	co.RecordUsage(context.Background(), "unknown-model", 1000, 500)

	st := co.GetBudgetStatus()
	if st.DailySpent != 0 {
		t.Fatal("expected zero cost for unknown model")
	}
}

func TestBudgetExceeded(t *testing.T) {
	r := router.NewRouter(nil)
	co := NewCostOptimizer(r, router.NewSemanticCache(), BudgetConfig{
		DailyLimitUSD:   1,
		MonthlyLimitUSD: 1,
		AlertThreshold:  0.5,
	}, "")

	co.RecordUsage(context.Background(), "gpt-4o", 100000, 100000)
	co.RecordUsage(context.Background(), "gpt-4o", 100000, 100000)

	st := co.GetBudgetStatus()
	if len(st.Alerts) == 0 {
		t.Fatal("expected alerts when budget exceeds threshold")
	}
	if st.Alerts[0].Level != "warning" && st.Alerts[0].Level != "critical" {
		t.Fatalf("unexpected alert level: %s", st.Alerts[0].Level)
	}
}

func TestSelectBestProvider(t *testing.T) {
	providers := []router.ProviderEntry{
		{Name: "provider-a", Model: "gpt-4o", Capabilities: nil},
		{Name: "provider-b", Model: "claude-sonnet-4", Capabilities: nil},
	}
	r := router.NewRouter(providers)
	co := NewCostOptimizer(r, router.NewSemanticCache(), BudgetConfig{}, "")

	best, err := co.SelectBestProvider(context.Background(), nil)
	if err != nil {
		t.Fatalf("SelectBestProvider: %v", err)
	}
	if best == nil {
		t.Fatal("expected a provider to be selected")
	}
}

func TestSelectBestProviderWithCapabilities(t *testing.T) {
	providers := []router.ProviderEntry{
		{Name: "basic", Model: "gpt-4o-mini", Capabilities: nil},
		{Name: "vision", Model: "gpt-4o", Capabilities: []router.Capability{router.CapVision}},
	}
	r := router.NewRouter(providers)
	co := NewCostOptimizer(r, router.NewSemanticCache(), BudgetConfig{}, "")

	best, err := co.SelectBestProvider(context.Background(), []router.Capability{router.CapVision})
	if err != nil {
		t.Fatalf("SelectBestProvider with capabilities: %v", err)
	}
	if best.Name != "vision" {
		t.Fatalf("expected 'vision' provider, got %q", best.Name)
	}
}

func TestSelectBestProviderNoMatch(t *testing.T) {
	r := router.NewRouter(nil)
	co := NewCostOptimizer(r, router.NewSemanticCache(), BudgetConfig{}, "")

	_, err := co.SelectBestProvider(context.Background(), []router.Capability{router.CapVision})
	if err == nil {
		t.Fatal("expected error when no provider matches")
	}
}

func TestAlertThreshold(t *testing.T) {
	r := router.NewRouter(nil)
	co := NewCostOptimizer(r, router.NewSemanticCache(), BudgetConfig{
		DailyLimitUSD:   5,
		MonthlyLimitUSD: 5,
		AlertThreshold:  0.5,
	}, "")

	co.RecordUsage(context.Background(), "gpt-4o", 500000, 250000)

	st := co.GetBudgetStatus()
	if len(st.Alerts) == 0 {
		t.Fatal("expected alerts when spend exceeds 50% threshold")
	}
}

func TestGetProviderScores(t *testing.T) {
	providers := []router.ProviderEntry{
		{Name: "fast", Model: "gpt-4o-mini", Capabilities: nil},
		{Name: "powerful", Model: "claude-sonnet-4", Capabilities: nil},
	}
	r := router.NewRouter(providers)
	co := NewCostOptimizer(r, router.NewSemanticCache(), BudgetConfig{}, "")

	scores := co.GetProviderScores()
	if len(scores) != 2 {
		t.Fatalf("expected 2 provider scores, got %d", len(scores))
	}
	for _, s := range scores {
		if s.Name == "" {
			t.Fatal("expected non-empty provider name in score")
		}
		if s.Score <= 0 {
			t.Fatal("expected positive score")
		}
	}
}

func TestBudgetStatusZeroLimits(t *testing.T) {
	r := router.NewRouter(nil)
	co := NewCostOptimizer(r, router.NewSemanticCache(), BudgetConfig{}, "")

	st := co.GetBudgetStatus()
	if st.DailyPercent != 0 || st.MonthlyPercent != 0 {
		t.Fatal("expected zero percentages when no limits set")
	}
}

func TestCacheHitRate(t *testing.T) {
	r := router.NewRouter(nil)
	cache := router.NewSemanticCache()
	co := NewCostOptimizer(r, cache, BudgetConfig{}, "")

	rate := co.cacheHitRate()
	if rate != 0 {
		t.Fatal("expected zero cache hit rate with empty cache")
	}
}
