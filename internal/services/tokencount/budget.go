package tokencount

import (
	"fmt"
)

// TruncationSuggestion describes how to fit messages within a budget.
type TruncationSuggestion struct {
	Fits                 bool
	ExcessTokens         int
	SuggestedDrops       []int             // indices of messages to drop entirely
	SuggestedTruncations []TruncationRange // ranges to truncate in place
}

// TruncationRange defines a contiguous range of messages to truncate.
type TruncationRange struct {
	Start          int
	End            int
	OriginalTokens int
	TargetTokens   int
}

// BudgetSummary breaks down budget allocation across named sections.
type BudgetSummary struct {
	MaxContext          int
	Allocated           map[string]int
	Available           int
	ReservedForResponse int
	TotalAllocated      int
}

// BudgetManager manages token allocation across different context sections.
type BudgetManager struct {
	estimator           *TokenEstimator
	maxContext          int
	reservedForResponse int
	sections            map[string]int
}

// NewBudgetManager returns a manager for the given context window size.
func NewBudgetManager(maxContext, reservedForResponse int) *BudgetManager {
	return &BudgetManager{
		estimator:           NewEstimator(),
		maxContext:          maxContext,
		reservedForResponse: reservedForResponse,
		sections:            make(map[string]int),
	}
}

// Allocate reserves tokens for a named section (e.g., "system", "tools", "conversation").
func (bm *BudgetManager) Allocate(name string, tokens int) error {
	if tokens < 0 {
		return fmt.Errorf("tokencount: negative allocation for %q", name)
	}
	currentTotal := bm.totalAllocated()
	if currentTotal+tokens > bm.maxContext-bm.reservedForResponse {
		return fmt.Errorf("tokencount: allocation of %d for %q exceeds available budget (%d)",
			tokens, name, bm.maxContext-bm.reservedForResponse-currentTotal)
	}
	bm.sections[name] = tokens
	return nil
}

// Available returns tokens available for conversation after all allocations.
func (bm *BudgetManager) Available() int {
	used := bm.totalAllocated()
	avail := bm.maxContext - bm.reservedForResponse - used
	if avail < 0 {
		return 0
	}
	return avail
}

// FitsConversation checks if a set of messages fits in remaining budget.
func (bm *BudgetManager) FitsConversation(messages []Message) bool {
	tokens := bm.estimateConversation(messages)
	return tokens <= bm.Available()
}

// FitsConversationWithSuggestion checks fit and suggests truncation if needed.
func (bm *BudgetManager) FitsConversationWithSuggestion(messages []Message) TruncationSuggestion {
	available := bm.Available()
	totalTokens := bm.estimateConversation(messages)

	if totalTokens <= available {
		return TruncationSuggestion{
			Fits:         true,
			ExcessTokens: 0,
		}
	}

	excess := totalTokens - available

	type msgInfo struct {
		index  int
		tokens int
	}
	msgInfos := make([]msgInfo, len(messages))
	for i, msg := range messages {
		tokens := msg.Tokens
		if tokens <= 0 {
			tokens = bm.estimator.EstimateTokens(msg.Content)
		}
		msgInfos[i] = msgInfo{index: i, tokens: tokens}
	}

	// Find oldest messages to drop first
	var suggestedDrops []int
	accumulated := 0
	for _, mi := range msgInfos {
		accumulated += mi.tokens
		suggestedDrops = append(suggestedDrops, mi.index)
		if accumulated >= excess {
			break
		}
	}

	return TruncationSuggestion{
		Fits:           false,
		ExcessTokens:   excess,
		SuggestedDrops: suggestedDrops,
	}
}

// Summary returns a breakdown of budget usage.
func (bm *BudgetManager) Summary() BudgetSummary {
	sectionsCopy := make(map[string]int, len(bm.sections))
	for k, v := range bm.sections {
		sectionsCopy[k] = v
	}

	return BudgetSummary{
		MaxContext:          bm.maxContext,
		Allocated:           sectionsCopy,
		Available:           bm.Available(),
		ReservedForResponse: bm.reservedForResponse,
		TotalAllocated:      bm.totalAllocated(),
	}
}

// AdjustForResponse reduces available budget when generating a response.
func (bm *BudgetManager) AdjustForResponse() {
	bm.sections["_response_reserved"] = bm.reservedForResponse
}

// ReleaseResponseBudget returns the response reservation.
func (bm *BudgetManager) ReleaseResponseBudget() {
	delete(bm.sections, "_response_reserved")
}

func (bm *BudgetManager) totalAllocated() int {
	total := 0
	for _, v := range bm.sections {
		total += v
	}
	return total
}

func (bm *BudgetManager) estimateConversation(messages []Message) int {
	total := 0
	for _, msg := range messages {
		tokens := msg.Tokens
		if tokens <= 0 {
			tokens = bm.estimator.EstimateTokens(msg.Content)
		}
		total += tokens
	}
	return total
}
