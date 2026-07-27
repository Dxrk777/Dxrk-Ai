// SPDX-License-Identifier: MIT
package autonomy

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type IQSnapshot struct {
	Timestamp       time.Time `json:"timestamp"`
	SuccessRate     float64   `json:"success_rate"`
	ErrorReduction  float64   `json:"error_reduction"`
	TokenEfficiency float64   `json:"token_efficiency"`
	LatencyP50      float64   `json:"latency_p50"`
	TestPassRate    float64   `json:"test_pass_rate"`
	AutoFixRate     float64   `json:"auto_fix_rate"`
	EvolutionScore  float64   `json:"evolution_score"`
	OverallIQ       float64   `json:"overall_iq"`
	TurnsCompleted  int       `json:"turns_completed"`
	ErrorsFixed     int       `json:"errors_fixed"`
}

type IQMetrics struct {
	mu   sync.Mutex
	path string

	successCount int
	failureCount int
	totalTokens  int
	totalLatency float64
	latencies    []float64
	testPasses   int
	testFails    int
	autoFixes    int
	autoFixFails int
	errorsFixed  int
	evolutions   int

	history    []IQSnapshot
	startTime  time.Time
	turnsCount int
}

func NewIQMetrics(path string) *IQMetrics {
	m := &IQMetrics{
		path:      path,
		startTime: time.Now(),
	}
	m.load()
	return m
}

func (m *IQMetrics) load() {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &m.history); err != nil {
		log.Printf("[metrics] failed to unmarshal history: %v", err)
	}
}

func (m *IQMetrics) save() {
	data, err := json.MarshalIndent(m.history, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o750); err != nil {
		log.Printf("[metrics] failed to create dir: %v", err)
		return
	}
	if err := os.WriteFile(m.path, data, 0o600); err != nil {
		log.Printf("[metrics] failed to write file: %v", err)
	}
}

func (m *IQMetrics) RecordTurn(success bool, tokens int, latencyMs float64) {
	m.mu.Lock()

	m.turnsCount++
	if success {
		m.successCount++
	} else {
		m.failureCount++
	}
	m.totalTokens += tokens
	m.totalLatency += latencyMs
	m.latencies = append(m.latencies, latencyMs)

	shouldSnap := m.turnsCount%10 == 0
	m.mu.Unlock()

	if shouldSnap {
		m.snapshot()
	}
}

func (m *IQMetrics) RecordTestResult(pass bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pass {
		m.testPasses++
	} else {
		m.testFails++
	}
}

func (m *IQMetrics) RecordAutoFix(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if success {
		m.autoFixes++
		m.errorsFixed++
	} else {
		m.autoFixFails++
	}
}

func (m *IQMetrics) RecordEvolution() {
	m.mu.Lock()
	m.evolutions++
	m.mu.Unlock()
}

func (m *IQMetrics) Score() IQSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	total := m.successCount + m.failureCount
	successRate := 0.0
	if total > 0 {
		successRate = float64(m.successCount) / float64(total) * 100
	}

	totalTests := m.testPasses + m.testFails
	testPassRate := 0.0
	if totalTests > 0 {
		testPassRate = float64(m.testPasses) / float64(totalTests) * 100
	}

	autoFixTotal := m.autoFixes + m.autoFixFails
	autoFixRate := 0.0
	if autoFixTotal > 0 {
		autoFixRate = float64(m.autoFixes) / float64(autoFixTotal) * 100
	}

	tokenEff := 100.0
	if m.turnsCount > 0 && m.totalTokens > 0 {
		tokenEff = float64(m.successCount) / float64(m.totalTokens) * 10000
	}

	latencyP50 := 0.0
	if len(m.latencies) > 0 {
		sorted := make([]float64, len(m.latencies))
		copy(sorted, m.latencies)
		sort.Float64s(sorted)
		latencyP50 = sorted[len(sorted)/2]
	}

	errReduction := m.calcErrorReduction()

	evolutionScore := float64(m.evolutions) * 5.0
	if evolutionScore > 100 {
		evolutionScore = 100
	}

	overallIQ := (successRate*0.25 +
		errReduction*0.20 +
		tokenEff*0.15 +
		(100-latencyP50/10)*0.10 +
		testPassRate*0.15 +
		autoFixRate*0.15)

	if overallIQ > 100 {
		overallIQ = 100
	}
	if overallIQ < 0 {
		overallIQ = 0
	}

	return IQSnapshot{
		Timestamp:       time.Now(),
		SuccessRate:     math.Round(successRate*100) / 100,
		ErrorReduction:  math.Round(errReduction*100) / 100,
		TokenEfficiency: math.Round(tokenEff*100) / 100,
		LatencyP50:      math.Round(latencyP50*100) / 100,
		TestPassRate:    math.Round(testPassRate*100) / 100,
		AutoFixRate:     math.Round(autoFixRate*100) / 100,
		EvolutionScore:  math.Round(evolutionScore*100) / 100,
		OverallIQ:       math.Round(overallIQ*100) / 100,
		TurnsCompleted:  m.turnsCount,
		ErrorsFixed:     m.errorsFixed,
	}
}

func (m *IQMetrics) calcErrorReduction() float64 {
	if len(m.history) < 2 {
		return 50.0
	}
	first := m.history[0]
	last := m.history[len(m.history)-1]
	if first.ErrorsFixed == 0 {
		return 50.0
	}
	reduction := (float64(last.ErrorsFixed-first.ErrorsFixed) / float64(first.ErrorsFixed)) * 100
	if reduction > 100 {
		return 100
	}
	return reduction
}

func (m *IQMetrics) snapshot() {
	s := m.Score()
	m.history = append(m.history, s)
	if len(m.history) > 100 {
		m.history = m.history[len(m.history)-100:]
	}
	m.save()
}
