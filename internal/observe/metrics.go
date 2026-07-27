// SPDX-License-Identifier: MIT
package observe

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type Counter struct {
	mu    sync.Mutex
	name  string
	count int64
}

type Gauge struct {
	mu    sync.Mutex
	name  string
	value float64
}

type Histogram struct {
	mu      sync.Mutex
	name    string
	buckets []float64
	counts  []int64
	total   int64
	sum     float64
}

type MetricsRegistry struct {
	mu         sync.Mutex
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

var globalMetrics = NewMetricsRegistry()

func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

func (r *MetricsRegistry) Counter(name string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &Counter{name: name}
	r.counters[name] = c
	return c
}

func (r *MetricsRegistry) Gauge(name string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g := &Gauge{name: name}
	r.gauges[name] = g
	return g
}

func (r *MetricsRegistry) Histogram(name string, buckets []float64) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histograms[name]; ok {
		return h
	}
	if buckets == nil {
		buckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000}
	}
	h := &Histogram{
		name:    name,
		buckets: buckets,
		counts:  make([]int64, len(buckets)+1),
	}
	r.histograms[name] = h
	return h
}

func (r *MetricsRegistry) Snapshot() MetricsSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	snap := MetricsSnapshot{
		Counters:   make(map[string]int64),
		Gauges:     make(map[string]float64),
		Histograms: make(map[string]HistogramSnap),
	}

	for n, c := range r.counters {
		c.mu.Lock()
		snap.Counters[n] = c.count
		c.mu.Unlock()
	}

	for n, g := range r.gauges {
		g.mu.Lock()
		snap.Gauges[n] = g.value
		g.mu.Unlock()
	}

	for n, h := range r.histograms {
		h.mu.Lock()
		snap.Histograms[n] = HistogramSnap{
			Buckets: append([]float64{}, h.buckets...),
			Counts:  append([]int64{}, h.counts...),
			Total:   h.total,
			Sum:     h.sum,
		}
		h.mu.Unlock()
	}

	return snap
}

func (c *Counter) Add(n int64) {
	c.mu.Lock()
	c.count += n
	c.mu.Unlock()
}

func (c *Counter) Inc() { c.Add(1) }

func (c *Counter) Value() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func (g *Gauge) Set(v float64) {
	g.mu.Lock()
	g.value = v
	g.mu.Unlock()
}

func (g *Gauge) Add(v float64) {
	g.mu.Lock()
	g.value += v
	g.mu.Unlock()
}

func (g *Gauge) Value() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.value
}

func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.total++
	h.sum += v

	for i, b := range h.buckets {
		if v <= b {
			h.counts[i]++
			return
		}
	}
	h.counts[len(h.counts)-1]++
}

type MetricsSnapshot struct {
	Counters   map[string]int64         `json:"counters"`
	Gauges     map[string]float64       `json:"gauges"`
	Histograms map[string]HistogramSnap `json:"histograms"`
}

type HistogramSnap struct {
	Buckets []float64 `json:"buckets"`
	Counts  []int64   `json:"counts"`
	Total   int64     `json:"total"`
	Sum     float64   `json:"sum"`
}

func (s MetricsSnapshot) String() string {
	var b strings.Builder
	for n, v := range s.Counters {
		fmt.Fprintf(&b, "counter %s: %d\n", n, v)
	}
	for n, v := range s.Gauges {
		fmt.Fprintf(&b, "gauge %s: %.2f\n", n, v)
	}
	for n, h := range s.Histograms {
		fmt.Fprintf(&b, "histogram %s: total=%d sum=%.2f\n", n, h.Total, h.Sum)
	}
	return b.String()
}

var (
	MetricRequests     = globalMetrics.Counter("requests_total")
	MetricErrors       = globalMetrics.Counter("errors_total")
	MetricTokensIn     = globalMetrics.Counter("tokens_input_total")
	MetricTokensOut    = globalMetrics.Counter("tokens_output_total")
	MetricCostTotal    = globalMetrics.Gauge("cost_total")
	MetricActiveAgents = globalMetrics.Gauge("active_agents")
	MetricQueueDepth   = globalMetrics.Gauge("queue_depth")
	MetricCacheHits    = globalMetrics.Counter("cache_hits_total")
	MetricCacheMisses  = globalMetrics.Counter("cache_misses_total")
	MetricLatency      = globalMetrics.Histogram("latency_ms", nil)
	MetricRAGVectors   = globalMetrics.Gauge("rag_vectors")
	MetricIQScore      = globalMetrics.Gauge("iq_score")

	LatencyMs = func(d time.Duration) float64 {
		return float64(d.Milliseconds())
	}
)
