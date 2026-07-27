// SPDX-License-Identifier: MIT
package marketplace

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func ptr[T any](v T) *T {
	return &v
}

func TestLocalStore_RegisterGetDelete(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())

	l := Listing{
		ID:      "test-1",
		Name:    "Test Plugin",
		Version: "1.0.0",
		Type:    TypePlugin,
		Tags:    []string{"ai", "tools"},
	}
	if err := s.Register(ctx, l); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := s.Get(ctx, "test-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Test Plugin" {
		t.Fatalf("Get name = %q, want %q", got.Name, "Test Plugin")
	}
	if got.Type != TypePlugin {
		t.Fatalf("Get type = %v, want %v", got.Type, TypePlugin)
	}

	if err := s.Delete(ctx, "test-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "test-1"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestLocalStore_RegisterExisting(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())

	l := Listing{ID: "dup", Name: "v1", Version: "1.0.0"}
	if err := s.Register(ctx, l); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	l2 := Listing{ID: "dup", Name: "v2", Version: "2.0.0"}
	if err := s.Register(ctx, l2); err != nil {
		t.Fatalf("second Register: %v", err)
	}

	got, err := s.Get(ctx, "dup")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "v2" {
		t.Fatalf("name = %q, want %q", got.Name, "v2")
	}
}

func TestLocalStore_ListFilterType(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())

	for i, tc := range []struct {
		id  string
		typ Type
	}{
		{"p1", TypePlugin},
		{"p2", TypePlugin},
		{"s1", TypeSkill},
		{"t1", TypeTheme},
	} {
		s.Register(ctx, Listing{ID: tc.id, Name: tc.id, Type: tc.typ})
		_ = i
	}

	plugins, err := s.List(ctx, Filter{Type: ptr(TypePlugin)})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want 2", len(plugins))
	}

	skills, err := s.List(ctx, Filter{Type: ptr(TypeSkill)})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
}

func TestLocalStore_ListFilterTags(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())

	s.Register(ctx, Listing{ID: "a", Name: "a", Tags: []string{"ai", "ml"}})
	s.Register(ctx, Listing{ID: "b", Name: "b", Tags: []string{"ai", "tools"}})
	s.Register(ctx, Listing{ID: "c", Name: "c", Tags: []string{"ml"}})

	result, err := s.List(ctx, Filter{Tags: []string{"ai"}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("got %d, want 2", len(result))
	}

	result, err = s.List(ctx, Filter{Tags: []string{"ai", "ml"}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("got %d, want 1", len(result))
	}
}

func TestLocalStore_ListSortByRating(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())

	s.Register(ctx, Listing{ID: "low", Name: "low", Rating: 1.0})
	s.Register(ctx, Listing{ID: "high", Name: "high", Rating: 5.0})
	s.Register(ctx, Listing{ID: "mid", Name: "mid", Rating: 3.0})

	result, err := s.List(ctx, Filter{SortBy: "rating", SortOrder: "desc"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("got %d listings, want 3", len(result))
	}
	if result[0].Rating != 5.0 || result[2].Rating != 1.0 {
		t.Fatal("list not sorted by rating desc")
	}

	result, err = s.List(ctx, Filter{SortBy: "rating", SortOrder: "asc"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result[0].Rating != 1.0 || result[2].Rating != 5.0 {
		t.Fatal("list not sorted by rating asc")
	}
}

func TestLocalStore_Search(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())

	s.Register(ctx, Listing{ID: "1", Name: "Code Assistant", Description: "AI-powered coding helper", Tags: []string{"ai", "code"}})
	s.Register(ctx, Listing{ID: "2", Name: "Weather Widget", Description: "Shows current weather", Tags: []string{"weather", "ui"}})
	s.Register(ctx, Listing{ID: "3", Name: "AI Chatbot", Description: "Chat with AI", Tags: []string{"ai", "chat"}})

	results, err := s.Search(ctx, "ai")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results for 'ai', want 2", len(results))
	}

	results, err = s.Search(ctx, "WEATHER")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results for 'WEATHER', want 1", len(results))
	}
	if results[0].ID != "2" {
		t.Fatalf("expected id '2', got %q", results[0].ID)
	}
}

func TestLocalStore_SearchTag(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())

	s.Register(ctx, Listing{ID: "1", Name: "Tool", Tags: []string{"ai", "code-helper"}})

	results, err := s.Search(ctx, "helper")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results for 'helper', want 1", len(results))
	}
}

func TestLocalStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("concurrent-%d", i)
			l := Listing{ID: id, Name: id, Type: TypePlugin}
			if err := s.Register(ctx, l); err != nil {
				t.Errorf("Register: %v", err)
			}
		}(i)
	}
	wg.Wait()

	listings, err := s.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listings) != 20 {
		t.Fatalf("got %d listings, want 20", len(listings))
	}
}

func TestLocalStore_ConcurrentReadWrite(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())

	s.Register(ctx, Listing{ID: "shared", Name: "original", Tags: []string{"test"}})

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Get(ctx, "shared")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.List(ctx, Filter{Tags: []string{"test"}})
		}()
	}
	wg.Wait()
}

func TestInstaller_InstallListUninstall(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())
	inst := NewInstaller(s)

	installed, err := inst.Install(ctx, "https://example.com/plugin.tgz", InstallOptions{
		Name:    "My Plugin",
		Version: "0.1.0",
		Type:    TypePlugin,
		Tags:    []string{"utility"},
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if installed.Name != "My Plugin" {
		t.Fatalf("name = %q, want %q", installed.Name, "My Plugin")
	}
	if installed.SourceURL != "https://example.com/plugin.tgz" {
		t.Fatalf("source = %q", installed.SourceURL)
	}

	list, err := inst.ListInstalled(ctx, Filter{Type: ptr(TypePlugin)})
	if err != nil {
		t.Fatalf("ListInstalled: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d installed, want 1", len(list))
	}

	if err := inst.Uninstall(ctx, installed.ID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := s.Get(ctx, installed.ID); err == nil {
		t.Fatal("expected error after uninstall")
	}
}

func TestInstaller_ListInstalledEmpty(t *testing.T) {
	ctx := context.Background()
	s := NewLocalStore(t.TempDir())
	inst := NewInstaller(s)

	list, err := inst.ListInstalled(ctx, Filter{})
	if err != nil {
		t.Fatalf("ListInstalled: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("got %d, want 0", len(list))
	}
}
