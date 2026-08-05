package tokencount

import (
	"unicode/utf8"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

// EstimationConfig overrides default estimation behavior.
type EstimationConfig struct {
	IsCode   bool
	Language string // "cjk", "english", or "" (auto-detect)
}

// Message represents a conversation message for token estimation.
type Message struct {
	Role    string
	Content string
	Tokens  int // Pre-computed token count; 0 means estimate.
}

// MessageTokens holds per-message token details.
type MessageTokens struct {
	Role      string
	Tokens    int
	Breakdown TokenBreakdown
}

// TokenBreakdown categorizes tokens by content type.
type TokenBreakdown struct {
	Text        int
	CodeBlocks  int
	ToolCalls   int
	ToolResults int
}

// ConversationTokens holds aggregate token counts for a conversation.
type ConversationTokens struct {
	Total  int
	ByRole map[string]int
	ByType TokenBreakdown
}

// ContextBudget shows remaining budget within a context window.
type ContextBudget struct {
	Used      int
	Remaining int
	Percent   float64
	Fits      bool
}

// TokenEstimator estimates token counts for text content.
// Uses heuristic-based estimation (avg 4 chars per token for English,
// ~2 chars per token for CJK, handles code blocks differently).
type TokenEstimator struct {
	// CharactersPerToken is the average characters per token estimate.
	// Default: 4.0 for English text.
	CharactersPerToken float64
	// CodeCharactersPerToken is the estimate for code content.
	// Code tends to have more tokens per character due to symbols.
	CodeCharactersPerToken float64
}

// NewEstimator returns a TokenEstimator with default settings.
func NewEstimator() *TokenEstimator {
	return &TokenEstimator{
		CharactersPerToken:     4.0,
		CodeCharactersPerToken: 3.0,
	}
}

// EstimateTokens returns the estimated token count for a string.
// It detects fenced code blocks (```) and applies a different ratio```) and applies a different ratio
// for code content. CJK characters are counted as ~1 token each.
func (e *TokenEstimator) EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	var normalTokens, codeTokens float64
	inCodeBlock := false
	lines := splitLines(text)

	for _, line := range lines {
		trimmed := trimSpace(line)
		if isCodeFence(trimmed) {
			inCodeBlock = !inCodeBlock
			codeTokens += float64(len(line)) / e.CodeCharactersPerToken
			continue
		}

		if inCodeBlock {
			codeTokens += float64(len(line)) / e.CodeCharactersPerToken
		} else {
			normalTokens += float64(countChars(line)) / e.CharactersPerToken
		}
	}

	total := int(normalTokens + codeTokens)
	if total == 0 && len(text) > 0 {
		return 1
	}
	return total
}

// EstimateTokensWithConfig returns token estimate with override settings.
func (e *TokenEstimator) EstimateTokensWithConfig(text string, cfg EstimationConfig) int {
	if len(text) == 0 {
		return 0
	}

	charPerToken := e.CharactersPerToken
	if cfg.IsCode {
		charPerToken = e.CodeCharactersPerToken
	}

	if cfg.Language == "cjk" {
		return countCJKTokens(text)
	}

	var tokens float64
	if cfg.IsCode {
		tokens = float64(len(text)) / charPerToken
	} else {
		tokens = float64(countChars(text)) / charPerToken
	}

	total := int(tokens)
	if total == 0 && len(text) > 0 {
		return 1
	}
	return total
}

// EstimateMessages returns token counts for a slice of messages.
func (e *TokenEstimator) EstimateMessages(messages []Message) []MessageTokens {
	result := make([]MessageTokens, len(messages))
	for i, msg := range messages {
		if msg.Tokens > 0 {
			result[i] = MessageTokens{
				Role:      msg.Role,
				Tokens:    msg.Tokens,
				Breakdown: e.breakdownContent(msg.Content),
			}
			continue
		}
		tokens := e.EstimateTokens(msg.Content)
		result[i] = MessageTokens{
			Role:      msg.Role,
			Tokens:    tokens,
			Breakdown: e.breakdownContent(msg.Content),
		}
	}
	return result
}

// EstimateConversation returns total tokens for a conversation.
func (e *TokenEstimator) EstimateConversation(messages []Message) ConversationTokens {
	byRole := make(map[string]int)
	var byType TokenBreakdown
	total := 0

	for _, msg := range messages {
		tokens := msg.Tokens
		if tokens <= 0 {
			tokens = e.EstimateTokens(msg.Content)
		}
		total += tokens
		byRole[msg.Role] += tokens

		bd := e.breakdownContent(msg.Content)
		byType.Text += bd.Text
		byType.CodeBlocks += bd.CodeBlocks
		byType.ToolCalls += bd.ToolCalls
		byType.ToolResults += bd.ToolResults
	}

	return ConversationTokens{
		Total:  total,
		ByRole: byRole,
		ByType: byType,
	}
}

// EstimateWithContext returns remaining tokens given a context window.
func (e *TokenEstimator) EstimateWithContext(messages []Message, systemPrompt string, maxTokens int) ContextBudget {
	systemTokens := e.EstimateTokens(systemPrompt)
	conv := e.EstimateConversation(messages)
	used := systemTokens + conv.Total
	remaining := maxTokens - used
	if remaining < 0 {
		remaining = 0
	}

	var pct float64
	if maxTokens > 0 {
		pct = float64(used) / float64(maxTokens) * 100.0
	}

	return ContextBudget{
		Used:      used,
		Remaining: remaining,
		Percent:   pct,
		Fits:      used <= maxTokens,
	}
}

func (e *TokenEstimator) breakdownContent(content string) TokenBreakdown {
	bd := TokenBreakdown{}

	lines := splitLines(content)
	inCodeBlock := false
	codeTokens := 0
	textTokens := 0

	for _, line := range lines {
		trimmed := trimSpace(line)
		if isCodeFence(trimmed) {
			if inCodeBlock {
				inCodeBlock = false
			} else {
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			codeTokens += len(line) / 3
		} else {
			textTokens += len(line) / 4
		}
	}

	bd.Text = textTokens
	bd.CodeBlocks = codeTokens

	if hasToolCalls(content) {
		bd.ToolCalls = e.EstimateTokens(content) / 3
	}
	if hasToolResults(content) {
		bd.ToolResults = e.EstimateTokens(content) / 3
	}

	return bd
}

func hasToolCalls(content string) bool {
	return containsStr(content, strconst.StrToolUse) || containsStr(content, "tool_call") || containsStr(content, "\"type\":\"tool_use\"")
}

func hasToolResults(content string) bool {
	return containsStr(content, strconst.StrToolResult) || containsStr(content, "\"type\":\"tool_result\"")
}

func countChars(s string) int {
	return utf8.RuneCountInString(s)
}

func countCJKTokens(text string) int {
	tokens := 0
	for range text {
		tokens++
	}
	if tokens == 0 && len(text) > 0 {
		return 1
	}
	return tokens
}

func isCodeFence(s string) bool {
	if len(s) < 3 {
		return false
	}
	for i := 0; i < 3 && i < len(s); i++ {
		if s[i] != '`' {
			return false
		}
	}
	return true
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func containsStr(s, substr string) bool {
	return len(substr) <= len(s) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
