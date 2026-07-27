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
)

type OpenAIProvider struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

type OpenAIProviderOption func(*OpenAIProvider)

func WithOpenAIClient(c *http.Client) OpenAIProviderOption {
	return func(p *OpenAIProvider) { p.client = c }
}

func WithOpenAIBaseURL(url string) OpenAIProviderOption {
	return func(p *OpenAIProvider) { p.baseURL = url }
}

func NewOpenAIProvider(apiKey, model string, opts ...OpenAIProviderOption) *OpenAIProvider {
	p := &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 5 * time.Minute},
		baseURL: "https://api.openai.com/v1",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *OpenAIProvider) Generate(ctx context.Context, messages []Message, tools []ToolSchema) (Response, error) {
	reqBody := p.buildRequest(messages, tools)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

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

func (p *OpenAIProvider) buildRequest(messages []Message, tools []ToolSchema) map[string]any {
	req := map[string]any{
		keyModel: p.model,
	}

	apiMsgs := make([]map[string]any, len(messages))
	for i, m := range messages {
		msg := map[string]any{"role": string(m.Role)}
		switch m.Role {
		case RoleTool:
			msg["role"] = "tool"
			msg["tool_call_id"] = m.ToolCallID
			msg["content"] = m.Content
		case RoleAssistant:
			msg["content"] = m.Content
		default:
			msg["content"] = m.Content
		}
		apiMsgs[i] = msg
	}
	req["messages"] = apiMsgs

	if len(tools) > 0 {
		apiTools := make([]map[string]any, len(tools))
		for i, t := range tools {
			apiTools[i] = map[string]any{
				keyType: keyFunction,
				keyFunction: map[string]any{
					"name":         t.Name,
					keyDescription: t.Description,
					keyParameters: map[string]any{
						keyType:       valObject,
						keyProperties: t.InputSchema,
					},
				},
			}
		}
		req["tools"] = apiTools
	}

	return req
}

func (p *OpenAIProvider) parseResponse(body []byte) (Response, error) {
	var raw struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			InputTokens  int `json:"prompt_tokens"`
			OutputTokens int `json:"completion_tokens"`
		} `json:"usage"`
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

	if len(raw.Choices) > 0 {
		resp.Text = raw.Choices[0].Message.Content
		for _, tc := range raw.Choices[0].Message.ToolCalls {
			var inputMap map[string]any
			if err := json.Unmarshal(tc.Function.Arguments, &inputMap); err != nil {
				return Response{}, fmt.Errorf("parse tool_use arguments: %w", err)
			}
			resp.ToolUses = append(resp.ToolUses, ToolUseBlock{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: inputMap,
			})
		}
	}

	for i := range resp.ToolUses {
		resp.ToolUses[i].Index = i
	}

	return resp, nil
}
