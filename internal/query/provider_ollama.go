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

type OllamaProvider struct {
	model   string
	client  *http.Client
	baseURL string
}

type OllamaProviderOption func(*OllamaProvider)

func WithOllamaClient(c *http.Client) OllamaProviderOption {
	return func(p *OllamaProvider) { p.client = c }
}

func WithOllamaBaseURL(url string) OllamaProviderOption {
	return func(p *OllamaProvider) { p.baseURL = url }
}

func NewOllamaProvider(model string, opts ...OllamaProviderOption) *OllamaProvider {
	p := &OllamaProvider{
		model:   model,
		client:  &http.Client{Timeout: 10 * time.Minute},
		baseURL: "http://localhost:11434",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *OllamaProvider) Generate(ctx context.Context, messages []Message, tools []ToolSchema) (Response, error) {
	reqBody := p.buildRequest(messages, tools)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

func (p *OllamaProvider) buildRequest(messages []Message, tools []ToolSchema) map[string]any {
	apiMsgs := make([]map[string]any, len(messages))
	for i, m := range messages {
		msg := map[string]any{"role": string(m.Role), "content": m.Content}
		if m.Role == RoleTool {
			msg["role"] = "tool"
		}
		apiMsgs[i] = msg
	}

	req := map[string]any{
		keyModel:   p.model,
		"messages": apiMsgs,
		"stream":   false,
	}

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

func (p *OllamaProvider) parseResponse(body []byte) (Response, error) {
	var raw struct {
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		DoneReason      string          `json:"done_reason"`
		Usage           json.RawMessage `json:"usage"`
		EvalCount       int             `json:"eval_count"`
		PromptEvalCount int             `json:"prompt_eval_count"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return Response{}, fmt.Errorf("parse response: %w", err)
	}

	resp := Response{
		Text: raw.Message.Content,
		Usage: Usage{
			InputTokens:  raw.PromptEvalCount,
			OutputTokens: raw.EvalCount,
		},
	}

	for _, tc := range raw.Message.ToolCalls {
		var inputMap map[string]any
		if err := json.Unmarshal(tc.Function.Arguments, &inputMap); err != nil {
			return Response{}, fmt.Errorf("parse tool_calls arguments: %w", err)
		}
		resp.ToolUses = append(resp.ToolUses, ToolUseBlock{
			Name:  tc.Function.Name,
			Input: inputMap,
		})
	}

	for i := range resp.ToolUses {
		resp.ToolUses[i].Index = i
	}

	return resp, nil
}
