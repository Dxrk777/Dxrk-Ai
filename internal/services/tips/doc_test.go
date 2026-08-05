package tips

import (
	"strings"
	"testing"
	"time"
)

func TestRegisterAndRetrieval(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10})

	engine.Register(Tip{
		ID:        "test-1",
		Category:  "test",
		Title:     "Test Tip",
		Content:   "This is a test tip.",
		Trigger:   TriggerOnStart,
		Frequency: FrequencyAlways,
		Priority:  50,
		Enabled:   true,
	})

	tips := engine.GetByCategory("test")
	if len(tips) != 1 {
		t.Fatalf("expected 1 tip, got %d", len(tips))
	}
	if tips[0].ID != "test-1" {
		t.Errorf("expected tip ID test-1, got %s", tips[0].ID)
	}
}

func TestRegisterBatch(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10})

	batch := []Tip{
		{ID: "a", Category: "cat1", Title: "A", Content: "a", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true},
		{ID: "b", Category: "cat1", Title: "B", Content: "b", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 20, Enabled: true},
		{ID: "c", Category: "cat2", Title: "C", Content: "c", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 30, Enabled: true},
	}
	engine.RegisterBatch(batch)

	stats := engine.Stats()
	if stats.Total != 3 {
		t.Errorf("expected 3 total tips, got %d", stats.Total)
	}
	if stats.ByCategory["cat1"] != 2 {
		t.Errorf("expected 2 tips in cat1, got %d", stats.ByCategory["cat1"])
	}
}

func TestGetNextPriorityOrder(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10, ShuffleTips: false})

	engine.Register(Tip{ID: "low", Category: "c", Title: "Low", Content: "low", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})
	engine.Register(Tip{ID: "high", Category: "c", Title: "High", Content: "high", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 100, Enabled: true})
	engine.Register(Tip{ID: "mid", Category: "c", Title: "Mid", Content: "mid", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 50, Enabled: true})

	next := engine.GetNext()
	if next == nil {
		t.Fatal("expected a tip, got nil")
	}
	if next.ID != "high" {
		t.Errorf("expected highest priority tip, got %s (priority %d)", next.ID, next.Priority)
	}
}

func TestGetNextDisabledSkipped(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10, ShuffleTips: false})

	engine.Register(Tip{ID: "disabled", Category: "c", Title: "Disabled", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 100, Enabled: false})
	engine.Register(Tip{ID: "enabled", Category: "c", Title: "Enabled", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})

	next := engine.GetNext()
	if next == nil {
		t.Fatal("expected a tip, got nil")
	}
	if next.ID != "enabled" {
		t.Errorf("expected enabled tip, got %s", next.ID)
	}
}

func TestGetNextRespectsMaxPerSession(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 2, ShuffleTips: false})

	engine.Register(Tip{ID: "a", Category: "c", Title: "A", Content: "a", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 30, Enabled: true})
	engine.Register(Tip{ID: "b", Category: "c", Title: "B", Content: "b", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 20, Enabled: true})
	engine.Register(Tip{ID: "c", Category: "c", Title: "C", Content: "c", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})

	// Show 2 tips
	engine.MarkShown("a")
	engine.MarkShown("b")

	next := engine.GetNext()
	if next != nil {
		t.Errorf("expected nil after max per session, got tip %s", next.ID)
	}
}

func TestFrequencyOnce(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10, MinInterval: 0})

	engine.Register(Tip{ID: "once", Category: "c", Title: "Once", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyOnce, Priority: 10, Enabled: true})

	next := engine.GetNext()
	if next == nil {
		t.Fatal("expected tip on first call")
	}

	engine.MarkShown("once")

	next = engine.GetNext()
	if next != nil {
		t.Errorf("expected nil after FrequencyOnce tip shown, got %s", next.ID)
	}
}

func TestFrequencyDaily(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10, MinInterval: 0})

	engine.Register(Tip{ID: "daily", Category: "c", Title: "Daily", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyDaily, Priority: 10, Enabled: true})

	next := engine.GetNext()
	if next == nil {
		t.Fatal("expected tip")
	}
	engine.MarkShown("daily")

	// Should not show again immediately
	next = engine.GetNext()
	if next != nil {
		t.Errorf("expected nil immediately after daily tip shown, got %s", next.ID)
	}
}

func TestFrequencyAlways(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10, MinInterval: 0})

	engine.Register(Tip{ID: "always", Category: "c", Title: "Always", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})

	for i := 0; i < 3; i++ {
		next := engine.GetNext()
		if next == nil {
			t.Fatalf("expected tip on iteration %d", i)
		}
		engine.MarkShown("always")
	}
}

func TestGetByTrigger(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10})

	engine.Register(Tip{ID: "start", Category: "c", Title: "Start", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})
	engine.Register(Tip{ID: "error", Category: "c", Title: "Error", Content: "x", Trigger: TriggerOnError, Frequency: FrequencyAlways, Priority: 10, Enabled: true})

	startTips := engine.GetByTrigger(TriggerOnStart)
	if len(startTips) != 1 || startTips[0].ID != "start" {
		t.Errorf("expected 1 start tip, got %d", len(startTips))
	}

	errorTips := engine.GetByTrigger(TriggerOnError)
	if len(errorTips) != 1 || errorTips[0].ID != "error" {
		t.Errorf("expected 1 error tip, got %d", len(errorTips))
	}
}

func TestGetByCategory(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10})

	engine.Register(Tip{ID: "a", Category: "tools", Title: "A", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})
	engine.Register(Tip{ID: "b", Category: "tools", Title: "B", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 20, Enabled: true})
	engine.Register(Tip{ID: "c", Category: "other", Title: "C", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 30, Enabled: true})

	tools := engine.GetByCategory("tools")
	if len(tools) != 2 {
		t.Errorf("expected 2 tools tips, got %d", len(tools))
	}
	// Should be sorted by priority descending
	if tools[0].Priority < tools[1].Priority {
		t.Error("expected tips sorted by priority descending")
	}
}

func TestMarkShown(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10})

	engine.Register(Tip{ID: "t", Category: "c", Title: "T", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})

	engine.MarkShown("t")

	tips := engine.GetByCategory("c")
	if tips[0].ShownCount != 1 {
		t.Errorf("expected ShownCount 1, got %d", tips[0].ShownCount)
	}
	if tips[0].LastShown.IsZero() {
		t.Error("expected LastShown to be set")
	}
}

func TestResetSession(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 1})

	engine.Register(Tip{ID: "t", Category: "c", Title: "T", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyOnce, Priority: 10, Enabled: true})

	engine.MarkShown("t")
	next := engine.GetNext()
	if next != nil {
		t.Fatal("expected nil after showing tip with max=1")
	}

	engine.ResetSession()

	next = engine.GetNext()
	if next == nil {
		t.Error("expected tip to be available after session reset")
	}
}

func TestEnableDisable(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10})

	engine.Register(Tip{ID: "t", Category: "c", Title: "T", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})

	if !engine.Disable("t") {
		t.Error("expected Disable to return true")
	}
	tips := engine.GetByCategory("c")
	if tips[0].Enabled {
		t.Error("expected tip to be disabled")
	}

	if !engine.Enable("t") {
		t.Error("expected Enable to return true")
	}
	tips = engine.GetByCategory("c")
	if !tips[0].Enabled {
		t.Error("expected tip to be enabled")
	}

	if engine.Enable("nonexistent") {
		t.Error("expected false for nonexistent tip")
	}
}

func TestEnableDisableCategory(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10})

	engine.Register(Tip{ID: "a", Category: "tools", Title: "A", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})
	engine.Register(Tip{ID: "b", Category: "tools", Title: "B", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})
	engine.Register(Tip{ID: "c", Category: "other", Title: "C", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})

	disabled := engine.DisableCategory("tools")
	if disabled != 2 {
		t.Errorf("expected 2 disabled, got %d", disabled)
	}

	tips := engine.GetByCategory("tools")
	for _, tip := range tips {
		if tip.Enabled {
			t.Errorf("expected tip %s to be disabled", tip.ID)
		}
	}

	enabled := engine.EnableCategory("tools")
	if enabled != 2 {
		t.Errorf("expected 2 enabled, got %d", enabled)
	}
}

func TestStats(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 10})

	engine.Register(Tip{ID: "a", Category: "tools", Title: "A", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})
	engine.Register(Tip{ID: "b", Category: "tools", Title: "B", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})
	engine.Register(Tip{ID: "c", Category: "other", Title: "C", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: false})

	engine.MarkShown("a")
	engine.MarkShown("a") // show twice

	stats := engine.Stats()
	if stats.Total != 3 {
		t.Errorf("expected 3 total, got %d", stats.Total)
	}
	if stats.Enabled != 2 {
		t.Errorf("expected 2 enabled, got %d", stats.Enabled)
	}
	if stats.Disabled != 1 {
		t.Errorf("expected 1 disabled, got %d", stats.Disabled)
	}
	if stats.ShownThisSession != 1 {
		t.Errorf("expected 1 shown this session, got %d", stats.ShownThisSession)
	}
	if stats.TotalShownAllTime != 2 {
		t.Errorf("expected 2 total shown, got %d", stats.TotalShownAllTime)
	}
	if stats.ByCategory["tools"] != 2 {
		t.Errorf("expected 2 in tools category, got %d", stats.ByCategory["tools"])
	}
}

func TestGetNextDisabledEngine(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: false})
	engine.Register(Tip{ID: "t", Category: "c", Title: "T", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})

	next := engine.GetNext()
	if next != nil {
		t.Error("expected nil from disabled engine")
	}
}

func TestMinInterval(t *testing.T) {
	engine := NewEngine(TipsConfig{
		Enabled:       true,
		MaxPerSession: 10,
		MinInterval:   1 * time.Hour,
	})

	engine.Register(Tip{ID: "t", Category: "c", Title: "T", Content: "x", Trigger: TriggerOnStart, Frequency: FrequencyAlways, Priority: 10, Enabled: true})

	engine.MarkShown("t")

	next := engine.GetNext()
	if next != nil {
		t.Error("expected nil due to min interval")
	}
}

// Presenter tests

func TestFormatTip(t *testing.T) {
	p := NewPresenter(TipStyle{UseEmoji: false, CompactMode: true})
	tip := Tip{Title: "Test", Content: "This is a test tip.", Category: "test"}

	result := p.FormatTip(tip)
	if !strings.Contains(result, "Test") {
		t.Error("expected title in output")
	}
	if !strings.Contains(result, "This is a test tip.") {
		t.Error("expected content in output")
	}
}

func TestFormatTipWithEmoji(t *testing.T) {
	p := NewPresenter(TipStyle{UseEmoji: true, CompactMode: true})
	tip := Tip{Title: "Test", Content: "Content", Category: "test"}

	result := p.FormatTip(tip)
	if !strings.Contains(result, "💡") {
		t.Error("expected emoji in output")
	}
}

func TestFormatTipBox(t *testing.T) {
	p := NewPresenter(TipStyle{Width: 50})
	tip := Tip{Title: "Test Tip", Content: "This is a longer test tip that wraps.", Category: "tools"}

	result := p.FormatTipBox(tip)
	if !strings.Contains(result, "┌") {
		t.Error("expected box border")
	}
	if !strings.Contains(result, "└") {
		t.Error("expected bottom border")
	}
	if !strings.Contains(result, "Test Tip") {
		t.Error("expected title in box")
	}
	if !strings.Contains(result, "[tools]") {
		t.Error("expected category in box")
	}
}

func TestFormatSummary(t *testing.T) {
	p := NewPresenter(TipStyle{})
	stats := TipsStats{
		Total:             10,
		Enabled:           8,
		Disabled:          2,
		ShownThisSession:  3,
		TotalShownAllTime: 42,
		ByCategory:        map[string]int{"tools": 5, "features": 3},
	}

	result := p.FormatSummary(stats)
	if !strings.Contains(result, "Total:") {
		t.Error("expected Total in summary")
	}
	if !strings.Contains(result, "10") {
		t.Error("expected total count")
	}
	if !strings.Contains(result, "42") {
		t.Error("expected all-time count")
	}
	if !strings.Contains(result, "tools") {
		t.Error("expected category in summary")
	}
}

func TestFormatCategoryGroup(t *testing.T) {
	p := NewPresenter(TipStyle{CompactMode: true})
	tips := []Tip{
		{Title: "Tip A", Category: "tools", Content: "a"},
		{Title: "Tip B", Category: "tools", Content: "b"},
		{Title: "Tip C", Category: "other", Content: "c"},
	}

	result := p.FormatCategoryGroup(tips)
	if !strings.Contains(result, "── tools ──") {
		t.Error("expected tools category header")
	}
	if !strings.Contains(result, "Tip A") {
		t.Error("expected Tip A in output")
	}
}

func TestFormatCategoryGroupEmpty(t *testing.T) {
	p := NewPresenter(TipStyle{})
	result := p.FormatCategoryGroup(nil)
	if result != "No tips available." {
		t.Errorf("expected empty message, got %s", result)
	}
}

func TestDefaultTips(t *testing.T) {
	defaults := DefaultTips()
	if len(defaults) < 5 {
		t.Errorf("expected at least 5 default tips, got %d", len(defaults))
	}

	ids := make(map[string]bool)
	for _, tip := range defaults {
		if tip.ID == "" {
			t.Error("tip with empty ID found")
		}
		if ids[tip.ID] {
			t.Errorf("duplicate tip ID: %s", tip.ID)
		}
		ids[tip.ID] = true
		if tip.Title == "" {
			t.Errorf("tip %s has empty title", tip.ID)
		}
	}
}

func TestEngineIntegration(t *testing.T) {
	engine := NewEngine(TipsConfig{Enabled: true, MaxPerSession: 3})
	engine.RegisterBatch(DefaultTips())

	shown := 0
	for {
		next := engine.GetNext()
		if next == nil {
			break
		}
		engine.MarkShown(next.ID)
		shown++
		if shown > 20 {
			t.Fatal("infinite loop in GetNext")
		}
	}

	if shown == 0 {
		t.Error("expected at least one tip to be shown")
	}

	stats := engine.Stats()
	if stats.ShownThisSession != shown {
		t.Errorf("expected ShownThisSession %d, got %d", shown, stats.ShownThisSession)
	}

	// Reset and verify we can get tips again
	engine.ResetSession()
	next := engine.GetNext()
	if next == nil {
		t.Error("expected tips available after session reset")
	}
}
