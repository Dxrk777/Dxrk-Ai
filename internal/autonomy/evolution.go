// SPDX-License-Identifier: MIT
package autonomy

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Genome struct {
	ID          string    `json:"id"`
	Prompt      string    `json:"prompt"`
	Strategy    string    `json:"strategy"`
	Score       float64   `json:"score"`
	Generations int       `json:"generations"`
	Created     time.Time `json:"created"`
	ParentID    string    `json:"parent_id,omitempty"`
	Mutations   int       `json:"mutations"`
}

type EvolutionEngine struct {
	mu            sync.Mutex
	path          string
	population    []Genome
	generation    int
	mutationRate  float64
	crossoverRate float64
	learner       *Learner
	metrics       *IQMetrics
}

func NewEvolutionEngine(path string, learner *Learner, metrics *IQMetrics) *EvolutionEngine {
	e := &EvolutionEngine{
		path:          path,
		population:    make([]Genome, 0, 50),
		mutationRate:  0.3,
		crossoverRate: 0.7,
		learner:       learner,
		metrics:       metrics,
	}
	e.load()
	if len(e.population) == 0 {
		e.seed()
	}
	return e
}

func (e *EvolutionEngine) seed() {
	basePrompts := []string{
		"You are Dxrk, an autonomous AI coding agent. Be concise and correct.",
		"You are Dxrk, an expert software engineer. Write clean, tested code.",
		"You are Dxrk, a senior developer. Prioritize readability and maintainability.",
		"You are Dxrk. Think step by step, verify your work, and fix errors.",
		"You are Dxrk, a coding agent. Write idiomatic Go code with proper error handling.",
	}

	for i, p := range basePrompts {
		e.population = append(e.population, Genome{
			ID:        fmt.Sprintf("gen-base-%d", i),
			Prompt:    p,
			Strategy:  "base",
			Score:     50.0,
			Created:   time.Now(),
			Mutations: 0,
		})
	}
	e.save()
}

func (e *EvolutionEngine) Evolve() *Genome {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.generation++

	e.evaluatePopulation()
	e.selectElite()

	children := e.reproduce()

	e.population = append(e.population, children...)
	if len(e.population) > 50 {
		e.population = e.population[:50]
	}

	e.mutate()

	e.save()

	if e.metrics != nil {
		e.metrics.RecordEvolution()
	}

	best := e.bestGenome()
	return &best
}

func (e *EvolutionEngine) evaluatePopulation() []Genome {
	sorted := make([]Genome, len(e.population))
	copy(sorted, e.population)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})
	return sorted
}

func (e *EvolutionEngine) selectElite() {
	sort.Slice(e.population, func(i, j int) bool {
		return e.population[i].Score > e.population[j].Score
	})
	keep := len(e.population) / 2
	if keep < 2 {
		keep = 2
	}
	e.population = e.population[:keep]
}

func (e *EvolutionEngine) reproduce() []Genome {
	var children []Genome
	pop := e.population

	for len(children) < 10 {
		p1 := e.tournamentSelect(pop)
		p2 := e.tournamentSelect(pop)
		if p1.ID == p2.ID {
			continue
		}

		child := Genome{
			ID:          fmt.Sprintf("gen-%d-%x", e.generation, randBytes(4)),
			Generations: e.generation,
			Created:     time.Now(),
			ParentID:    p1.ID,
			Strategy:    fmt.Sprintf("cross_%s_%s", p1.ID[:8], p2.ID[:8]),
		}

		child.Prompt = e.crossover(p1.Prompt, p2.Prompt)
		child.Score = (p1.Score + p2.Score) / 2
		children = append(children, child)
	}

	return children
}

func (e *EvolutionEngine) tournamentSelect(pop []Genome) Genome {
	k := 3
	if k > len(pop) {
		k = len(pop)
	}
	best := Genome{Score: -1}
	for i := 0; i < k; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(pop))))
		candidate := pop[idx.Int64()]
		if candidate.Score > best.Score {
			best = candidate
		}
	}
	return best
}

func (e *EvolutionEngine) crossover(a, b string) string {
	runesA := []rune(a)
	runesB := []rune(b)
	if len(runesA) < 3 || len(runesB) < 3 {
		return a
	}

	roll, _ := rand.Int(rand.Reader, big.NewInt(100))
	if roll.Int64() > int64(e.crossoverRate*100) {
		return a
	}

	point1, _ := rand.Int(rand.Reader, big.NewInt(int64(len(runesA)-2)))
	point2, _ := rand.Int(rand.Reader, big.NewInt(int64(len(runesB)-2)))
	point1 = big.NewInt(int64(int(point1.Int64()) + 1))
	point2 = big.NewInt(int64(int(point2.Int64()) + 1))

	result := string(runesA[:point1.Int64()]) + string(runesB[point2.Int64():])
	if len([]rune(result)) < 10 {
		return a
	}
	return result
}

func (e *EvolutionEngine) mutate() {
	for i := range e.population {
		runes := []rune(e.population[i].Prompt)
		roll, _ := rand.Int(rand.Reader, big.NewInt(100))
		if roll.Int64() > int64(e.mutationRate*100) {
			continue
		}

		if len(runes) < 5 {
			continue
		}

		pos, _ := rand.Int(rand.Reader, big.NewInt(int64(len(runes))))
		mutagen, _ := rand.Int(rand.Reader, big.NewInt(3))

		switch mutagen.Int64() {
		case 0:
			runes = append(runes[:pos.Int64()], runes[pos.Int64()+1:]...)
		case 1:
			insert := []rune("!.,?")
			ch, _ := rand.Int(rand.Reader, big.NewInt(int64(len(insert))))
			runes = append(runes[:pos.Int64()], append([]rune{insert[ch.Int64()]}, runes[pos.Int64():]...)...)
		case 2:
			if pos.Int64()+1 < int64(len(runes)) {
				runes[pos.Int64()], runes[pos.Int64()+1] = runes[pos.Int64()+1], runes[pos.Int64()]
			}
		}

		e.population[i].Prompt = string(runes)
		e.population[i].Mutations++
	}
}

func (e *EvolutionEngine) BestGenome() Genome {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.bestGenome()
}

func (e *EvolutionEngine) bestGenome() Genome {
	best := e.population[0]
	for _, g := range e.population {
		if g.Score > best.Score {
			best = g
		}
	}
	return best
}

func (e *EvolutionEngine) UpdateScore(genomeID string, score float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.population {
		if e.population[i].ID == genomeID {
			e.population[i].Score = (e.population[i].Score + score) / 2
			return
		}
	}
}

func (e *EvolutionEngine) Population() []Genome {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Genome, len(e.population))
	copy(out, e.population)
	return out
}

func (e *EvolutionEngine) load() {
	data, err := os.ReadFile(e.path)
	if err != nil {
		return
	}
	var store struct {
		Generation int      `json:"generation"`
		Population []Genome `json:"population"`
	}
	if err := json.Unmarshal(data, &store); err != nil {
		log.Printf("[evolution] failed to unmarshal store: %v", err)
		return
	}
	e.generation = store.Generation
	e.population = store.Population
}

func (e *EvolutionEngine) save() {
	store := struct {
		Generation int      `json:"generation"`
		Population []Genome `json:"population"`
	}{
		Generation: e.generation,
		Population: e.population,
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		log.Printf("[evolution] failed to marshal store: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(e.path), 0o750); err != nil {
		log.Printf("[evolution] failed to create dir: %v", err)
		return
	}
	if err := os.WriteFile(e.path, data, 0o600); err != nil {
		log.Printf("[evolution] failed to write file: %v", err)
	}
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}
