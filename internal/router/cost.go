// SPDX-License-Identifier: MIT
package router

import (
	"sync"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

type CostConfig struct {
	InputPricePer1K  float64
	OutputPricePer1K float64
}

var DefaultCosts = map[string]CostConfig{
	strconst.StrClaudeSonnet420250514: {InputPricePer1K: 0.003, OutputPricePer1K: 0.015},
	"claude-sonnet-4":                 {InputPricePer1K: 0.003, OutputPricePer1K: 0.015},
	"claude-3-5-sonnet-20241022":      {InputPricePer1K: 0.003, OutputPricePer1K: 0.015},
	"claude-3-haiku-20240307":         {InputPricePer1K: 0.00025, OutputPricePer1K: 0.00125},
	"claude-opus-4-20250514":          {InputPricePer1K: 0.015, OutputPricePer1K: 0.075},
	"gpt-4o":                          {InputPricePer1K: 0.0025, OutputPricePer1K: 0.01},
	"gpt-4o-mini":                     {InputPricePer1K: 0.00015, OutputPricePer1K: 0.0006},
	"gpt-4-turbo":                     {InputPricePer1K: 0.01, OutputPricePer1K: 0.03},
	"gemini-1.5-pro":                  {InputPricePer1K: 0.00125, OutputPricePer1K: 0.005},
	"gemini-1.5-flash":                {InputPricePer1K: 0.000075, OutputPricePer1K: 0.0003},
	"gemini-2.0-flash":                {InputPricePer1K: 0.0001, OutputPricePer1K: 0.0004},
	"ollama/llama3.1:8b":              {InputPricePer1K: 0, OutputPricePer1K: 0},
	"ollama/mixtral:8x7b":             {InputPricePer1K: 0, OutputPricePer1K: 0},
	"bedrock/claude-sonnet-4":         {InputPricePer1K: 0.003, OutputPricePer1K: 0.015},
	"bedrock/claude-haiku-3":          {InputPricePer1K: 0.00025, OutputPricePer1K: 0.00125},
}

type CostTracker struct {
	mu    sync.Mutex
	costs map[string]float64
	total float64
}

func NewCostTracker() *CostTracker {
	return &CostTracker{costs: make(map[string]float64)}
}

func (ct *CostTracker) Add(model string, inputTokens, outputTokens int) {
	cfg, ok := DefaultCosts[model]
	if !ok {
		return
	}
	cost := (float64(inputTokens)/1000)*cfg.InputPricePer1K +
		(float64(outputTokens)/1000)*cfg.OutputPricePer1K
	ct.mu.Lock()
	ct.costs[model] += cost
	ct.total += cost
	ct.mu.Unlock()
}

func (ct *CostTracker) Total() float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.total
}

func (ct *CostTracker) ByModel() map[string]float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	out := make(map[string]float64, len(ct.costs))
	for k, v := range ct.costs {
		out[k] = v
	}
	return out
}

func (ct *CostTracker) Reset() {
	ct.mu.Lock()
	ct.costs = make(map[string]float64)
	ct.total = 0
	ct.mu.Unlock()
}
