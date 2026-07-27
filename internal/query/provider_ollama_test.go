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

func TestOllamaProvider_GenerateTextOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/chat") {
			t.Fatalf("path = %q, want /api/chat", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": "Hello from Ollama!",
			},
			"done_reason":       "stop",
			"eval_count":        5,
			"prompt_eval_count": 10,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOllamaProvider("llama3", WithOllamaBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Say hi"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "Hello from Ollama!" {
		t.Fatalf("Text = %q, want %q", resp.Text, "Hello from Ollama!")
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

func TestOllamaProvider_GenerateWithToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": "Let me check...",
				"tool_calls": []map[string]any{
					{
						"function": map[string]any{
							"name":      "get_weather",
							"arguments": map[string]any{"city": "Berlin"},
						},
					},
				},
			},
			"done_reason":       "stop",
			"eval_count":        15,
			"prompt_eval_count": 20,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOllamaProvider("llama3", WithOllamaBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Weather in Berlin?"},
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
	city, ok := resp.ToolUses[0].Input["city"].(string)
	if !ok || city != "Berlin" {
		t.Fatalf("city = %v, want %q", resp.ToolUses[0].Input["city"], "Berlin")
	}
	if resp.Text != "Let me check..." {
		t.Fatalf("Text = %q, want %q", resp.Text, "Let me check...")
	}
}

func TestOllamaProvider_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	p := NewOllamaProvider("llama3", WithOllamaBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOllamaProvider_GenerateWithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := reqBody["tools"]; !ok {
			t.Fatal("tools not found in request")
		}
		if reqBody["model"] != "llama3" {
			t.Fatalf("model = %v, want llama3", reqBody["model"])
		}
		if reqBody["stream"] != false {
			t.Fatal("stream != false")
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"message":           map[string]any{"role": "assistant", "content": "OK"},
			"done_reason":       "stop",
			"eval_count":        5,
			"prompt_eval_count": 5,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOllamaProvider("llama3", WithOllamaBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Use tools"},
	}, []ToolSchema{
		{Name: "tool_a", Description: "Tool A", InputSchema: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestOllamaProvider_EmptyToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"message":           map[string]any{"role": "assistant", "content": "No tools needed"},
			"done_reason":       "stop",
			"eval_count":        3,
			"prompt_eval_count": 3,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOllamaProvider("llama3", WithOllamaBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Say something"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "No tools needed" {
		t.Fatalf("Text = %q, want %q", resp.Text, "No tools needed")
	}
	if len(resp.ToolUses) != 0 {
		t.Fatalf("ToolUses = %d, want 0", len(resp.ToolUses))
	}
}
