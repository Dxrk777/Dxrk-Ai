// Package messages provides message normalization, context management,
// and conversation utilities for the Dxrk conversation system.
//
// The package defines a rich message model that supports multi-modal content
// (text, images, tool uses, tool results) and offers tools for normalizing,
// searching, formatting, and managing conversation context windows.
//
// # Message Model
//
// Messages consist of a role (user, assistant, system, tool_use, tool_result)
// and a slice of Content blocks. Each content block has a type that determines
// its payload: plain text, image data, tool invocation, or tool result.
//
// Use the builder pattern for ergonomic construction:
//
//	msg := messages.NewMessage(messages.RoleAssistant).
//		Text("Processing your request...").
//		ToolUse("call-123", "read_file", map[string]any{"path": "/etc/hosts"}).
//		Build()
//
// # Normalization
//
// Raw conversation logs often contain inconsistencies: duplicate tool results,
// out-of-order tool_use/tool_result pairs, or consecutive messages from the
// same role. The NormalizeMessages function and its helpers clean these up
// before the messages enter the context window.
//
// # Context Window
//
// ContextWindow manages a bounded collection of messages within a token budget.
// It supports automatic compaction strategies that score messages by recency,
// tool result size, error status, and role priority to decide what to keep
// when the budget is tight.
//
// # Formatting
//
// FormatMessage renders a Message into various output styles (plain, markdown,
// rich, compact, verbose) suitable for terminal display, logging, or export.
//
// # Search
//
// SearchMessages performs substring matching across conversation content and
// returns scored results with highlighted matches. Filter helpers narrow by
// role, time range, token count, tool name, or regex pattern.
package messages
