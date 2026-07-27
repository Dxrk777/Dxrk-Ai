// SPDX-License-Identifier: MIT
package rag

import (
	"sync"
)

// RAG ties together the indexer, store, and embedder for codebase search.
type RAG struct {
	Indexer  *Indexer
	Store    *VectorStore
	Embedder Embedder
	mu       sync.RWMutex
	enabled  bool
}

type Config struct {
	Enabled        bool
	EmbeddingModel string
	ChunkSize      int
	ChunkOverlap   int
	MaxResults     int
	APIKey         string
	BaseURL        string
	RootDir        string
	PersistPath    string
}

func New(cfg Config) (*RAG, error) {
	chunkCfg := ChunkConfig{
		ChunkSize:    cfg.ChunkSize,
		ChunkOverlap: cfg.ChunkOverlap,
	}
	if chunkCfg.ChunkSize <= 0 {
		chunkCfg.ChunkSize = 512
	}
	if chunkCfg.ChunkOverlap <= 0 {
		chunkCfg.ChunkOverlap = 64
	}

	model := cfg.EmbeddingModel
	if model == "" {
		model = "text-embedding-3-small"
	}

	embedder := NewOpenAIEmbedder(cfg.APIKey, model, cfg.BaseURL)
	store := NewVectorStore(embedder.Dimensions(), cfg.PersistPath)
	indexer := NewIndexer(store, embedder, cfg.RootDir, chunkCfg)

	return &RAG{
		Indexer:  indexer,
		Store:    store,
		Embedder: embedder,
		enabled:  cfg.Enabled,
	}, nil
}

func (r *RAG) IsEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled
}

func (r *RAG) Query(query string, maxResults int) ([]SearchResult, error) {
	if !r.IsEnabled() {
		return nil, nil
	}
	if maxResults <= 0 {
		maxResults = 5
	}
	return r.Indexer.Query(query, maxResults)
}

// RAGContextKey is the context key for the RAG instance.
type RAGContextKey struct{}
