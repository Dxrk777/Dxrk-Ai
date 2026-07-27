// SPDX-License-Identifier: MIT
package devex

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/log"
)

func TestNewManager(t *testing.T) {
	m := New(nil, nil)
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
}

func TestManager_TrackEvent(t *testing.T) {
	logger := log.NewNop()
	analytics := NewAnalytics(nil)
	m := New(logger, analytics)

	m.TrackEvent("test_event", map[string]string{"key": "value"})

	count := analytics.Count(context.Background(), "test_event")
	if count != 1 {
		t.Errorf("Count() = %d, want 1", count)
	}
}

func TestManager_TrackEvent_Multiple(t *testing.T) {
	logger := log.NewNop()
	analytics := NewAnalytics(nil)
	m := New(logger, analytics)

	for i := 0; i < 5; i++ {
		m.TrackEvent("multi", map[string]string{"i": itoa(i)})
	}

	count := analytics.Count(context.Background(), "multi")
	if count != 5 {
		t.Errorf("Count() = %d, want 5", count)
	}
}

func TestManager_StartTimer(t *testing.T) {
	m := New(log.NewNop(), nil)
	timer := m.StartTimer("test_op")
	if timer == nil {
		t.Fatal("expected non-nil Timer")
	}
	if timer.name != "test_op" {
		t.Errorf("timer.name = %q, want %q", timer.name, "test_op")
	}
}

func TestTimer_Stop(t *testing.T) {
	m := New(log.NewNop(), nil)
	timer := m.StartTimer("timed_op")
	time.Sleep(time.Millisecond)
	duration := timer.Stop()

	if duration <= 0 {
		t.Errorf("Stop() duration = %v, want > 0", duration)
	}
}

func TestTimer_Stop_Idempotent(t *testing.T) {
	timer := &Timer{name: "idempotent", start: time.Now()}
	first := timer.Stop()
	second := timer.Stop()

	if first != second {
		t.Errorf("second Stop() = %v, want %v (same as first)", second, first)
	}
}

func TestTimer_String(t *testing.T) {
	timer := &Timer{name: "bench", start: time.Now().Add(-time.Second)}
	timer.Stop()
	s := timer.String()
	if !strings.Contains(s, "bench") {
		t.Errorf("String() = %q, want it to contain 'bench'", s)
	}
}

func TestShowTip(t *testing.T) {
	m := New(log.NewNop(), nil)
	ctx := context.Background()

	tip := m.ShowTip(ctx)
	if tip == "" {
		t.Fatal("expected non-empty tip")
	}

	if !strings.Contains(tip, "dxrk") {
		t.Errorf("tip = %q, expected it to mention 'dxrk'", tip)
	}
}

func TestShowTip_Tracked(t *testing.T) {
	analytics := NewAnalytics(nil)
	m := New(log.NewNop(), analytics)
	ctx := context.Background()

	m.ShowTip(ctx)
	m.ShowTip(ctx)
	m.ShowTip(ctx)

	count := analytics.Count(ctx, "tip_shown")
	if count != 3 {
		t.Errorf("Count(tip_shown) = %d, want 3", count)
	}
}

func TestNewAnalytics(t *testing.T) {
	a := NewAnalytics(nil)
	if a == nil {
		t.Fatal("expected non-nil Analytics")
	}
}

func TestNewAnalytics_WithStorage(t *testing.T) {
	store := map[string]int{"preloaded": 10}
	a := NewAnalytics(store)
	if a.Count(context.Background(), "preloaded") != 10 {
		t.Errorf("Count(preloaded) = %d, want 10", a.Count(context.Background(), "preloaded"))
	}
}

func TestAnalytics_Increment_Count(t *testing.T) {
	ctx := context.Background()
	a := NewAnalytics(nil)

	tests := []struct {
		feature string
		times   int
		want    int
	}{
		{"sync.run", 3, 3},
		{"backup.create", 1, 1},
		{"install.run", 5, 5},
		{"never.used", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.feature, func(t *testing.T) {
			for i := 0; i < tt.times; i++ {
				a.Increment(ctx, tt.feature)
			}
			if got := a.Count(ctx, tt.feature); got != tt.want {
				t.Errorf("Count(%q) = %d, want %d", tt.feature, got, tt.want)
			}
		})
	}
}

func TestAnalytics_Snapshot(t *testing.T) {
	ctx := context.Background()
	a := NewAnalytics(nil)

	a.Increment(ctx, "a")
	a.Increment(ctx, "b")
	a.Increment(ctx, "b")
	a.Increment(ctx, "c")
	a.Increment(ctx, "c")
	a.Increment(ctx, "c")

	snap := a.Snapshot()
	if snap["a"] != 1 {
		t.Errorf("snapshot[a] = %d, want 1", snap["a"])
	}
	if snap["b"] != 2 {
		t.Errorf("snapshot[b] = %d, want 2", snap["b"])
	}
	if snap["c"] != 3 {
		t.Errorf("snapshot[c] = %d, want 3", snap["c"])
	}
}

func TestAnalytics_Reset(t *testing.T) {
	ctx := context.Background()
	a := NewAnalytics(nil)

	a.Increment(ctx, "a")
	a.Increment(ctx, "b")
	a.Reset()

	if count := a.Count(ctx, "a"); count != 0 {
		t.Errorf("Count(a) after reset = %d, want 0", count)
	}
	if count := a.Count(ctx, "b"); count != 0 {
		t.Errorf("Count(b) after reset = %d, want 0", count)
	}
}

func TestAnalytics_ConcurrentSafety(t *testing.T) {
	ctx := context.Background()
	a := NewAnalytics(nil)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			a.Increment(ctx, "concurrent")
			_ = a.Count(ctx, "concurrent")
			_ = a.Snapshot()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if count := a.Count(ctx, "concurrent"); count != 10 {
		t.Errorf("Count(concurrent) = %d, want 10", count)
	}
}

func TestNewSuggester(t *testing.T) {
	s := NewSuggester()
	if s == nil {
		t.Fatal("expected non-nil Suggester")
	}
}

func TestSuggester_Suggest_Empty(t *testing.T) {
	s := NewSuggester()
	ctx := context.Background()

	results := s.Suggest(ctx, "")
	if len(results) == 0 {
		t.Fatal("expected top suggestions for empty input")
	}
}

func TestSuggester_Suggest_Filter(t *testing.T) {
	s := NewSuggester()
	ctx := context.Background()

	results := s.Suggest(ctx, "sdd")
	if len(results) == 0 {
		t.Fatal("expected suggestions matching 'sdd'")
	}

	for _, r := range results {
		if !strings.Contains(strings.ToLower(r.Command), "sdd") && !strings.Contains(strings.ToLower(r.Description), "sdd") {
			t.Errorf("result %+v does not match 'sdd'", r)
		}
	}
}

func TestSuggester_Suggest_Backup(t *testing.T) {
	s := NewSuggester()
	ctx := context.Background()

	results := s.Suggest(ctx, "backup")
	if len(results) == 0 {
		t.Fatal("expected suggestions matching 'backup'")
	}

	hasCreate := false
	hasList := false
	for _, r := range results {
		if strings.Contains(r.Command, "backup create") {
			hasCreate = true
		}
		if strings.Contains(r.Command, "backup list") {
			hasList = true
		}
	}
	if !hasCreate {
		t.Error("expected 'backup create' in suggestions")
	}
	if !hasList {
		t.Error("expected 'backup list' in suggestions")
	}
}

func TestSuggester_Suggest_Model(t *testing.T) {
	s := NewSuggester()
	ctx := context.Background()

	results := s.Suggest(ctx, "model")
	if len(results) == 0 {
		t.Fatal("expected suggestions matching 'model'")
	}

	hasList := false
	hasSwitch := false
	for _, r := range results {
		if strings.Contains(r.Command, "model list") {
			hasList = true
		}
		if strings.Contains(r.Command, "model switch") {
			hasSwitch = true
		}
	}
	if !hasList {
		t.Error("expected 'model list' in suggestions")
	}
	if !hasSwitch {
		t.Error("expected 'model switch' in suggestions")
	}
}

func TestSuggester_Suggest_Priority(t *testing.T) {
	s := NewSuggester()
	ctx := context.Background()

	results := s.Suggest(ctx, "")
	if len(results) == 0 {
		t.Fatal("expected top suggestions")
	}

	for i := 1; i < len(results); i++ {
		if results[i-1].Priority < results[i].Priority {
			t.Errorf("suggestions not sorted by priority descending: %d < %d at index %d",
				results[i-1].Priority, results[i].Priority, i)
		}
	}
}

func TestNewBanner(t *testing.T) {
	b := NewBanner("1.0.0")
	if b == nil {
		t.Fatal("expected non-nil Banner")
	}
	if b.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", b.Version, "1.0.0")
	}
}

func TestBanner_Render(t *testing.T) {
	b := NewBanner("2.0.0")
	rendered := b.Render(context.Background())

	if !strings.Contains(rendered, "Dxrk") {
		t.Errorf("Render() missing 'Dxrk', got: %s", rendered)
	}
	if !strings.Contains(rendered, "Dxrk") {
		t.Errorf("Render() missing 'Dxrk', got: %s", rendered)
	}
	if !strings.Contains(rendered, "2.0.0") {
		t.Errorf("Render() missing version '2.0.0', got: %s", rendered)
	}
}

func TestBanner_ShortBanner(t *testing.T) {
	b := NewBanner("0.5.0")
	short := b.ShortBanner()

	if !strings.Contains(short, "Dxrk") {
		t.Errorf("ShortBanner() missing 'Dxrk', got: %s", short)
	}
	if !strings.Contains(short, "Dxrk") {
		t.Errorf("ShortBanner() missing 'Dxrk', got: %s", short)
	}
	if !strings.Contains(short, "0.5.0") {
		t.Errorf("ShortBanner() missing version '0.5.0', got: %s", short)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
