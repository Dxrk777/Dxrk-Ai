// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
	"github.com/spf13/cobra"
)

// CostConfig defines per-model pricing (USD per 1K tokens).
type CostConfig struct {
	InputPricePer1K  float64
	OutputPricePer1K float64
}

var modelCosts = map[string]CostConfig{
	strconst.StrClaudeSonnet420250514: {InputPricePer1K: 0.003, OutputPricePer1K: 0.015},
	"claude-sonnet-4":                 {InputPricePer1K: 0.003, OutputPricePer1K: 0.015},
	"claude-3-5-sonnet-20241022":      {InputPricePer1K: 0.003, OutputPricePer1K: 0.015},
	"claude-3-haiku-20240307":         {InputPricePer1K: 0.00025, OutputPricePer1K: 0.00125},
	"claude-opus-4-20250514":          {InputPricePer1K: 0.015, OutputPricePer1K: 0.075},
	"gpt-4o":                          {InputPricePer1K: 0.0025, OutputPricePer1K: 0.01},
	"gpt-4o-mini":                     {InputPricePer1K: 0.00015, OutputPricePer1K: 0.0006},
	"gemini-2.0-flash":                {InputPricePer1K: 0.0001, OutputPricePer1K: 0.0004},
}

// CostTracker accumulates cost data for the current session.
type CostTracker struct {
	mu    sync.Mutex
	costs map[string]float64
	total float64
}

var globalCostTracker = &CostTracker{
	costs: make(map[string]float64),
}

func RegisterCostCommand(reg *Registry) {
	reg.AddCommand(&cobra.Command{
		Use:   "cost",
		Short: "Show cost tracking and estimates",
		Long:  "Display estimated costs by model for the current session.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCost()
		},
	})
}

// TrackCost records cost for a model based on token usage.
func TrackCost(model string, inputTokens, outputTokens int) {
	cfg, ok := modelCosts[model]
	if !ok {
		return
	}
	cost := (float64(inputTokens)/1000)*cfg.InputPricePer1K +
		(float64(outputTokens)/1000)*cfg.OutputPricePer1K

	globalCostTracker.mu.Lock()
	globalCostTracker.costs[model] += cost
	globalCostTracker.total += cost
	globalCostTracker.mu.Unlock()
}

// ResetCosts clears all tracked costs.
func ResetCosts() {
	globalCostTracker.mu.Lock()
	globalCostTracker.costs = make(map[string]float64)
	globalCostTracker.total = 0
	globalCostTracker.mu.Unlock()
}

func runCost() error {
	globalCostTracker.mu.Lock()
	defer globalCostTracker.mu.Unlock()

	if len(globalCostTracker.costs) == 0 {
		fmt.Fprintln(os.Stderr, "No costs recorded yet.")
		return nil
	}

	type entry struct {
		model string
		cost  float64
	}
	entries := make([]entry, 0, len(globalCostTracker.costs))
	for m, c := range globalCostTracker.costs {
		entries = append(entries, entry{model: m, cost: c})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].cost > entries[j].cost
	})

	fmt.Fprintf(os.Stderr, "Cost Breakdown (estimated)\n")
	fmt.Fprintf(os.Stderr, "─────────────────────────\n")
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "  %-30s  $%.6f\n", e.model, e.cost)
	}
	fmt.Fprintf(os.Stderr, "─────────────────────────\n")
	fmt.Fprintf(os.Stderr, "  %-30s  $%.6f\n", "TOTAL", globalCostTracker.total)

	return nil
}
