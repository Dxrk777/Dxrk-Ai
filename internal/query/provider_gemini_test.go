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

func TestGeminiProvider_GenerateTextOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if !strings.Contains(r.URL.String(), ":generateContent") {
			t.Fatalf("path = %q, want :generateContent", r.URL.String())
		}
		if !strings.Contains(r.URL.RawQuery, "key=test-key") {
			t.Fatalf("query = %q, want key=test-key", r.URL.RawQuery)
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": "Hello from Gemini!"},
						},
						"role": "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]any{
				"promptTokenCount":     10,
				"candidatesTokenCount": 5,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewGeminiProvider("test-key", "gemini-pro", WithGeminiBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Say hi"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "Hello from Gemini!" {
		t.Fatalf("Text = %q, want %q", resp.Text, "Hello from Gemini!")
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

func TestGeminiProvider_GenerateWithFunctionCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": "Checking..."},
							{
								"functionCall": map[string]any{
									"name": "get_weather",
									"args": map[string]any{"city": "Tokyo"},
								},
							},
						},
						"role": "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]any{
				"promptTokenCount":     20,
				"candidatesTokenCount": 15,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewGeminiProvider("test-key", "gemini-pro", WithGeminiBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Weather in Tokyo?"},
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
	if !ok || city != "Tokyo" {
		t.Fatalf("city = %v, want %q", resp.ToolUses[0].Input["city"], "Tokyo")
	}
	if resp.Text != "Checking..." {
		t.Fatalf("Text = %q, want %q", resp.Text, "Checking...")
	}
}

func TestGeminiProvider_GenerateWithSystemMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := reqBody["system_instruction"]; !ok {
			t.Fatal("system_instruction not found in request")
		}

		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{{"text": "OK"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]any{"promptTokenCount": 5, "candidatesTokenCount": 5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewGeminiProvider("test-key", "gemini-pro", WithGeminiBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleSystem, Content: "You are helpful."},
		{Role: RoleUser, Content: "Do something"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestGeminiProvider_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error": "access denied"}`))
	}))
	defer server.Close()

	p := NewGeminiProvider("bad-key", "gemini-pro", WithGeminiBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGeminiProvider_EmptyCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"candidates":    []any{},
			"usageMetadata": map[string]any{"promptTokenCount": 0, "candidatesTokenCount": 0},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewGeminiProvider("test-key", "gemini-pro", WithGeminiBaseURL(server.URL))
	resp, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "" {
		t.Fatalf("Text = %q, want empty", resp.Text)
	}
}

func TestGeminiProvider_GenerateWithTools(t *testing.T) {
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
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{{"text": "OK"}},
						"role":  "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]any{"promptTokenCount": 5, "candidatesTokenCount": 5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewGeminiProvider("test-key", "gemini-pro", WithGeminiBaseURL(server.URL))
	_, err := p.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Use tools"},
	}, []ToolSchema{
		{Name: "tool_a", Description: "Tool A", InputSchema: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
}
