// SPDX-License-Identifier: MIT
package query

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

// AnthropicProvider implements Provider for the Anthropic API.
type AnthropicProvider struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

// AnthropicOption configures the Anthropic provider.
type AnthropicOption func(*AnthropicProvider)

// WithAnthropicClient sets the HTTP client.
func WithAnthropicClient(c *http.Client) AnthropicOption {
	return func(p *AnthropicProvider) { p.client = c }
}

// WithAnthropicBaseURL sets the API base URL.
func WithAnthropicBaseURL(url string) AnthropicOption {
	return func(p *AnthropicProvider) { p.baseURL = url }
}

// NewAnthropicProvider creates a new Anthropic API provider.
func NewAnthropicProvider(apiKey, model string, opts ...AnthropicOption) *AnthropicProvider {
	p := &AnthropicProvider{
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 5 * time.Minute},
		baseURL: "https://api.anthropic.com/v1",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Generate sends a request to the Anthropic Messages API.
func (p *AnthropicProvider) Generate(ctx context.Context, messages []Message, tools []ToolSchema) (Response, error) {
	reqBody := p.buildRequest(messages, tools)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return p.parseResponse(respBody)
}

func (p *AnthropicProvider) buildRequest(messages []Message, tools []ToolSchema) map[string]any {
	system := extractSystem(messages)
	chatMsgs := filterNonSystem(messages)

	req := map[string]any{
		keyModel:     p.model,
		"max_tokens": 8192,
		"messages":   toAPIMessages(chatMsgs),
	}

	if len(tools) > 0 {
		req["tools"] = toAPITools(tools)
	}

	if system != "" {
		req[strconst.StrSystem] = system
	}

	return req
}

func (p *AnthropicProvider) parseResponse(body []byte) (Response, error) {
	var raw struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		StopReason string `json:"stop_reason"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return Response{}, fmt.Errorf("parse response: %w", err)
	}

	resp := Response{
		Usage: Usage{
			InputTokens:  raw.Usage.InputTokens,
			OutputTokens: raw.Usage.OutputTokens,
		},
	}

	var textParts []string
	for _, block := range raw.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case strconst.StrToolUse:
			var inputMap map[string]any
			if err := json.Unmarshal(block.Input, &inputMap); err != nil {
				return Response{}, fmt.Errorf("parse tool_use input: %w", err)
			}
			resp.ToolUses = append(resp.ToolUses, ToolUseBlock{
				ID:    block.ID,
				Name:  block.Name,
				Input: inputMap,
			})
		}
	}

	for i := range resp.ToolUses {
		resp.ToolUses[i].Index = i
	}

	for _, t := range textParts {
		if resp.Text != "" {
			resp.Text += "\n"
		}
		resp.Text += t
	}

	return resp, nil
}

func extractSystem(msgs []Message) string {
	for _, m := range msgs {
		if m.Role == RoleSystem {
			return m.Content
		}
	}
	return ""
}

func filterNonSystem(msgs []Message) []Message {
	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role != RoleSystem {
			out = append(out, m)
		}
	}
	return out
}

func toAPIMessages(msgs []Message) []any {
	result := make([]any, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, toAPIMessage(m))
	}
	return result
}

func toAPIMessage(m Message) map[string]any {
	msg := map[string]any{
		"role": string(m.Role),
	}
	switch m.Role {
	case RoleTool:
		msg[strconst.StrContent] = []map[string]any{
			{
				"type":              strconst.StrToolResult,
				"tool_use_id":       m.ToolCallID,
				strconst.StrContent: m.Content,
			},
		}
	default:
		msg[strconst.StrContent] = m.Content
	}
	return msg
}

func toAPITools(tools []ToolSchema) []map[string]any {
	result := make([]map[string]any, len(tools))
	for i, t := range tools {
		result[i] = map[string]any{
			"name":                  t.Name,
			strconst.StrDescription: t.Description,
			"input_schema": map[string]any{
				"type":                 strconst.StrObject,
				strconst.StrProperties: t.InputSchema,
			},
		}
	}
	return result
}
