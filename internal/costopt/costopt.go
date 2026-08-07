// SPDX-License-Identifier: MIT
package costopt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Dxrk777/Dxrk/internal/router"
	"github.com/Dxrk777/Dxrk/internal/strconst"
)

type BudgetConfig struct {
	DailyLimitUSD   float64  `json:"daily_limit_usd"`
	MonthlyLimitUSD float64  `json:"monthly_limit_usd"`
	AlertThreshold  float64  `json:"alert_threshold"`
	AutoSwitch      bool     `json:"auto_switch"`
	PreferredModels []string `json:"preferred_models"`
}

type CostOptimizer struct {
	mu           sync.RWMutex
	router       *router.Router
	cache        *router.SemanticCache
	budget       BudgetConfig
	dailySpent   float64
	monthlySpent float64
	lastReset    time.Time
	alerts       []Alert
	path         string
}

type Alert struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Current   float64   `json:"current"`
	Limit     float64   `json:"limit"`
}

type ProviderScore struct {
	Name            string  `json:"name"`
	Model           string  `json:"model"`
	CostPer1kTokens float64 `json:"cost_per_1k_tokens"`
	LatencyMs       int64   `json:"latency_ms"`
	SuccessRate     float64 `json:"success_rate"`
	Score           float64 `json:"score"`
}

func NewCostOptimizer(r *router.Router, c *router.SemanticCache, budget BudgetConfig, path string) *CostOptimizer {
	co := &CostOptimizer{
		router:    r,
		cache:     c,
		budget:    budget,
		path:      path,
		lastReset: time.Now().Truncate(24 * time.Hour),
	}
	co.load()
	return co
}

func (co *CostOptimizer) RecordUsage(ctx context.Context, model string, inputTokens, outputTokens int) {
	co.mu.Lock()
	defer co.mu.Unlock()

	cost := co.calculateCost(model, inputTokens, outputTokens)
	co.dailySpent += cost
	co.monthlySpent += cost

	co.checkBudget()
	co.save()
}

func (co *CostOptimizer) calculateCost(model string, in, out int) float64 {
	if cfg, ok := router.DefaultCosts[model]; ok {
		return (float64(in)/1000)*cfg.InputPricePer1K + (float64(out)/1000)*cfg.OutputPricePer1K
	}
	return 0
}

func (co *CostOptimizer) checkBudget() {
	if co.budget.DailyLimitUSD > 0 && co.dailySpent >= co.budget.DailyLimitUSD*co.budget.AlertThreshold {
		co.alerts = append(co.alerts, Alert{
			Timestamp: time.Now(), Level: "warning",
			Message: fmt.Sprintf("Daily budget at %.0f%%", co.dailySpent/co.budget.DailyLimitUSD*100),
			Current: co.dailySpent, Limit: co.budget.DailyLimitUSD,
		})
	}
	if co.budget.MonthlyLimitUSD > 0 && co.monthlySpent >= co.budget.MonthlyLimitUSD*co.budget.AlertThreshold {
		co.alerts = append(co.alerts, Alert{
			Timestamp: time.Now(), Level: strconst.StrCritical,
			Message: fmt.Sprintf("Monthly budget at %.0f%%", co.monthlySpent/co.budget.MonthlyLimitUSD*100),
			Current: co.monthlySpent, Limit: co.budget.MonthlyLimitUSD,
		})
	}
	if len(co.alerts) > 100 {
		co.alerts = co.alerts[len(co.alerts)-100:]
	}
}

func (co *CostOptimizer) SelectBestProvider(ctx context.Context, requiredCaps []router.Capability) (*router.ProviderEntry, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()

	var best *router.ProviderEntry
	var bestScore float64

	for _, p := range co.router.Providers() {
		if !co.hasCapabilities(p, requiredCaps) {
			continue
		}
		score := co.scoreProvider(p)
		if best == nil || score > bestScore {
			best = &p
			bestScore = score
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no provider matches capabilities")
	}
	return best, nil
}

func (co *CostOptimizer) hasCapabilities(p router.ProviderEntry, req []router.Capability) bool {
	if len(req) == 0 {
		return true
	}
	has := make(map[router.Capability]bool)
	for _, c := range p.Capabilities {
		has[c] = true
	}
	for _, r := range req {
		if !has[r] {
			return false
		}
	}
	return true
}

func (co *CostOptimizer) scoreProvider(p router.ProviderEntry) float64 {
	cfg, ok := router.DefaultCosts[p.Model]
	if !ok {
		return 0
	}
	cost := cfg.InputPricePer1K
	latency := 100.0
	success := 1.0

	if co.cache != nil {
		st := co.cache.Stats()
		if st.Size > 0 {
			success *= 0.9 + 0.1*float64(st.Hits)/float64(st.Size+1)
		}
	}

	return (1.0/(cost+0.0001))*0.5 + (1.0/(latency+1))*0.3 + success*0.2
}

func (co *CostOptimizer) GetBudgetStatus() BudgetStatus {
	co.mu.RLock()
	defer co.mu.RUnlock()

	dailyPct := 0.0
	if co.budget.DailyLimitUSD > 0 {
		dailyPct = co.dailySpent / co.budget.DailyLimitUSD * 100
	}
	monthlyPct := 0.0
	if co.budget.MonthlyLimitUSD > 0 {
		monthlyPct = co.monthlySpent / co.budget.MonthlyLimitUSD * 100
	}

	return BudgetStatus{
		DailySpent:     co.dailySpent,
		DailyLimit:     co.budget.DailyLimitUSD,
		DailyPercent:   dailyPct,
		MonthlySpent:   co.monthlySpent,
		MonthlyLimit:   co.budget.MonthlyLimitUSD,
		MonthlyPercent: monthlyPct,
		Alerts:         co.alerts,
		CacheHitRate:   co.cacheHitRate(),
	}
}

func (co *CostOptimizer) cacheHitRate() float64 {
	if co.cache == nil {
		return 0
	}
	st := co.cache.Stats()
	if st.Size == 0 {
		return 0
	}
	return float64(st.Hits) / float64(st.Size)
}

func (co *CostOptimizer) GetProviderScores() []ProviderScore {
	co.mu.RLock()
	defer co.mu.RUnlock()

	var scores []ProviderScore
	for _, p := range co.router.Providers() {
		cfg, ok := router.DefaultCosts[p.Model]
		if !ok {
			continue
		}
		scores = append(scores, ProviderScore{
			Name:            p.Name,
			Model:           p.Model,
			CostPer1kTokens: cfg.InputPricePer1K,
			LatencyMs:       100,
			SuccessRate:     1.0,
			Score:           co.scoreProvider(p),
		})
	}
	return scores
}

func (co *CostOptimizer) save() {
	if co.path == "" {
		return
	}
	data, err := json.Marshal(struct {
		Daily   float64 `json:"daily_spent"`
		Monthly float64 `json:"monthly_spent"`
		Reset   int64   `json:"last_reset"`
	}{
		Daily:   co.dailySpent,
		Monthly: co.monthlySpent,
		Reset:   co.lastReset.Unix(),
	})
	if err != nil {
		log.Printf("[costopt] failed to marshal state: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(co.path), 0o750); err != nil {
		log.Printf("[costopt] failed to create dir: %v", err)
		return
	}
	if err := os.WriteFile(co.path, data, 0o600); err != nil {
		log.Printf("[costopt] failed to write file: %v", err)
	}
}

func (co *CostOptimizer) load() {
	if co.path == "" {
		return
	}
	data, err := os.ReadFile(co.path)
	if err != nil {
		return
	}
	var s struct {
		Daily   float64 `json:"daily_spent"`
		Monthly float64 `json:"monthly_spent"`
		Reset   int64   `json:"last_reset"`
	}
	if json.Unmarshal(data, &s) == nil {
		co.dailySpent = s.Daily
		co.monthlySpent = s.Monthly
		co.lastReset = time.Unix(s.Reset, 0)
	}
}

type BudgetStatus struct {
	DailySpent     float64 `json:"daily_spent"`
	DailyLimit     float64 `json:"daily_limit"`
	DailyPercent   float64 `json:"daily_percent"`
	MonthlySpent   float64 `json:"monthly_spent"`
	MonthlyLimit   float64 `json:"monthly_limit"`
	MonthlyPercent float64 `json:"monthly_percent"`
	Alerts         []Alert `json:"alerts"`
	CacheHitRate   float64 `json:"cache_hit_rate"`
}
