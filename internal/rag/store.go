// SPDX-License-Identifier: MIT
package rag

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type VectorRecord struct {
	ID        string    `json:"id"`
	Chunk     Chunk     `json:"chunk"`
	Embedding []float64 `json:"embedding"`
}

type SearchResult struct {
	Record VectorRecord `json:"record"`
	Score  float64      `json:"score"`
}

type VectorStore struct {
	mu      sync.RWMutex
	records map[string]VectorRecord
	dims    int
	persist string
}

func NewVectorStore(dimensions int, persistPath string) *VectorStore {
	store := &VectorStore{
		records: make(map[string]VectorRecord),
		dims:    dimensions,
		persist: persistPath,
	}
	if persistPath != "" {
		store.load()
	}
	return store
}

func (s *VectorStore) Insert(records []VectorRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range records {
		if r.ID == "" {
			continue
		}
		s.records[r.ID] = r
	}

	if s.persist != "" {
		s.save()
	}
}

func (s *VectorStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.records, id)
	if s.persist != "" {
		s.save()
	}
}

func (s *VectorStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = make(map[string]VectorRecord)
	if s.persist != "" {
		s.save()
	}
}

func (s *VectorStore) Search(query []float64, maxResults int) []SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.records) == 0 || maxResults <= 0 {
		return nil
	}

	type scored struct {
		record VectorRecord
		score  float64
	}

	var results []scored
	for _, rec := range s.records {
		score := cosineSimilarity(query, rec.Embedding)
		results = append(results, scored{record: rec, score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	out := make([]SearchResult, len(results))
	for i, r := range results {
		out[i] = SearchResult{Record: r.record, Score: r.score}
	}
	return out
}

func (s *VectorStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

func (s *VectorStore) Stats() (count int, dims int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records), s.dims
}

func (s *VectorStore) load() {
	data, err := os.ReadFile(s.persist)
	if err != nil {
		return
	}

	var records []VectorRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return
	}

	s.records = make(map[string]VectorRecord, len(records))
	for _, r := range records {
		s.records[r.ID] = r
	}
}

func (s *VectorStore) save() {
	s.mu.Unlock()
	defer s.mu.Lock()

	records := make([]VectorRecord, 0, len(s.records))
	for _, r := range s.records {
		records = append(records, r)
	}

	data, err := json.Marshal(records)
	if err != nil {
		return
	}

	if err := os.MkdirAll(filepath.Dir(s.persist), 0o750); err != nil {
		log.Printf("[rag] failed to create dir: %v", err)
		return
	}
	if err := os.WriteFile(s.persist, data, 0o600); err != nil {
		log.Printf("[rag] failed to write file: %v", err)
	}
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
