// SPDX-License-Identifier: MIT
package rag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Embedder interface {
	Embed(texts []string) ([][]float64, error)
	Model() string
	Dimensions() int
}

type OpenAIEmbedder struct {
	apiKey     string
	model      string
	baseURL    string
	dimensions int
	client     *http.Client
}

type openAIEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func NewOpenAIEmbedder(apiKey, model, baseURL string) *OpenAIEmbedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := strings.TrimRight(baseURL, "/")

	return &OpenAIEmbedder{
		apiKey:     apiKey,
		model:      model,
		baseURL:    url,
		dimensions: 1536,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    60 * time.Second,
				DisableCompression: false,
			},
		},
	}
}

func (e *OpenAIEmbedder) Embed(texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Batch: OpenAI supports up to 2048 inputs per request
	const maxBatch = 256
	var all [][]float64

	for i := 0; i < len(texts); i += maxBatch {
		end := i + maxBatch
		if end > len(texts) {
			end = len(texts)
		}

		batch, err := e.embedBatch(texts[i:end])
		if err != nil {
			return nil, fmt.Errorf("batch %d: %w", i/maxBatch, err)
		}
		all = append(all, batch...)
	}

	return all, nil
}

func (e *OpenAIEmbedder) embedBatch(texts []string) ([][]float64, error) {
	body := openAIEmbedRequest{
		Model: e.model,
		Input: texts,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(raw))
	}

	var result openAIEmbedResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	// Sort by index
	ordered := make([][]float64, len(result.Data))
	for _, d := range result.Data {
		if d.Index >= 0 && d.Index < len(ordered) {
			ordered[d.Index] = d.Embedding
		}
	}

	return ordered, nil
}

func (e *OpenAIEmbedder) Model() string   { return e.model }
func (e *OpenAIEmbedder) Dimensions() int { return e.dimensions }
