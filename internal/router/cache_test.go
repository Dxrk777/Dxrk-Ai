// SPDX-License-Identifier: MIT
package router

import (
	"context"
	"testing"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/query"
)

func TestSemanticCache_SetAndGet(t *testing.T) {
	c := NewSemanticCache()
	c.Set("hello world", queryResponse{Text: "hi there", InputTokens: 10, OutputTokens: 5})

	resp, ok := c.Get("hello world")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if resp.Text != "hi there" {
		t.Fatalf("expected 'hi there', got %q", resp.Text)
	}
}

func TestSemanticCache_Miss(t *testing.T) {
	c := NewSemanticCache()
	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestSemanticCache_TTL(t *testing.T) {
	c := NewSemanticCache(WithTTL(1 * time.Nanosecond))
	c.Set("key", queryResponse{Text: "val"})
	time.Sleep(1 * time.Microsecond)

	_, ok := c.Get("key")
	if ok {
		t.Fatal("expected expired entry")
	}
}

func TestSemanticCache_Eviction(t *testing.T) {
	c := NewSemanticCache(WithMaxSize(3))
	for i := 0; i < 5; i++ {
		key := keyForIndex(i)
		c.Set(key, queryResponse{Text: valForIndex(i)})
	}

	_, ok := c.Get(keyForIndex(0))
	if ok {
		t.Fatal("expected oldest entry to be evicted")
	}
}

func TestSemanticCache_UpdateExisting(t *testing.T) {
	c := NewSemanticCache()
	c.Set("key", queryResponse{Text: "old"})
	c.Set("key", queryResponse{Text: "new"})

	resp, ok := c.Get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if resp.Text != "new" {
		t.Fatalf("expected 'new', got %q", resp.Text)
	}
}

func TestSemanticCache_Invalidate(t *testing.T) {
	c := NewSemanticCache()
	c.Set("key", queryResponse{Text: "val"})
	c.Invalidate("key")

	_, ok := c.Get("key")
	if ok {
		t.Fatal("expected cache miss after invalidation")
	}
}

func TestSemanticCache_Clear(t *testing.T) {
	c := NewSemanticCache()
	c.Set("a", queryResponse{Text: "1"})
	c.Set("b", queryResponse{Text: "2"})
	c.Clear()

	if st := c.Stats(); st.Size != 0 {
		t.Fatalf("expected empty cache, got size %d", st.Size)
	}
}

func TestSemanticCache_CustomKeyFn(t *testing.T) {
	c := NewSemanticCache(WithCustomKeyFn(func(s string) string {
		if len(s) > 5 {
			return s[:5]
		}
		return s
	}))
	c.Set("hello world", queryResponse{Text: "greeting"})
	c.Set("hello there", queryResponse{Text: "also greeting"})

	if st := c.Stats(); st.Size != 1 {
		t.Fatalf("expected 1 entry (collision), got %d", st.Size)
	}
}

func TestSemanticCache_Stats(t *testing.T) {
	c := NewSemanticCache(WithMaxSize(500), WithTTL(10*time.Minute))
	c.Set("key", queryResponse{Text: "val"})
	c.Get("key")
	c.Get("key")

	st := c.Stats()
	if st.Size != 1 {
		t.Fatalf("expected size 1, got %d", st.Size)
	}
	if st.MaxSize != 500 {
		t.Fatalf("expected maxsize 500, got %d", st.MaxSize)
	}
}

func TestSemanticCache_SemanticMatching(t *testing.T) {
	c := NewSemanticCache(WithSemanticMatching(true, 0.3))
	c.Set("What is the capital of France?", queryResponse{Text: "Paris"})

	resp, ok := c.Get("What is the capital of France?")
	if !ok {
		t.Fatal("expected exact cache hit")
	}
	if resp.Text != "Paris" {
		t.Fatalf("expected 'Paris', got %q", resp.Text)
	}
}

func TestSemanticCache_SemanticNearMiss(t *testing.T) {
	c := NewSemanticCache(WithSemanticMatching(true, 0.85))
	c.Set("The quick brown fox jumps over the lazy dog", queryResponse{Text: "animal"})

	_, ok := c.Get("a fast brown fox jumped over a sleepy dog")
	if ok {
		t.Log("semantic near miss hit at threshold 0.85")
	}
}

func TestSemanticCache_SemanticThreshold(t *testing.T) {
	c := NewSemanticCache(WithSemanticMatching(true, 0.99))
	c.Set("The quick brown fox", queryResponse{Text: "animal"})

	_, ok := c.Get("jumps over lazy dog")
	if ok {
		t.Fatal("expected semantic miss with high threshold")
	}
}

func TestCachingRouter(t *testing.T) {
	r := NewRouter([]ProviderEntry{
		{Name: "test", Model: "gpt-4o-mini", Provider: &mockProvider{name: "test"}},
	})
	cache := NewSemanticCache()
	cr := NewCachingRouter(r, cache)

	ctx := context.Background()
	msgs := []query.Message{{Role: query.RoleUser, Content: "hi"}}

	resp1, err := cr.CachedGenerate(ctx, msgs, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp2, err := cr.CachedGenerate(ctx, msgs, nil)
	if err != nil {
		t.Fatal(err)
	}

	if resp1.Text != resp2.Text {
		t.Fatal("expected cached response to match")
	}
}

func keyForIndex(i int) string {
	return "k" + string(rune('0'+i)) //nolint:gosec
}

func valForIndex(i int) string {
	return "v" + string(rune('0'+i)) //nolint:gosec
}
