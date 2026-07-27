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

type GeminiProvider struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

type GeminiProviderOption func(*GeminiProvider)

func WithGeminiClient(c *http.Client) GeminiProviderOption {
	return func(p *GeminiProvider) { p.client = c }
}

func WithGeminiBaseURL(url string) GeminiProviderOption {
	return func(p *GeminiProvider) { p.baseURL = url }
}

func NewGeminiProvider(apiKey, model string, opts ...GeminiProviderOption) *GeminiProvider {
	p := &GeminiProvider{
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 5 * time.Minute},
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *GeminiProvider) Generate(ctx context.Context, messages []Message, tools []ToolSchema) (Response, error) {
	reqBody := p.buildRequest(messages, tools)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent", p.baseURL, p.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.apiKey)

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

func (p *GeminiProvider) buildRequest(messages []Message, tools []ToolSchema) map[string]any {
	parts := make([]map[string]any, 0)

	for _, m := range messages {
		if m.Role == RoleSystem {
			continue
		}
		part := map[string]any{
			"role": m.Role,
			"parts": []map[string]any{
				{"text": m.Content},
			},
		}
		parts = append(parts, part)
	}

	req := map[string]any{
		"contents": parts,
	}

	system := extractSystem(messages)
	if system != "" {
		req["system_instruction"] = map[string]any{
			"parts": []map[string]any{{"text": system}},
		}
	}

	if len(tools) > 0 {
		apiTools := make([]map[string]any, len(tools))
		for i, t := range tools {
			apiTools[i] = map[string]any{
				"functionDeclarations": []map[string]any{
					{
						"name":         t.Name,
						keyDescription: t.Description,
						keyParameters: map[string]any{
							keyType:       valObject,
							keyProperties: t.InputSchema,
						},
					},
				},
			}
		}
		req["tools"] = apiTools
	}

	return req
}

func (p *GeminiProvider) parseResponse(body []byte) (Response, error) {
	var raw struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
				Role string `json:"role"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return Response{}, fmt.Errorf("parse response: %w", err)
	}

	resp := Response{
		Usage: Usage{
			InputTokens:  raw.UsageMetadata.PromptTokenCount,
			OutputTokens: raw.UsageMetadata.CandidatesTokenCount,
		},
	}

	if len(raw.Candidates) > 0 {
		for _, part := range raw.Candidates[0].Content.Parts {
			if part.Text != "" {
				if resp.Text != "" {
					resp.Text += "\n"
				}
				resp.Text += part.Text
			}
			if part.FunctionCall != nil {
				var inputMap map[string]any
				if err := json.Unmarshal(part.FunctionCall.Args, &inputMap); err != nil {
					return Response{}, fmt.Errorf("parse functionCall args: %w", err)
				}
				resp.ToolUses = append(resp.ToolUses, ToolUseBlock{
					Name:  part.FunctionCall.Name,
					Input: inputMap,
				})
			}
		}
	}

	for i := range resp.ToolUses {
		resp.ToolUses[i].Index = i
	}

	return resp, nil
}
