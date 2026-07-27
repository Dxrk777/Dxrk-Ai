// SPDX-License-Identifier: MIT
package compress

import (
	"strings"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	c := New()
	if c.maxTokens != 128000 {
		t.Fatalf("maxTokens = %d, want 128000", c.maxTokens)
	}
}

func TestCompress_UnderBudget(t *testing.T) {
	c := New(WithMaxTokens(1000))
	contents := []Content{
		{ID: "1", Text: "hello", Size: 5},
	}
	result, changed := c.Compress(contents)
	if changed {
		t.Fatal("expected no compression when under budget")
	}
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
}

func TestCompress_Snip(t *testing.T) {
	c := New(WithMaxTokens(50), WithCompressionPct(50), WithStrategy(StrategySnip))
	contents := []Content{
		{ID: "old1", Text: "old content that should be removed because it exceeds the budget by far", Size: 80, CreatedAt: time.Now().Add(-1 * time.Hour)},
		{ID: "old2", Text: "another old chunk of data that is taking up way too much space for no reason", Size: 85, CreatedAt: time.Now().Add(-30 * time.Minute)},
		{ID: "new1", Text: "new content", Size: 11, CreatedAt: time.Now()},
	}
	result, changed := c.Compress(contents)
	if !changed {
		t.Fatal("expected compression")
	}
	if len(result) == 0 {
		t.Fatal("expected at least one content after snip")
	}
	if result[len(result)-1].ID != "new1" {
		t.Fatalf("expected newest content kept, got %q", result[len(result)-1].ID)
	}
}

func TestCompress_TrimHead(t *testing.T) {
	c := New(WithMaxTokens(50), WithCompressionPct(50), WithStrategy(StrategyTrimHead))
	long := ""
	for range 200 {
		long += "a"
	}
	contents := []Content{
		{ID: "1", Text: long, Size: 200},
	}
	result, changed := c.Compress(contents)
	if !changed {
		t.Fatal("expected compression")
	}
	if len(result[0].Text) >= 200 {
		t.Fatal("expected text to be trimmed")
	}
}

func TestCompress_Summarize(t *testing.T) {
	c := New(WithMaxTokens(50), WithCompressionPct(50), WithStrategy(StrategySummary))
	long := "this is a very long piece of content that should be summarized to fit within the budget"
	contents := []Content{
		{ID: "1", Text: long, Size: len(long)},
	}
	result, changed := c.Compress(contents)
	if !changed {
		t.Fatal("expected compression")
	}
	if !stringsSuffix(result[0].Text, "...") {
		t.Fatal("expected summary to end with ...")
	}
}

func stringsSuffix(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}

func TestTokenCount(t *testing.T) {
	// English: ~4 chars per token
	if n := TokenCount("hello world"); n != 2 {
		t.Fatalf("TokenCount = %d, want 2", n)
	}
}

func TestBudget(t *testing.T) {
	b := NewBudget(1000)
	b.Add(100)
	if b.Remaining() != 900 {
		t.Fatalf("Remaining = %d, want 900", b.Remaining())
	}
	if b.NeedsCompression() {
		t.Fatal("NeedsCompression should be false at 10% usage")
	}
	b.Add(800)
	if !b.NeedsCompression() {
		t.Fatal("NeedsCompression should be true at 90% usage")
	}
	b.Add(100)
	if !b.IsNearLimit() {
		t.Fatal("IsNearLimit should be true when remaining <= 500")
	}
}

func TestBudgetReset(t *testing.T) {
	b := NewBudget(1000)
	b.Add(500)
	b.Reset()
	if b.Remaining() != 1000 {
		t.Fatalf("after Reset Remaining = %d, want 1000", b.Remaining())
	}
}

func TestSnapshotter(t *testing.T) {
	s := NewSnapshotter(10*time.Second, 3)
	snap := s.Record("1", []Content{{ID: "c1", Text: "hello", Size: 5}})
	if snap.ID != "1" {
		t.Fatalf("ID = %q, want %q", snap.ID, "1")
	}
	recent := s.Recent()
	if len(recent) != 1 {
		t.Fatalf("Recent = %d, want 1", len(recent))
	}
}

func TestSnapshotter_MaxCount(t *testing.T) {
	s := NewSnapshotter(time.Hour, 2)
	s.Record("1", []Content{{ID: "c1", Text: "a", Size: 1}})
	s.Record("2", []Content{{ID: "c2", Text: "b", Size: 1}})
	s.Record("3", []Content{{ID: "c3", Text: "c", Size: 1}})
	recent := s.Recent()
	if len(recent) != 2 {
		t.Fatalf("Recent = %d, want 2 (maxCount)", len(recent))
	}
	if recent[0].ID != "2" {
		t.Fatalf("oldest should be 2, got %q", recent[0].ID)
	}
}

func TestTrim(t *testing.T) {
	text := "hello world this is a test"
	result := Trim(text, 10)
	if len(result.Content) > 10 {
		t.Fatalf("Trimmed content too long: %d", len(result.Content))
	}
	if result.TrimmedBytes <= 0 {
		t.Fatal("expected some bytes trimmed")
	}
}

func TestTrimUnderLimit(t *testing.T) {
	result := Trim("hello", 100)
	if result.TrimmedBytes != 0 {
		t.Fatalf("TrimmedBytes = %d, want 0", result.TrimmedBytes)
	}
	if result.Strategy != "none" {
		t.Fatalf("Strategy = %q, want %q", result.Strategy, "none")
	}
}

func TestTrimToTokens(t *testing.T) {
	text := "a b c d e f g h i j k l m n o p q r s t u v w x y z"
	result := TrimToTokens(text, 3) // 3 tokens ≈ 12 bytes
	if len(result.Content) > 12 {
		t.Fatalf("TrimToTokens too long: %d bytes", len(result.Content))
	}
}

func TestCombineContext(t *testing.T) {
	contents := []Content{
		{ID: "1", Role: "user", Text: "hello"},
		{ID: "2", Role: "assistant", Text: "world"},
	}
	result := CombineContext(contents, "")
	if !strings.Contains(result, "<USER>") {
		t.Fatal("expected <USER> tag")
	}
	if !strings.Contains(result, "</ASSISTANT>") {
		t.Fatal("expected </ASSISTANT> tag")
	}
}

func TestSnapshotter_SaveLoad(t *testing.T) {
	s := NewSnapshotter(time.Hour, 10)
	s.Record("s1", []Content{{ID: "c1", Text: "snapshot data", Size: 13}})

	path := t.TempDir() + "/snapshots.json"
	if err := s.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	s2 := NewSnapshotter(time.Hour, 10)
	n, err := s2.LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("loaded %d snapshots, want 1", n)
	}
	recent := s2.Recent()
	if len(recent) != 1 {
		t.Fatalf("Recent = %d, want 1", len(recent))
	}
	if recent[0].ID != "s1" {
		t.Fatalf("ID = %q, want %q", recent[0].ID, "s1")
	}
}

func TestSnapshotter_LoadMissingFile(t *testing.T) {
	s := NewSnapshotter(time.Hour, 5)
	n, err := s.LoadFromFile("/nonexistent/path.json")
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if n != 0 {
		t.Fatalf("loaded %d snapshots, want 0", n)
	}
}

func TestMergeSnapshots(t *testing.T) {
	s1 := Snapshot{ID: "s1", Content: []Content{{ID: "c1", Text: "first", Size: 5}}}
	s2 := Snapshot{ID: "s2", Content: []Content{{ID: "c2", Text: "second", Size: 6}}}
	s3 := Snapshot{ID: "s3", Content: []Content{{ID: "c1", Text: "first-dup", Size: 9}}}

	result := MergeSnapshots([]Snapshot{s1, s2}, 100)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	// Duplicate c1 should be deduplicated (only last wins)
	result = MergeSnapshots([]Snapshot{s1, s2, s3}, 100)
	if len(result) != 2 {
		t.Fatalf("len with dup = %d, want 2", len(result))
	}
}
