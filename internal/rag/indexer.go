// SPDX-License-Identifier: MIT
package rag

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Indexer struct {
	store      *VectorStore
	embedder   Embedder
	cfg        ChunkConfig
	rootDir    string
	ignoreDirs map[string]bool

	mu         sync.RWMutex
	lastRun    time.Time
	fileHashes map[string]string
}

type IndexStats struct {
	FilesScanned  int    `json:"files_scanned"`
	FilesIndexed  int    `json:"files_indexed"`
	ChunksCreated int    `json:"chunks_created"`
	DurationMs    int64  `json:"duration_ms"`
	TotalVectors  int    `json:"total_vectors"`
	LastRun       string `json:"last_run"`
}

func NewIndexer(store *VectorStore, embedder Embedder, rootDir string, cfg ChunkConfig) *Indexer {
	dirs := make(map[string]bool)
	for k, v := range DefaultIgnoreDirs {
		dirs[k] = v
	}

	return &Indexer{
		store:      store,
		embedder:   embedder,
		cfg:        cfg,
		rootDir:    rootDir,
		ignoreDirs: dirs,
		fileHashes: make(map[string]string),
	}
}

func (idx *Indexer) AddIgnoreDir(dir string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.ignoreDirs[dir] = true
}

func (idx *Indexer) Index() (*IndexStats, error) {
	start := time.Now()

	var files []string
	err := filepath.Walk(idx.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if idx.shouldIgnoreDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsCodeFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}

	scanned := len(files)
	var indexed int
	var totalChunks int

	for _, file := range files {
		hash, err := fileHash(file)
		if err != nil {
			continue
		}

		idx.mu.RLock()
		oldHash := idx.fileHashes[file]
		idx.mu.RUnlock()

		if oldHash == hash {
			continue
		}

		chunks, err := ChunkFile(file, idx.cfg)
		if err != nil || len(chunks) == 0 {
			continue
		}

		texts := make([]string, len(chunks))
		for i, c := range chunks {
			texts[i] = c.Text
		}

		embeddings, err := idx.embedder.Embed(texts)
		if err != nil {
			continue
		}

		var records []VectorRecord
		for i, chunk := range chunks {
			if i < len(embeddings) {
				id := fmt.Sprintf("%x", sha256.Sum256([]byte(chunk.FilePath+":"+fmt.Sprintf("%d", chunk.StartLine))))
				records = append(records, VectorRecord{
					ID:        id,
					Chunk:     chunk,
					Embedding: embeddings[i],
				})
			}
		}

		idx.store.Insert(records)
		idx.mu.Lock()
		idx.fileHashes[file] = hash
		idx.mu.Unlock()

		indexed++
		totalChunks += len(records)
	}

	idx.mu.Lock()
	idx.lastRun = time.Now()
	idx.mu.Unlock()

	count, _ := idx.store.Stats()

	return &IndexStats{
		FilesScanned:  scanned,
		FilesIndexed:  indexed,
		ChunksCreated: totalChunks,
		DurationMs:    time.Since(start).Milliseconds(),
		TotalVectors:  count,
		LastRun:       idx.lastRun.Format(time.RFC3339),
	}, nil
}

func (idx *Indexer) Query(text string, maxResults int) ([]SearchResult, error) {
	embeddings, err := idx.embedder.Embed([]string{text})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, nil
	}

	return idx.store.Search(embeddings[0], maxResults), nil
}

func (idx *Indexer) LastRun() time.Time {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.lastRun
}

func (idx *Indexer) TotalVectors() int {
	n, _ := idx.store.Stats()
	return n
}

func (idx *Indexer) shouldIgnoreDir(name string) bool {
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.ignoreDirs[name]
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
