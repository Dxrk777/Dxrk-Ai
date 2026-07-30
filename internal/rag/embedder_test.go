// SPDX-License-Identifier: MIT
package rag

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewOpenAIEmbedder_Defaults(t *testing.T) {
	e := NewOpenAIEmbedder("", "", "")
	if e.model != "text-embedding-3-small" {
		t.Errorf("expected default model, got %s", e.model)
	}
	if e.baseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default base URL, got %s", e.baseURL)
	}
	if e.dimensions != 1536 {
		t.Errorf("expected 1536 dimensions, got %d", e.dimensions)
	}
	if e.apiKey != "" {
		t.Errorf("expected empty api key, got %s", e.apiKey)
	}
}

func TestNewOpenAIEmbedder_CustomBaseURL(t *testing.T) {
	e := NewOpenAIEmbedder("key", "custom-model", "https://custom.example.com/v1/")
	if e.model != "custom-model" {
		t.Errorf("expected custom-model, got %s", e.model)
	}
	if e.baseURL != "https://custom.example.com/v1" {
		t.Errorf("expected trimmed base URL, got %s", e.baseURL)
	}
}

func TestOpenAIEmbedder_Model(t *testing.T) {
	e := NewOpenAIEmbedder("key", "my-model", "")
	if e.Model() != "my-model" {
		t.Errorf("expected my-model, got %s", e.Model())
	}
}

func TestOpenAIEmbedder_Embed_Empty(t *testing.T) {
	e := NewOpenAIEmbedder("test-key", "text-embedding-3-small", "")
	result, err := e.Embed(nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil for nil input")
	}

	result, err = e.Embed([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil for empty input")
	}
}

func TestOpenAIEmbedder_Embed_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/embeddings" {
			t.Errorf("expected /embeddings, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		var req openAIEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "test-model" {
			t.Errorf("expected test-model, got %s", req.Model)
		}
		if len(req.Input) != 2 || req.Input[0] != "hello" || req.Input[1] != "world" {
			t.Errorf("unexpected input: %v", req.Input)
		}

		resp := openAIEmbedResponse{
			Data: []struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float64{0.1, 0.2, 0.3}, Index: 0},
				{Embedding: []float64{0.4, 0.5, 0.6}, Index: 1},
			},
			Usage: struct {
				PromptTokens int `json:"prompt_tokens"`
				TotalTokens  int `json:"total_tokens"`
			}{PromptTokens: 4, TotalTokens: 4},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e := NewOpenAIEmbedder("test-key", "test-model", server.URL)
	embeddings, err := e.Embed([]string{"hello", "world"})
	if err != nil {
		t.Fatal(err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embeddings))
	}
	if len(embeddings[0]) != 3 || embeddings[0][0] != 0.1 {
		t.Errorf("unexpected first embedding: %v", embeddings[0])
	}
	if len(embeddings[1]) != 3 || embeddings[1][0] != 0.4 {
		t.Errorf("unexpected second embedding: %v", embeddings[1])
	}
}

func TestOpenAIEmbedder_Embed_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	e := NewOpenAIEmbedder("key", "test-model", server.URL)
	_, err := e.Embed([]string{"test"})
	if err == nil {
		t.Error("expected error when server is closed")
	}
}

func TestOpenAIEmbedder_Embed_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	e := NewOpenAIEmbedder("bad-key", "test-model", server.URL)
	_, err := e.Embed([]string{"test"})
	if err == nil {
		t.Fatal("expected error for API error")
	}
}

func TestOpenAIEmbedder_Embed_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid`))
	}))
	defer server.Close()

	e := NewOpenAIEmbedder("key", "test-model", server.URL)
	_, err := e.Embed([]string{"test"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestOpenAIEmbedder_Embed_OutOfOrderIndices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIEmbedResponse{
			Data: []struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float64{0.3, 0.4}, Index: 1},
				{Embedding: []float64{0.1, 0.2}, Index: 0},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e := NewOpenAIEmbedder("key", "test-model", server.URL)
	embeddings, err := e.Embed([]string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embeddings))
	}
	if embeddings[0][0] != 0.1 {
		t.Errorf("expected first embedding to have 0.1 at index 0, got %v", embeddings[0])
	}
	if embeddings[1][0] != 0.3 {
		t.Errorf("expected second embedding to have 0.3 at index 0, got %v", embeddings[1])
	}
}

func TestOpenAIEmbedder_Embed_InvalidIndex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIEmbedResponse{
			Data: []struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float64{0.1, 0.2}, Index: -1},
				{Embedding: []float64{0.3, 0.4}, Index: 5},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e := NewOpenAIEmbedder("key", "test-model", server.URL)
	embeddings, err := e.Embed([]string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embeddings))
	}
	// Invalid indices should result in nil entries at those positions
	for i, emb := range embeddings {
		if emb != nil {
			t.Errorf("expected nil embedding at index %d due to invalid index, got %v", i, emb)
		}
	}
}

func TestOpenAIEmbedder_Embed_Batching(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req openAIEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}

		data := make([]struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		}, len(req.Input))
		for i := range req.Input {
			data[i] = struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				Embedding: []float64{float64(i), 0},
				Index:     i,
			}
		}

		resp := openAIEmbedResponse{Data: data}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e := NewOpenAIEmbedder("key", "test-model", server.URL)

	// Use 300 texts to trigger batching (maxBatch = 256)
	texts := make([]string, 300)
	for i := range texts {
		texts[i] = "text"
	}

	embeddings, err := e.Embed(texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(embeddings) != 300 {
		t.Fatalf("expected 300 embeddings, got %d", len(embeddings))
	}
	if callCount != 2 {
		t.Errorf("expected 2 batch calls (256+44), got %d", callCount)
	}
}
