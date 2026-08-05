package messages

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// SearchResult holds a single search hit.
type SearchResult struct {
	Message      Message
	MatchContent string
	Score        float64
	Highlight    []string
}

// ToolCall is a resolved tool invocation with its context.
type ToolCall struct {
	Message   Message
	ToolUse   ToolUseData
	Result    *ToolResultData
	ResultMsg *Message
	Timestamp time.Time
}

// Stats holds aggregate statistics for a conversation.
type Stats struct {
	TotalMessages    int
	TotalTokens      int
	ByRole           map[Role]int
	AvgMessageLength float64
	LongestMessage   int
	ToolCallCount    int
	ToolResultCount  int
	ErrorCount       int
	FirstMessageTime time.Time
	LastMessageTime  time.Time
	AvgTokenPerMsg   float64
}

// SearchMessages performs substring search across all message content.
// Results are scored by match count and position, with highlighted fragments.
func SearchMessages(msgs []Message, query string) []SearchResult {
	if query == "" || len(msgs) == 0 {
		return nil
	}

	queryLower := strings.ToLower(query)
	var results []SearchResult

	for _, m := range msgs {
		text := m.TextContent()
		if text == "" {
			continue
		}

		textLower := strings.ToLower(text)
		score := 0.0
		var highlights []string

		idx := 0
		for {
			pos := strings.Index(textLower[idx:], queryLower)
			if pos < 0 {
				break
			}
			absPos := idx + pos
			score += 1.0
			if absPos < 10 {
				score += 0.5
			}

			start := absPos - 20
			if start < 0 {
				start = 0
			}
			end := absPos + len(query) + 20
			if end > len(text) {
				end = len(text)
			}
			highlights = append(highlights, text[start:end])

			idx = absPos + len(query)
		}

		if score > 0 {
			results = append(results, SearchResult{
				Message:      m,
				MatchContent: text,
				Score:        score,
				Highlight:    highlights,
			})
		}
	}

	for i := range results {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// FilterByRole returns messages matching the given role.
func FilterByRole(msgs []Message, role Role) []Message {
	var result []Message
	for _, m := range msgs {
		if m.Role == role {
			result = append(result, m)
		}
	}
	return result
}

// FilterByTime returns messages within the given time range (inclusive).
func FilterByTime(msgs []Message, from, to time.Time) []Message {
	var result []Message
	for _, m := range msgs {
		if (from.IsZero() || !m.Timestamp.Before(from)) &&
			(to.IsZero() || !m.Timestamp.After(to)) {
			result = append(result, m)
		}
	}
	return result
}

// FilterByTokenRange returns messages whose estimated token count falls
// within [minTokens, maxTokens]. Use 0 for minTokens or -1 for maxTokens
// to leave a bound open.
func FilterByTokenRange(msgs []Message, minTokens, maxTokens int) []Message {
	var result []Message
	for _, m := range msgs {
		tokens := m.EstimateTokens()
		if tokens >= minTokens && (maxTokens < 0 || tokens <= maxTokens) {
			result = append(result, m)
		}
	}
	return result
}

// FilterByTool returns messages that contain a tool_use content block
// with the given name. If toolName is empty, any tool use matches.
func FilterByTool(msgs []Message, toolName string) []Message {
	var result []Message
	for _, m := range msgs {
		if m.HasToolUse(toolName) {
			result = append(result, m)
		}
	}
	return result
}

// FilterByRegex returns messages where any text content matches the
// given regular expression pattern.
func FilterByRegex(msgs []Message, pattern string) []Message {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	var result []Message
	for _, m := range msgs {
		text := m.TextContent()
		if text != "" && re.MatchString(text) {
			result = append(result, m)
		}
	}
	return result
}

// FindToolCalls resolves tool_use and tool_result pairs into ToolCall structs.
// If toolName is empty, all tool calls are returned.
func FindToolCalls(msgs []Message, toolName string) []ToolCall {
	resultIndex := make(map[string]*Message, len(msgs)/2)
	for i := range msgs {
		if msgs[i].Role == RoleToolResult {
			for _, c := range msgs[i].Contents {
				if c.Type == ContentToolResult && c.ToolResult != nil {
					resultIndex[c.ToolResult.ToolUseID] = &msgs[i]
				}
			}
		}
	}

	var calls []ToolCall
	for _, m := range msgs {
		for _, c := range m.Contents {
			if c.Type == ContentToolUse && c.ToolUse != nil {
				if toolName != "" && c.ToolUse.Name != toolName {
					continue
				}

				tc := ToolCall{
					Message:   m,
					ToolUse:   *c.ToolUse,
					Timestamp: m.Timestamp,
				}

				if resultMsg, ok := resultIndex[c.ToolUse.ID]; ok {
					tc.ResultMsg = resultMsg
					for _, rc := range resultMsg.Contents {
						if rc.Type == ContentToolResult && rc.ToolResult != nil &&
							rc.ToolResult.ToolUseID == c.ToolUse.ID {
							tc.Result = rc.ToolResult
							break
						}
					}
				}

				calls = append(calls, tc)
			}
		}
	}

	return calls
}

// GetConversationStats computes aggregate statistics for a message slice.
func GetConversationStats(msgs []Message) Stats {
	if len(msgs) == 0 {
		return Stats{
			ByRole: make(map[Role]int),
		}
	}

	stats := Stats{
		TotalMessages: len(msgs),
		ByRole:        make(map[Role]int),
	}

	totalTextLen := 0
	totalTokens := 0

	for _, m := range msgs {
		stats.ByRole[m.Role]++
		tokens := m.EstimateTokens()
		totalTokens += tokens

		textLen := len(m.TextContent())
		totalTextLen += textLen
		if textLen > stats.LongestMessage {
			stats.LongestMessage = textLen
		}

		if m.Role == RoleToolUse {
			stats.ToolCallCount++
		}
		if m.Role == RoleToolResult {
			stats.ToolResultCount++
			for _, c := range m.Contents {
				if c.Type == ContentToolResult && c.ToolResult != nil && c.ToolResult.IsError {
					stats.ErrorCount++
				}
			}
		}

		if stats.FirstMessageTime.IsZero() || m.Timestamp.Before(stats.FirstMessageTime) {
			stats.FirstMessageTime = m.Timestamp
		}
		if m.Timestamp.After(stats.LastMessageTime) {
			stats.LastMessageTime = m.Timestamp
		}
	}

	stats.TotalTokens = totalTokens
	stats.AvgMessageLength = float64(totalTextLen) / float64(len(msgs))
	stats.AvgTokenPerMsg = float64(totalTokens) / float64(len(msgs))

	return stats
}

// String returns a human-readable summary of the stats.
func (s Stats) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Messages: %d | Tokens: %d | Avg: %.0f/msg\n",
		s.TotalMessages, s.TotalTokens, s.AvgTokenPerMsg)

	for role, count := range s.ByRole {
		fmt.Fprintf(&sb, "  %s: %d\n", role, count)
	}

	if s.ToolCallCount > 0 {
		fmt.Fprintf(&sb, "  Tool calls: %d (errors: %d)\n", s.ToolCallCount, s.ErrorCount)
	}

	if !s.FirstMessageTime.IsZero() {
		duration := s.LastMessageTime.Sub(s.FirstMessageTime)
		fmt.Fprintf(&sb, "  Duration: %s\n", duration.Round(time.Second))
	}

	return sb.String()
}
