// SPDX-License-Identifier: MIT
package query

import "context"

// Provider is the LLM backend abstraction.
type Provider interface {
	// Generate sends messages and tool schemas to the LLM and returns a response.
	Generate(ctx context.Context, messages []Message, tools []ToolSchema) (Response, error)
}

// ToolSchema describes a tool to the LLM.
type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// Response from the LLM.
type Response struct {
	Text     string         `json:"text"`
	ToolUses []ToolUseBlock `json:"tool_uses,omitempty"`
	Usage    Usage          `json:"usage"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
