// SPDX-License-Identifier: MIT
package observe

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestCounter(t *testing.T) {
	c := globalMetrics.Counter("test_counter")
	c.Inc()
	c.Add(5)
	if v := c.Value(); v != 6 {
		t.Fatalf("expected 6, got %d", v)
	}
}

func TestGauge(t *testing.T) {
	g := globalMetrics.Gauge("test_gauge")
	g.Set(42.5)
	if v := g.Value(); v != 42.5 {
		t.Fatalf("expected 42.5, got %f", v)
	}
	g.Add(-10)
	if v := g.Value(); v != 32.5 {
		t.Fatalf("expected 32.5, got %f", v)
	}
}

func TestHistogram(t *testing.T) {
	h := NewMetricsRegistry().Histogram("test_hist", []float64{1, 5, 10})
	h.Observe(0.5)
	h.Observe(3)
	h.Observe(7)
	h.Observe(15)

	snap := h.counts
	if snap[0] != 1 || snap[1] != 1 || snap[2] != 1 || snap[3] != 1 {
		t.Fatalf("unexpected bucket counts: %v", snap)
	}
}

func TestMetricsSnapshot(t *testing.T) {
	r := NewMetricsRegistry()
	r.Counter("c").Add(10)
	r.Gauge("g").Set(3.14)
	snap := r.Snapshot()

	if snap.Counters["c"] != 10 {
		t.Fatalf("expected 10, got %d", snap.Counters["c"])
	}
	if snap.Gauges["g"] != 3.14 {
		t.Fatalf("expected 3.14, got %f", snap.Gauges["g"])
	}
}

func TestLogger_Levels(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("test", LevelWarn)
	l.SetOutput(&buf)

	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")

	output := buf.String()
	if strings.Contains(output, "debug") || strings.Contains(output, "info") {
		t.Fatal("debug/info should not appear at WARN level")
	}
	if !strings.Contains(output, "WARN") {
		t.Fatal("expected WARN message")
	}
	if !strings.Contains(output, "ERROR") {
		t.Fatal("expected ERROR message")
	}
}

func TestLogger_Format(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("dxrk", LevelInfo)
	l.SetOutput(&buf)

	l.Info("hello %s", "world")

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Fatal("expected INFO")
	}
	if !strings.Contains(output, "hello world") {
		t.Fatal("expected 'hello world'")
	}
}

func TestLogger_WithFields(t *testing.T) {
	l := NewLogger("test", LevelInfo)
	lf := l.WithFields(LogFields{"agent": "coder", "model": "claude"})

	if !strings.Contains(lf.prefix, "agent=coder") {
		t.Fatal("expected agent=coder in prefix")
	}
	if !strings.Contains(lf.prefix, "model=claude") {
		t.Fatal("expected model=claude in prefix")
	}
}

func TestGlobalMetrics(t *testing.T) {
	MetricRequests.Inc()
	MetricErrors.Add(3)
	MetricTokensIn.Add(1000)
	MetricCostTotal.Set(0.42)

	snap := globalMetrics.Snapshot()
	if snap.Counters["requests_total"] < 1 {
		t.Fatal("expected requests_total >= 1")
	}
	if snap.Gauges["cost_total"] != 0.42 {
		t.Fatalf("expected 0.42, got %f", snap.Gauges["cost_total"])
	}
}

func TestSpanHelpers(t *testing.T) {
	start := time.Now()
	_ = start

	_ = StrAttr("key", "val")
	_ = IntAttr("count", 42)
	_ = BoolAttr("flag", true)
	_ = StrSliceAttr("items", []string{"a", "b"})
	_ = FormatProviderSpanName("openai", "gpt-4o")
	_ = FormatStageSpanName("main", "coder")
}

func TestMetricsString(t *testing.T) {
	snap := MetricsSnapshot{
		Counters: map[string]int64{"req": 100},
		Gauges:   map[string]float64{"cost": 0.42},
	}
	out := snap.String()
	if !strings.Contains(out, "req: 100") {
		t.Fatal("expected counter in output")
	}
}
