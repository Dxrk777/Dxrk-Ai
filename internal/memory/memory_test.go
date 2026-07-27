// SPDX-License-Identifier: MIT
package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestMemory_StoreAndRetrieve(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "mem.json")
	m := NewAgentMemory(path, 100, nil, nil)

	entry := MemoryEntry{
		ID:         "test-1",
		Type:       MemorySemantic,
		Content:    "How to write a Fibonacci function in Go",
		ProjectID:  "proj-1",
		SessionID:  "sess-1",
		Importance: 0.8,
	}
	if err := m.Store(context.Background(), entry); err != nil {
		t.Fatal(err)
	}

	retrieved, ok := m.Retrieve(context.Background(), "test-1")
	if !ok {
		t.Fatal("expected to find entry")
	}
	if retrieved.Content != entry.Content {
		t.Fatalf("content mismatch: %q vs %q", retrieved.Content, entry.Content)
	}
	if retrieved.AccessCount != 1 {
		t.Fatalf("expected access count 1, got %d", retrieved.AccessCount)
	}
}

func TestMemory_Search(t *testing.T) {
	m := NewAgentMemory("", 100, nil, nil)

	_ = m.Store(context.Background(), MemoryEntry{ID: "1", Type: MemorySemantic, Content: "Fibonacci in Go", ProjectID: "p1", SessionID: "s1", Importance: 0.9})
	_ = m.Store(context.Background(), MemoryEntry{ID: "2", Type: MemoryEpisodic, Content: "Fixed bug in auth", ProjectID: "p1", SessionID: "s1", Importance: 0.7})
	_ = m.Store(context.Background(), MemoryEntry{ID: "3", Type: MemoryProcedural, Content: "Deploy to k8s", ProjectID: "p2", SessionID: "s2", Importance: 0.8})

	results := m.Search(context.Background(), "p1", "Fibonacci", MemorySemantic, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "1" {
		t.Fatalf("expected ID 1, got %s", results[0].ID)
	}

	all := m.Search(context.Background(), "p1", "", 0, 10)
	if len(all) != 2 {
		t.Fatalf("expected 2 results for project p1, got %d", len(all))
	}
}

func TestMemory_Eviction(t *testing.T) {
	m := NewAgentMemory("", 3, nil, nil)

	for i := 0; i < 5; i++ {
		_ = m.Store(context.Background(), MemoryEntry{
			ID:         fmt.Sprintf("entry-%d", i),
			Type:       MemorySemantic,
			Content:    fmt.Sprintf("content %d", i),
			ProjectID:  "p1",
			SessionID:  "s1",
			Importance: float64(i),
		})
	}

	if len(m.entries) > 3 {
		t.Fatalf("expected max 3 entries, got %d", len(m.entries))
	}
}

func TestMemory_GetByProject(t *testing.T) {
	m := NewAgentMemory("", 100, nil, nil)
	_ = m.Store(context.Background(), MemoryEntry{ID: "1", Type: MemorySemantic, Content: "a", ProjectID: "p1"})
	_ = m.Store(context.Background(), MemoryEntry{ID: "2", Type: MemorySemantic, Content: "b", ProjectID: "p2"})

	p1 := m.GetByProject("p1")
	if len(p1) != 1 || p1[0].ID != "1" {
		t.Fatal("expected 1 entry for p1")
	}
}

func TestMemory_GetByType(t *testing.T) {
	m := NewAgentMemory("", 100, nil, nil)
	_ = m.Store(context.Background(), MemoryEntry{ID: "1", Type: MemorySemantic, Content: "a"})
	_ = m.Store(context.Background(), MemoryEntry{ID: "2", Type: MemoryEpisodic, Content: "b"})

	sem := m.GetByType(MemorySemantic)
	if len(sem) != 1 || sem[0].ID != "1" {
		t.Fatal("expected 1 semantic memory")
	}
}

func TestMemory_Delete(t *testing.T) {
	m := NewAgentMemory("", 100, nil, nil)
	_ = m.Store(context.Background(), MemoryEntry{ID: "del-1", Type: MemorySemantic, Content: "delete me"})

	if err := m.Delete("del-1"); err != nil {
		t.Fatal(err)
	}
	_, ok := m.Retrieve(context.Background(), "del-1")
	if ok {
		t.Fatal("expected entry to be deleted")
	}
}

func TestMemory_Stats(t *testing.T) {
	m := NewAgentMemory("", 100, nil, nil)
	_ = m.Store(context.Background(), MemoryEntry{ID: "1", Type: MemorySemantic, Content: "a", ProjectID: "p1"})
	_ = m.Store(context.Background(), MemoryEntry{ID: "2", Type: MemoryEpisodic, Content: "b", ProjectID: "p1"})
	_ = m.Store(context.Background(), MemoryEntry{ID: "3", Type: MemorySemantic, Content: "c", ProjectID: "p2"})

	stats := m.Stats()
	if stats.TotalEntries != 3 {
		t.Fatalf("expected 3 entries, got %d", stats.TotalEntries)
	}
	if stats.ByProject != 2 {
		t.Fatalf("expected 2 projects, got %d", stats.ByProject)
	}
	if stats.ByType[MemorySemantic] != 2 {
		t.Fatalf("expected 2 semantic, got %d", stats.ByType[MemorySemantic])
	}
	if stats.ByType[MemoryEpisodic] != 1 {
		t.Fatalf("expected 1 episodic, got %d", stats.ByType[MemoryEpisodic])
	}
}
