package tokencount

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// --- Estimator tests ---

func TestEstimateTokensEmpty(t *testing.T) {
	e := NewEstimator()
	if got := e.EstimateTokens(""); got != 0 {
		t.Errorf("EstimateTokens(\"\") = %d, want 0", got)
	}
}

func TestEstimateTokensEnglish(t *testing.T) {
	e := NewEstimator()
	text := "Hello world, this is a test message."
	got := e.EstimateTokens(text)
	// 36 chars / 4 = 9 tokens
	if got < 8 || got > 10 {
		t.Errorf("EstimateTokens(english) = %d, want ~9", got)
	}
}

func TestEstimateTokensCodeBlock(t *testing.T) {
	e := NewEstimator()
	text := "Here is some code:\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\nDone."
	got := e.EstimateTokens(text)
	// Code lines are divided by 3, text lines by 4
	if got < 5 || got > 20 {
		t.Errorf("EstimateTokens(code block) = %d, want ~10-15", got)
	}
}

func TestEstimateTokensCJK(t *testing.T) {
	e := NewEstimator()
	text := "你好世界"
	got := e.EstimateTokensWithConfig(text, EstimationConfig{Language: "cjk"})
	// 4 CJK chars = 4 tokens
	if got != 4 {
		t.Errorf("EstimateTokensWithConfig(cjk) = %d, want 4", got)
	}
}

func TestEstimateTokensCodeConfig(t *testing.T) {
	e := NewEstimator()
	text := "func main() { return }"
	got := e.EstimateTokensWithConfig(text, EstimationConfig{IsCode: true})
	// 23 chars / 3 ≈ 7
	if got < 6 || got > 9 {
		t.Errorf("EstimateTokensWithConfig(code) = %d, want ~7", got)
	}
}

func TestEstimateMessages(t *testing.T) {
	e := NewEstimator()
	messages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}
	result := e.EstimateMessages(messages)
	if len(result) != 3 {
		t.Fatalf("EstimateMessages returned %d results, want 3", len(result))
	}
	for i, mt := range result {
		if mt.Tokens <= 0 {
			t.Errorf("message %d: tokens = %d, want > 0", i, mt.Tokens)
		}
	}
}

func TestEstimateMessagesPrecomputed(t *testing.T) {
	e := NewEstimator()
	messages := []Message{
		{Role: "user", Content: "Hello", Tokens: 10},
	}
	result := e.EstimateMessages(messages)
	if result[0].Tokens != 10 {
		t.Errorf("EstimateMessages used precomputed: got %d, want 10", result[0].Tokens)
	}
}

func TestEstimateConversation(t *testing.T) {
	e := NewEstimator()
	messages := []Message{
		{Role: "user", Content: "Hello world"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "user", Content: "Goodbye"},
	}
	conv := e.EstimateConversation(messages)
	if conv.Total <= 0 {
		t.Errorf("EstimateConversation total = %d, want > 0", conv.Total)
	}
	if conv.ByRole["user"] == 0 {
		t.Error("ByRole[user] = 0, want > 0")
	}
	if conv.ByRole["assistant"] == 0 {
		t.Error("ByRole[assistant] = 0, want > 0")
	}
}

func TestEstimateConversationCode(t *testing.T) {
	e := NewEstimator()
	messages := []Message{
		{Role: "assistant", Content: "```go\nfmt.Println(\"hi\")\n```"},
	}
	conv := e.EstimateConversation(messages)
	if conv.ByType.CodeBlocks == 0 {
		t.Error("ByType.CodeBlocks = 0, want > 0 for code content")
	}
}

func TestEstimateWithContext(t *testing.T) {
	e := NewEstimator()
	messages := []Message{
		{Role: "user", Content: strings.Repeat("hello ", 100)},
	}
	systemPrompt := "You are a helpful assistant."
	budget := e.EstimateWithContext(messages, systemPrompt, 200000)

	if budget.Used <= 0 {
		t.Errorf("Used = %d, want > 0", budget.Used)
	}
	if budget.Remaining <= 0 {
		t.Errorf("Remaining = %d, want > 0", budget.Remaining)
	}
	if !budget.Fits {
		t.Error("Fits = false, want true for small messages")
	}
}

func TestEstimateWithContextOverflow(t *testing.T) {
	e := NewEstimator()
	messages := []Message{
		{Role: "user", Content: strings.Repeat("hello ", 50000)},
	}
	budget := e.EstimateWithContext(messages, "", 100)
	if budget.Fits {
		t.Error("Fits = true, want false when messages exceed context")
	}
}

func TestEstimateTokensLongText(t *testing.T) {
	e := NewEstimator()
	text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 1000)
	got := e.EstimateTokens(text)
	expected := 46000 / 4 // ~11500
	if got < 10000 || got > 13000 {
		t.Errorf("EstimateTokens(long text) = %d, want ~%d", got, expected)
	}
}

// --- RunningCounter tests ---

func TestRunningCounterAddAndStatus(t *testing.T) {
	rc := NewRunningCounter(1000)
	status := rc.AddMessage(Message{Role: "user", Content: "Hello world"})
	if status.Total <= 0 {
		t.Errorf("Total = %d, want > 0", status.Total)
	}
	if status.Max != 1000 {
		t.Errorf("Max = %d, want 1000", status.Max)
	}
}

func TestRunningCounterRemoveLast(t *testing.T) {
	rc := NewRunningCounter(1000)
	rc.AddMessage(Message{Role: "user", Content: "Hello"})
	before := rc.Status().Total
	rc.AddMessage(Message{Role: "assistant", Content: "Hi there!"})
	rc.RemoveLastMessage()
	after := rc.Status().Total
	if after >= before+10 {
		t.Errorf("After remove: %d, expected to decrease from %d", after, before+10)
	}
}

func TestRunningCounterRemoveEmpty(t *testing.T) {
	rc := NewRunningCounter(1000)
	status := rc.RemoveLastMessage()
	if status.Total != 0 {
		t.Errorf("RemoveLastMessage on empty: Total = %d, want 0", status.Total)
	}
}

func TestRunningCounterReset(t *testing.T) {
	rc := NewRunningCounter(1000)
	rc.AddMessage(Message{Role: "user", Content: "Hello"})
	rc.Reset()
	status := rc.Status()
	if status.Total != 0 {
		t.Errorf("After reset: Total = %d, want 0", status.Total)
	}
}

func TestRunningCounterWarning(t *testing.T) {
	rc := NewRunningCounter(100)
	rc.SetWarningThreshold(0.5)
	// Add messages that use > 50% of 100 tokens (> 50 chars)
	rc.AddMessage(Message{Role: "user", Content: strings.Repeat("x", 300)})
	status := rc.Status()
	if !status.Warning {
		t.Error("Warning = false, want true when above 50% threshold")
	}
}

func TestRunningCounterNeedsCompaction(t *testing.T) {
	rc := NewRunningCounter(100)
	rc.SetWarningThreshold(0.5)
	rc.AddMessage(Message{Role: "user", Content: strings.Repeat("x", 300)})
	if !rc.NeedsCompaction() {
		t.Error("NeedsCompaction = false, want true when above threshold")
	}
}

func TestRunningCounterSetMaxTokens(t *testing.T) {
	rc := NewRunningCounter(1000)
	rc.AddMessage(Message{Role: "user", Content: "Hello"})
	rc.SetMaxTokens(500)
	status := rc.Status()
	if status.Max != 500 {
		t.Errorf("Max = %d, want 500", status.Max)
	}
}

func TestRunningCounterSetWarningThresholdClamp(t *testing.T) {
	rc := NewRunningCounter(1000)
	rc.SetWarningThreshold(-0.5)
	rc.SetWarningThreshold(1.5)
	// Should not panic; thresholds clamped to [0, 1]
}

func TestRunningCounterMessagesNeedingCompaction(t *testing.T) {
	rc := NewRunningCounter(100)
	rc.SetWarningThreshold(0.5)
	for i := 0; i < 20; i++ {
		rc.AddMessage(Message{Role: "user", Content: strings.Repeat("x", 50)})
	}
	msgs := rc.MessagesNeedingCompaction()
	if len(msgs) == 0 {
		t.Error("MessagesNeedingCompaction returned empty, want candidates")
	}
}

func TestRunningCounterConcurrentAccess(t *testing.T) {
	rc := NewRunningCounter(1000000)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			rc.AddMessage(Message{
				Role:    "user",
				Content: fmt.Sprintf("Message %d with some content", n),
			})
			_ = rc.Status()
			_ = rc.NeedsCompaction()
		}(i)
	}
	wg.Wait()
	// No race condition means success
}

// --- BudgetManager tests ---

func TestBudgetManagerAllocate(t *testing.T) {
	bm := NewBudgetManager(100000, 10000)
	err := bm.Allocate("system", 5000)
	if err != nil {
		t.Fatalf("Allocate(system) error: %v", err)
	}
	err = bm.Allocate("tools", 3000)
	if err != nil {
		t.Fatalf("Allocate(tools) error: %v", err)
	}
	avail := bm.Available()
	// 100000 - 10000 (reserved) - 5000 (system) - 3000 (tools) = 82000
	if avail != 82000 {
		t.Errorf("Available = %d, want 82000", avail)
	}
}

func TestBudgetManagerAllocateExceeds(t *testing.T) {
	bm := NewBudgetManager(100, 10)
	err := bm.Allocate("system", 200)
	if err == nil {
		t.Error("Allocate(200) in 100 budget: got nil error, want error")
	}
}

func TestBudgetManagerAllocateNegative(t *testing.T) {
	bm := NewBudgetManager(100000, 10000)
	err := bm.Allocate("bad", -100)
	if err == nil {
		t.Error("Allocate(-100): got nil error, want error")
	}
}

func TestBudgetManagerFitsConversation(t *testing.T) {
	bm := NewBudgetManager(100000, 10000)
	bm.Allocate("system", 5000)
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}
	if !bm.FitsConversation(messages) {
		t.Error("FitsConversation = false, want true for small messages")
	}
}

func TestBudgetManagerFitsConversationExceeds(t *testing.T) {
	bm := NewBudgetManager(100, 10)
	bm.Allocate("system", 80)
	messages := []Message{
		{Role: "user", Content: strings.Repeat("x", 500)},
	}
	if bm.FitsConversation(messages) {
		t.Error("FitsConversation = true, want false when exceeds budget")
	}
}

func TestBudgetManagerFitsWithSuggestion(t *testing.T) {
	bm := NewBudgetManager(100, 10)
	bm.Allocate("system", 80)
	messages := []Message{
		{Role: "user", Content: strings.Repeat("x", 500)},
	}
	suggestion := bm.FitsConversationWithSuggestion(messages)
	if suggestion.Fits {
		t.Error("Suggestion.Fits = true, want false")
	}
	if suggestion.ExcessTokens <= 0 {
		t.Errorf("ExcessTokens = %d, want > 0", suggestion.ExcessTokens)
	}
	if len(suggestion.SuggestedDrops) == 0 {
		t.Error("SuggestedDrops is empty, want indices")
	}
}

func TestBudgetManagerFitsWithSuggestionFits(t *testing.T) {
	bm := NewBudgetManager(100000, 10000)
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}
	suggestion := bm.FitsConversationWithSuggestion(messages)
	if !suggestion.Fits {
		t.Error("Suggestion.Fits = false, want true")
	}
}

func TestBudgetManagerSummary(t *testing.T) {
	bm := NewBudgetManager(100000, 10000)
	bm.Allocate("system", 5000)
	bm.Allocate("tools", 3000)
	summary := bm.Summary()
	if summary.MaxContext != 100000 {
		t.Errorf("MaxContext = %d, want 100000", summary.MaxContext)
	}
	if summary.TotalAllocated != 8000 {
		t.Errorf("TotalAllocated = %d, want 8000", summary.TotalAllocated)
	}
	if summary.ReservedForResponse != 10000 {
		t.Errorf("ReservedForResponse = %d, want 10000", summary.ReservedForResponse)
	}
}

func TestBudgetManagerAdjustForResponse(t *testing.T) {
	bm := NewBudgetManager(100000, 10000)
	bm.Allocate("system", 5000)
	availBefore := bm.Available()
	bm.AdjustForResponse()
	availAfter := bm.Available()
	if availAfter >= availBefore {
		t.Errorf("Available after AdjustForResponse: %d, want < %d", availAfter, availBefore)
	}
}

func TestBudgetManagerReleaseResponseBudget(t *testing.T) {
	bm := NewBudgetManager(100000, 10000)
	bm.Allocate("system", 5000)
	bm.AdjustForResponse()
	bm.ReleaseResponseBudget()
	// After release, available should return to original
	avail := bm.Available()
	if avail != 85000 {
		t.Errorf("Available after release = %d, want 85000", avail)
	}
}

// --- Edge cases ---

func TestEstimateTokensSingleChar(t *testing.T) {
	e := NewEstimator()
	got := e.EstimateTokens("a")
	if got != 1 {
		t.Errorf("EstimateTokens(\"a\") = %d, want 1", got)
	}
}

func TestEstimateTokensOnlyFences(t *testing.T) {
	e := NewEstimator()
	got := e.EstimateTokens("```\n```")
	if got < 0 {
		t.Errorf("EstimateTokens(code fences only) = %d, want >= 0", got)
	}
}

func TestEstimateMessagesEmpty(t *testing.T) {
	e := NewEstimator()
	result := e.EstimateMessages(nil)
	if len(result) != 0 {
		t.Errorf("EstimateMessages(nil) returned %d results, want 0", len(result))
	}
}

func TestEstimateConversationEmpty(t *testing.T) {
	e := NewEstimator()
	conv := e.EstimateConversation(nil)
	if conv.Total != 0 {
		t.Errorf("EstimateConversation(nil).Total = %d, want 0", conv.Total)
	}
}

func TestEstimateWithContextZeroMax(t *testing.T) {
	e := NewEstimator()
	messages := []Message{{Role: "user", Content: "Hello"}}
	budget := e.EstimateWithContext(messages, "", 0)
	if budget.Fits {
		t.Error("Fits = true with maxTokens=0, want false")
	}
}

func TestRunningCounterPrecomputedTokens(t *testing.T) {
	rc := NewRunningCounter(1000)
	status := rc.AddMessage(Message{Role: "user", Content: "Hello", Tokens: 42})
	if status.Total != 42 {
		t.Errorf("Total = %d, want 42 (precomputed)", status.Total)
	}
}

func TestBudgetManagerSummarySections(t *testing.T) {
	bm := NewBudgetManager(100000, 0)
	bm.Allocate("system", 5000)
	bm.Allocate("tools", 3000)
	summary := bm.Summary()
	if summary.Allocated["system"] != 5000 {
		t.Errorf("Allocated[system] = %d, want 5000", summary.Allocated["system"])
	}
	if summary.Allocated["tools"] != 3000 {
		t.Errorf("Allocated[tools] = %d, want 3000", summary.Allocated["tools"])
	}
}
