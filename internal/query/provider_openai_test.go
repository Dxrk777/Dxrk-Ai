// SPDX-License-Identifier: MIT
package query

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIProvider_GenerateTextOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want Bearer test-key", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content":    "Hello from OpenAI!",
						"tool_calls": []any{},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider("test-key", "gpt-4", WithOpenAIBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Say hi"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "Hello from OpenAI!" {
		t.Fatalf("Text = %q, want %q", resp.Text, "Hello from OpenAI!")
	}
	if len(resp.ToolUses) != 0 {
		t.Fatalf("ToolUses = %d, want 0", len(resp.ToolUses))
	}
	if resp.Usage.InputTokens != 10 {
		t.Fatalf("InputTokens = %d, want 10", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 5 {
		t.Fatalf("OutputTokens = %d, want 5", resp.Usage.OutputTokens)
	}
}

func TestOpenAIProvider_GenerateWithToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		msgs, ok := reqBody["messages"].([]any)
		if !ok || len(msgs) == 0 {
			t.Fatal("no messages in request")
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "Let me check...",
						"tool_calls": []map[string]any{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]any{
									"name":      "get_weather",
									"arguments": json.RawMessage(`{"city":"London"}`),
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     20,
				"completion_tokens": 15,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider("test-key", "gpt-4", WithOpenAIBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "What's the weather?"},
	}, []ToolSchema{
		{Name: "get_weather", Description: "Get weather", InputSchema: map[string]any{"city": "string"}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(resp.ToolUses) != 1 {
		t.Fatalf("ToolUses = %d, want 1", len(resp.ToolUses))
	}
	if resp.ToolUses[0].Name != "get_weather" {
		t.Fatalf("Tool name = %q, want %q", resp.ToolUses[0].Name, "get_weather")
	}
	if resp.ToolUses[0].ID != "call_123" {
		t.Fatalf("Tool ID = %q, want %q", resp.ToolUses[0].ID, "call_123")
	}
	city, ok := resp.ToolUses[0].Input["city"].(string)
	if !ok || city != "London" {
		t.Fatalf("city = %v, want %q", resp.ToolUses[0].Input["city"], "London")
	}
}

func TestOpenAIProvider_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	p := NewOpenAIProvider("bad-key", "gpt-4", WithOpenAIBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOpenAIProvider_GenerateWithSystemMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		msgs, _ := reqBody["messages"].([]any)
		sysFound := false
		for _, m := range msgs {
			msg := m.(map[string]any)
			if msg["role"] == "system" {
				sysFound = true
			}
		}
		if !sysFound {
			t.Fatal("system message not found in request")
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "OK", "tool_calls": []any{}}},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider("test-key", "gpt-4", WithOpenAIBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleSystem, Content: "You are helpful."},
		{Role: RoleUser, Content: "Do something"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestOpenAIProvider_GenerateWithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := reqBody["tools"]; !ok {
			t.Fatal("tools not found in request")
		}
		tools, _ := reqBody["tools"].([]any)
		if len(tools) != 2 {
			t.Fatalf("len(tools) = %d, want 2", len(tools))
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "", "tool_calls": []any{}}},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider("test-key", "gpt-4", WithOpenAIBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Use tools"},
	}, []ToolSchema{
		{Name: "tool_a", Description: "Tool A", InputSchema: map[string]any{}},
		{Name: "tool_b", Description: "Tool B", InputSchema: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestOpenAIProvider_RequestFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if reqBody["model"] != "gpt-4" {
			t.Fatalf("model = %v, want gpt-4", reqBody["model"])
		}

		msgs, _ := reqBody["messages"].([]any)
		if len(msgs) != 2 {
			t.Fatalf("len(messages) = %d, want 2", len(msgs))
		}

		userMsg := msgs[0].(map[string]any)
		if userMsg["role"] != "user" {
			t.Fatalf("role = %q, want user", userMsg["role"])
		}

		toolMsg := msgs[1].(map[string]any)
		if toolMsg["role"] != "tool" {
			t.Fatalf("role = %q, want tool", toolMsg["role"])
		}
		if toolMsg["tool_call_id"] != "tc_1" {
			t.Fatalf("tool_call_id = %q, want tc_1", toolMsg["tool_call_id"])
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "done", "tool_calls": []any{}}},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider("test-key", "gpt-4", WithOpenAIBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "do it"},
		{Role: RoleTool, Content: "result", ToolCallID: "tc_1", ToolName: "tool_x"},
	}, []ToolSchema{
		{Name: "tool_x", Description: "Tool X", InputSchema: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
}
