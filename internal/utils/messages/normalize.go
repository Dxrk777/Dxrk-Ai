package messages

import (
	"sort"
	"strings"
)

// NormalizeMessages applies a standard set of fixups to a message slice:
// merge consecutive same-role messages, deduplicate tool results, fix
// tool result ordering, and compact text-only contents.
func NormalizeMessages(msgs []Message) []Message {
	if len(msgs) == 0 {
		return msgs
	}
	msgs = FixToolResultOrder(msgs)
	msgs = DeduplicateToolResults(msgs)
	msgs = MergeConsecutiveRole(msgs, RoleUser)
	msgs = MergeConsecutiveRole(msgs, RoleAssistant)
	msgs = CompactContent(msgs)
	return msgs
}

// MergeConsecutiveRole merges consecutive messages with the same role into
// a single message. Contents are concatenated in order. Metadata from the
// first message is kept; timestamps use the earliest.
func MergeConsecutiveRole(msgs []Message, role Role) []Message {
	if len(msgs) == 0 {
		return msgs
	}

	result := make([]Message, 0, len(msgs))
	var current *Message

	for i := range msgs {
		if msgs[i].Role == role {
			if current == nil {
				cp := msgs[i]
				current = &cp
			} else {
				current.Contents = append(current.Contents, msgs[i].Contents...)
				if msgs[i].TokenCount > 0 {
					current.TokenCount += msgs[i].TokenCount
				}
				if msgs[i].Timestamp.Before(current.Timestamp) {
					current.Timestamp = msgs[i].Timestamp
				}
			}
		} else {
			if current != nil {
				result = append(result, *current)
				current = nil
			}
			result = append(result, msgs[i])
		}
	}
	if current != nil {
		result = append(result, *current)
	}
	return result
}

// StripSystemMessages removes all messages with RoleSystem.
func StripSystemMessages(msgs []Message) []Message {
	result := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role != RoleSystem {
			result = append(result, m)
		}
	}
	return result
}

// DeduplicateToolResults removes duplicate tool results (same ToolUseID)
// keeping only the first occurrence.
func DeduplicateToolResults(msgs []Message) []Message {
	seen := make(map[string]bool, len(msgs))
	result := make([]Message, 0, len(msgs))

	for _, m := range msgs {
		if m.Role == RoleToolResult {
			for _, c := range m.Contents {
				if c.Type == ContentToolResult && c.ToolResult != nil {
					if seen[c.ToolResult.ToolUseID] {
						goto skip
					}
					seen[c.ToolResult.ToolUseID] = true
				}
			}
		}
		result = append(result, m)
	skip:
	}
	return result
}

// FixToolResultOrder ensures every tool_result message follows its
// corresponding tool_use. If a tool_result appears before its tool_use,
// it is moved to after the tool_use. This operates on a best-effort basis.
func FixToolResultOrder(msgs []Message) []Message {
	if len(msgs) <= 1 {
		return msgs
	}

	toolUses := make(map[string]int, len(msgs)/2)
	toolResults := make(map[string]int, len(msgs)/2)

	for i, m := range msgs {
		for _, c := range m.Contents {
			if c.Type == ContentToolUse && c.ToolUse != nil {
				toolUses[c.ToolUse.ID] = i
			}
			if c.Type == ContentToolResult && c.ToolResult != nil {
				toolResults[c.ToolResult.ToolUseID] = i
			}
		}
	}

	needsReorder := false
	for id, resultIdx := range toolResults {
		useIdx, ok := toolUses[id]
		if ok && resultIdx < useIdx {
			needsReorder = true
			break
		}
		_ = useIdx
	}

	if !needsReorder {
		return msgs
	}

	type indexedMsg struct {
		msg Message
		idx int
	}
	ordered := make([]indexedMsg, 0, len(msgs))
	for i, m := range msgs {
		ordered = append(ordered, indexedMsg{msg: m, idx: i})
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].idx < ordered[j].idx
	})

	result := make([]Message, 0, len(msgs))
	pending := make(map[string]Message, len(toolResults))

	for _, im := range ordered {
		for _, c := range im.msg.Contents {
			if c.Type == ContentToolUse && c.ToolUse != nil {
				if resultMsg, ok := pending[c.ToolUse.ID]; ok {
					result = append(result, resultMsg)
					delete(pending, c.ToolUse.ID)
				}
			}
		}
		result = append(result, im.msg)
	}

	for _, m := range pending {
		result = append(result, m)
	}

	return result
}

// CompactContent merges consecutive text-only content blocks within
// each message into a single content block.
func CompactContent(msgs []Message) []Message {
	result := make([]Message, len(msgs))
	for i, m := range msgs {
		compacted := compactMessageContents(m)
		result[i] = compacted
	}
	return result
}

func compactMessageContents(m Message) Message {
	if len(m.Contents) <= 1 {
		return m
	}

	var textParts []string
	other := make([]Content, 0, len(m.Contents))

	for _, c := range m.Contents {
		if c.Type == ContentText && c.Text != "" {
			textParts = append(textParts, c.Text)
		} else {
			other = append(other, c)
		}
	}

	if len(textParts) <= 1 {
		return m
	}

	merged := Content{
		Type: ContentText,
		Text: strings.Join(textParts, "\n"),
	}

	result := make([]Content, 0, len(other)+1)
	result = append(result, merged)
	result = append(result, other...)
	m.Contents = result
	return m
}

// TruncateByTokens keeps the most recent messages that fit within the
// given token budget. System messages are always preserved. Returns
// the truncated slice and sets TokenCount on retained messages.
func TruncateByTokens(msgs []Message, maxTokens int) []Message {
	if len(msgs) == 0 || maxTokens <= 0 {
		return nil
	}

	var systemMsgs []Message
	var nonSystem []Message

	for _, m := range msgs {
		if m.Role == RoleSystem {
			systemMsgs = append(systemMsgs, m)
		} else {
			nonSystem = append(nonSystem, m)
		}
	}

	systemTokens := 0
	for _, m := range systemMsgs {
		systemTokens += m.EstimateTokens()
	}

	budget := maxTokens - systemTokens
	if budget <= 0 {
		return systemMsgs
	}

	result := make([]Message, 0, len(nonSystem))
	used := 0
	for i := len(nonSystem) - 1; i >= 0; i-- {
		tokens := nonSystem[i].EstimateTokens()
		if used+tokens > budget {
			break
		}
		used += tokens
		result = append(result, nonSystem[i])
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	result = append(systemMsgs, result...)
	return result
}

// CountTokens returns the total estimated token count across all messages.
func CountTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += m.EstimateTokens()
	}
	return total
}
