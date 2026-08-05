package cost

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type ModelUsage struct {
	Model               string
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	CostUSD             float64
	Duration            time.Duration
	Calls               int
}

type SessionCost struct {
	mu                sync.Mutex
	SessionID         string
	Models            map[string]*ModelUsage
	TotalCostUSD      float64
	TotalInputTokens  int
	TotalOutputTokens int
	StartTime         time.Time
	LastActivity      time.Time
}

func NewSessionCost(sessionID string) *SessionCost {
	now := time.Now()
	return &SessionCost{
		SessionID:    sessionID,
		Models:       make(map[string]*ModelUsage),
		StartTime:    now,
		LastActivity: now,
	}
}

func (sc *SessionCost) RecordUsage(model string, inputTokens, outputTokens, cacheRead, cacheCreation int, duration time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	cost := CalculateCost(model, inputTokens, outputTokens, cacheRead, cacheCreation)

	usage, ok := sc.Models[model]
	if !ok {
		usage = &ModelUsage{Model: model}
		sc.Models[model] = usage
	}

	usage.InputTokens += inputTokens
	usage.OutputTokens += outputTokens
	usage.CacheReadTokens += cacheRead
	usage.CacheCreationTokens += cacheCreation
	usage.CostUSD += cost
	usage.Duration += duration
	usage.Calls++

	sc.TotalCostUSD += cost
	sc.TotalInputTokens += inputTokens
	sc.TotalOutputTokens += outputTokens
	sc.LastActivity = time.Now()
}

func (sc *SessionCost) GetTotalCost() float64 {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.TotalCostUSD
}

func (sc *SessionCost) GetModelBreakdown() map[string]ModelUsage {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	result := make(map[string]ModelUsage, len(sc.Models))
	for k, v := range sc.Models {
		result[k] = *v
	}
	return result
}

func (sc *SessionCost) Summary() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	var b strings.Builder

	fmt.Fprintf(&b, "Session: %s\n", sc.SessionID)
	fmt.Fprintf(&b, "Total cost: $%.4f\n", sc.TotalCostUSD)
	fmt.Fprintf(&b, "Total input tokens: %d\n", sc.TotalInputTokens)
	fmt.Fprintf(&b, "Total output tokens: %d\n", sc.TotalOutputTokens)
	fmt.Fprintf(&b, "Duration: %s\n", sc.LastActivity.Sub(sc.StartTime).Round(time.Millisecond))

	if len(sc.Models) > 0 {
		b.WriteString("Model breakdown:\n")
		for model, usage := range sc.Models {
			fmt.Fprintf(&b, "  %s: $%.4f (%d input, %d output, %d cache read, %d cache write, %d calls)\n",
				model,
				usage.CostUSD,
				usage.InputTokens,
				usage.OutputTokens,
				usage.CacheReadTokens,
				usage.CacheCreationTokens,
				usage.Calls)
		}
	}

	return b.String()
}

func (sc *SessionCost) Compact() map[string]interface{} {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	models := make(map[string]interface{}, len(sc.Models))
	for name, usage := range sc.Models {
		models[name] = map[string]interface{}{
			"input_tokens":          usage.InputTokens,
			"output_tokens":         usage.OutputTokens,
			"cache_read_tokens":     usage.CacheReadTokens,
			"cache_creation_tokens": usage.CacheCreationTokens,
			"cost_usd":              usage.CostUSD,
			"calls":                 usage.Calls,
		}
	}

	return map[string]interface{}{
		"session_id":          sc.SessionID,
		"total_cost_usd":      sc.TotalCostUSD,
		"total_input_tokens":  sc.TotalInputTokens,
		"total_output_tokens": sc.TotalOutputTokens,
		"start_time":          sc.StartTime.Unix(),
		"last_activity":       sc.LastActivity.Unix(),
		"models":              models,
	}
}

func CalculateCost(model string, inputTokens, outputTokens, cacheRead, cacheCreation int) float64 {
	var inputPrice, outputPrice, cacheReadPrice, cacheCreationPrice float64

	switch {
	case strings.Contains(model, "haiku"):
		inputPrice = 0.25
		outputPrice = 1.25
		cacheReadPrice = 0.03
		cacheCreationPrice = 0.03
	case strings.Contains(model, "sonnet"):
		inputPrice = 3.0
		outputPrice = 15.0
		cacheReadPrice = 0.3
		cacheCreationPrice = 0.3
	case strings.Contains(model, "opus"):
		inputPrice = 15.0
		outputPrice = 75.0
		cacheReadPrice = 1.5
		cacheCreationPrice = 1.5
	default:
		inputPrice = 3.0
		outputPrice = 15.0
		cacheReadPrice = 0.3
		cacheCreationPrice = 0.3
	}

	inputCost := (float64(inputTokens) / 1_000_000) * inputPrice
	outputCost := (float64(outputTokens) / 1_000_000) * outputPrice
	cacheReadCost := (float64(cacheRead) / 1_000_000) * cacheReadPrice
	cacheCreationCost := (float64(cacheCreation) / 1_000_000) * cacheCreationPrice

	return inputCost + outputCost + cacheReadCost + cacheCreationCost
}
