// SPDX-License-Identifier: MIT
package autonomy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryItem struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Category  string    `json:"category"`

	Input   string `json:"input"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	FixedBy string `json:"fixed_by,omitempty"`

	Tags      []string          `json:"tags"`
	Tokens    int               `json:"tokens"`
	LatencyMs float64           `json:"latency_ms"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type Learner struct {
	mu       sync.RWMutex
	path     string
	maxItems int
	memories []MemoryItem

	patterns      map[string]*Pattern
	errorPatterns map[string]int
}

type Pattern struct {
	Trigger     string   `json:"trigger"`
	Action      string   `json:"action"`
	SuccessRate float64  `json:"success_rate"`
	Count       int      `json:"count"`
	Tags        []string `json:"tags"`
}

func NewLearner(path string, maxItems int) *Learner {
	l := &Learner{
		path:          path,
		maxItems:      maxItems,
		memories:      make([]MemoryItem, 0, maxItems),
		patterns:      make(map[string]*Pattern),
		errorPatterns: make(map[string]int),
	}
	l.load()
	return l
}

func (l *Learner) load() {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return
	}
	var store struct {
		Memories []MemoryItem `json:"memories"`
		Patterns []Pattern    `json:"patterns"`
	}
	if err := json.Unmarshal(data, &store); err != nil {
		log.Printf("[learner] failed to unmarshal store: %v", err)
		return
	}
	l.memories = store.Memories
	for _, p := range store.Patterns {
		l.patterns[p.Trigger] = &p
	}
}

func (l *Learner) save() {
	l.mu.RLock()
	store := l.makeStore()
	l.mu.RUnlock()

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		log.Printf("[learner] failed to marshal store: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o750); err != nil {
		log.Printf("[learner] failed to create dir: %v", err)
		return
	}
	if err := os.WriteFile(l.path, data, 0o600); err != nil {
		log.Printf("[learner] failed to write file: %v", err)
	}
}

func (l *Learner) makeStore() struct {
	Memories []MemoryItem `json:"memories"`
	Patterns []Pattern    `json:"patterns"`
} {
	store := struct {
		Memories []MemoryItem `json:"memories"`
		Patterns []Pattern    `json:"patterns"`
	}{
		Memories: l.memories,
	}
	for _, p := range l.patterns {
		store.Patterns = append(store.Patterns, *p)
	}
	return store
}

func (l *Learner) Record(item MemoryItem) {
	l.mu.Lock()

	if item.ID == "" {
		item.ID = fmt.Sprintf("%x", sha256.Sum256([]byte(item.Input+item.Output+time.Now().String())))[:16]
	}
	item.Timestamp = time.Now()
	l.memories = append(l.memories, item)

	if len(l.memories) > l.maxItems {
		l.memories = l.memories[len(l.memories)-l.maxItems:]
	}

	l.learnPattern(item)

	if !item.Success && item.Error != "" {
		key := errorKey(item.Error)
		l.errorPatterns[key]++
	}

	l.mu.Unlock()

	l.save()
}

func (l *Learner) learnPattern(item MemoryItem) {
	trigger := extractTrigger(item)
	if trigger == "" {
		return
	}

	p, exists := l.patterns[trigger]
	if !exists {
		p = &Pattern{
			Trigger: trigger,
			Tags:    item.Tags,
		}
		l.patterns[trigger] = p
	}

	total := p.Count*int(p.SuccessRate+50) + 1
	if item.Success {
		total++
	}
	p.Count++
	p.SuccessRate = float64(total-50) / float64(p.Count)
	if p.SuccessRate > 1 {
		p.SuccessRate = 1
	}
	if p.SuccessRate < 0 {
		p.SuccessRate = 0
	}
	p.Action = item.Output
}

func (l *Learner) Suggest(input string) []Pattern {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var candidates []Pattern
	for _, p := range l.patterns {
		if strings.Contains(input, p.Trigger) || strings.Contains(p.Trigger, input) {
			candidates = append(candidates, *p)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].SuccessRate > candidates[j].SuccessRate
	})

	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return candidates
}

func (l *Learner) RecentMemories(n int) []MemoryItem {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if n > len(l.memories) {
		n = len(l.memories)
	}
	out := make([]MemoryItem, n)
	copy(out, l.memories[len(l.memories)-n:])
	return out
}

func (l *Learner) TopErrors(n int) []struct {
	Error string
	Count int
} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var entries []struct {
		Error string
		Count int
	}
	for err, count := range l.errorPatterns {
		entries = append(entries, struct {
			Error string
			Count int
		}{err, count})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})
	if len(entries) > n {
		entries = entries[:n]
	}
	return entries
}

func extractTrigger(item MemoryItem) string {
	if item.Input == "" {
		return ""
	}
	words := strings.Fields(item.Input)
	if len(words) > 10 {
		return strings.Join(words[:10], " ")
	}
	return item.Input
}

func errorKey(err string) string {
	err = strings.ToLower(err)
	err = strings.ReplaceAll(err, " ", "_")
	if len(err) > 64 {
		err = err[:64]
	}
	return err
}
