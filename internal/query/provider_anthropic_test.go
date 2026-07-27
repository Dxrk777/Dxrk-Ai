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

func TestAnthropicProvider_GenerateTextOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/messages") {
			t.Fatalf("path = %q, want /messages", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("x-api-key = %q, want test-key", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("anthropic-version = %q, want 2023-06-01", r.Header.Get("anthropic-version"))
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "Hello from Claude!"},
			},
			"usage": map[string]any{
				"input_tokens":  10,
				"output_tokens": 5,
			},
			"stop_reason": "end_turn",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-3-opus-20240229", WithAnthropicBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Say hi"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "Hello from Claude!" {
		t.Fatalf("Text = %q, want %q", resp.Text, "Hello from Claude!")
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

func TestAnthropicProvider_GenerateWithToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type contentBlock struct {
			Type  string          `json:"type"`
			Text  string          `json:"text,omitempty"`
			ID    string          `json:"id,omitempty"`
			Name  string          `json:"name,omitempty"`
			Input json.RawMessage `json:"input,omitempty"`
		}

		w.WriteHeader(http.StatusOK)
		inputRaw, _ := json.Marshal(map[string]string{"city": "Paris"})
		resp := map[string]any{
			"content": []contentBlock{
				{Type: "text", Text: "Let me check the weather..."},
				{Type: "tool_use", ID: "tu_123", Name: "get_weather", Input: inputRaw},
			},
			"usage": map[string]any{
				"input_tokens":  20,
				"output_tokens": 15,
			},
			"stop_reason": "tool_use",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-3-opus-20240229", WithAnthropicBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Weather in Paris?"},
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
	if resp.ToolUses[0].ID != "tu_123" {
		t.Fatalf("Tool ID = %q, want %q", resp.ToolUses[0].ID, "tu_123")
	}
	city, ok := resp.ToolUses[0].Input["city"].(string)
	if !ok || city != "Paris" {
		t.Fatalf("city = %v, want %q", resp.ToolUses[0].Input["city"], "Paris")
	}
}

func TestAnthropicProvider_GenerateWithSystemMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := reqBody["system"]; !ok {
			t.Fatal("system not found in request")
		}
		if reqBody["system"] != "You are helpful." {
			t.Fatalf("system = %q, want %q", reqBody["system"], "You are helpful.")
		}
		msgs, _ := reqBody["messages"].([]any)
		if len(msgs) != 1 {
			t.Fatalf("len(messages) = %d, want 1", len(msgs))
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"content":     []map[string]any{{"type": "text", "text": "OK"}},
			"usage":       map[string]any{"input_tokens": 5, "output_tokens": 5},
			"stop_reason": "end_turn",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-3-opus-20240229", WithAnthropicBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleSystem, Content: "You are helpful."},
		{Role: RoleUser, Content: "Do something"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestAnthropicProvider_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": "rate limited"}`))
	}))
	defer server.Close()

	p := NewAnthropicProvider("bad-key", "claude-3-opus-20240229", WithAnthropicBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAnthropicProvider_ToolMessageFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		msgs, _ := reqBody["messages"].([]any)
		lastMsg := msgs[len(msgs)-1].(map[string]any)
		content, _ := lastMsg["content"].([]any)
		if len(content) == 0 {
			t.Fatal("no content in tool message")
		}
		contentBlock := content[0].(map[string]any)
		if contentBlock["type"] != "tool_result" {
			t.Fatalf("type = %q, want tool_result", contentBlock["type"])
		}
		if contentBlock["tool_use_id"] != "tu_1" {
			t.Fatalf("tool_use_id = %q, want tu_1", contentBlock["tool_use_id"])
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"content":     []map[string]any{{"type": "text", "text": "Done"}},
			"usage":       map[string]any{"input_tokens": 5, "output_tokens": 5},
			"stop_reason": "end_turn",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-3-opus-20240229", WithAnthropicBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "do it"},
		{Role: RoleTool, Content: "result", ToolCallID: "tu_1", ToolName: "tool_x"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestAnthropicProvider_GenerateWithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := reqBody["tools"]; !ok {
			t.Fatal("tools not found in request")
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"content":     []map[string]any{{"type": "text", "text": "OK"}},
			"usage":       map[string]any{"input_tokens": 5, "output_tokens": 5},
			"stop_reason": "end_turn",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-3-opus-20240229", WithAnthropicBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Use tools"},
	}, []ToolSchema{
		{Name: "tool_a", Description: "Tool A", InputSchema: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestAnthropicProvider_EmptyToolInputParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		inputRaw := json.RawMessage(`{}`)
		resp := map[string]any{
			"content": []map[string]any{
				{
					"type":  "tool_use",
					"id":    "tu_1",
					"name":  "greet",
					"input": &inputRaw,
				},
			},
			"usage":       map[string]any{"input_tokens": 5, "output_tokens": 5},
			"stop_reason": "tool_use",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-3-opus-20240229", WithAnthropicBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "greet"},
	}, []ToolSchema{
		{Name: "greet", Description: "Greet", InputSchema: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(resp.ToolUses) != 1 {
		t.Fatalf("ToolUses = %d, want 1", len(resp.ToolUses))
	}
	if resp.ToolUses[0].Name != "greet" {
		t.Fatalf("Tool name = %q, want %q", resp.ToolUses[0].Name, "greet")
	}
}
