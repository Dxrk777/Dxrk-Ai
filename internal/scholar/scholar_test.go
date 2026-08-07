// SPDX-License-Identifier: MIT
package scholar

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	name   string
	search func(ctx context.Context, query string, limit int) ([]Paper, error)
	fetch  func(ctx context.Context, doi string) (*Paper, error)
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Search(ctx context.Context, query string, limit int) ([]Paper, error) {
	if f.search == nil {
		return nil, nil
	}
	return f.search(ctx, query, limit)
}

func (f *fakeProvider) FetchByDOI(ctx context.Context, doi string) (*Paper, error) {
	if f.fetch == nil {
		return nil, nil
	}
	return f.fetch(ctx, doi)
}

func TestNewAndSearch(t *testing.T) {
	s := New(
		&fakeProvider{name: "one", search: func(ctx context.Context, q string, l int) ([]Paper, error) {
			return []Paper{{Title: "a", Source: "one"}, {Title: "b", Source: "one"}}, nil
		}},
		&fakeProvider{name: "two", search: func(ctx context.Context, q string, l int) ([]Paper, error) {
			return []Paper{{Title: "c", Source: "two"}}, nil
		}},
	)

	got, err := s.Search(context.Background(), "query", 0)
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Search() got %d papers, want 3", len(got))
	}
	if got[0].Title != "a" || got[2].Title != "c" {
		t.Errorf("Search() order mismatch: %#v", got)
	}

	gotLimit, err := s.Search(context.Background(), "query", 2)
	if err != nil {
		t.Fatalf("Search() limited unexpected error: %v", err)
	}
	if len(gotLimit) != 2 {
		t.Errorf("Search() limited got %d papers, want 2", len(gotLimit))
	}

	empty := New()
	gotEmpty, err := empty.Search(context.Background(), "q", 0)
	if err != nil {
		t.Fatalf("Search() empty Scholar error: %v", err)
	}
	if len(gotEmpty) != 0 {
		t.Errorf("Search() empty Scholar got %d papers, want 0", len(gotEmpty))
	}
}

func TestSearchSkipsErrors(t *testing.T) {
	s := New(
		&fakeProvider{name: "bad", search: func(ctx context.Context, q string, l int) ([]Paper, error) {
			return nil, errors.New("boom")
		}},
		&fakeProvider{name: "good", search: func(ctx context.Context, q string, l int) ([]Paper, error) {
			return []Paper{{Title: "ok", Source: "good"}}, nil
		}},
	)
	got, err := s.Search(context.Background(), "q", 0)
	if err != nil {
		t.Fatalf("Search() should skip failed providers, got error: %v", err)
	}
	if len(got) != 1 || got[0].Title != "ok" {
		t.Errorf("Search() = %#v, want only the good provider paper", got)
	}
}

func TestFetchByDOI(t *testing.T) {
	want := &Paper{Title: "found", DOI: "10.1000/abc"}
	s := New(
		&fakeProvider{name: "first", fetch: func(ctx context.Context, doi string) (*Paper, error) {
			return nil, nil
		}},
		&fakeProvider{name: "second", fetch: func(ctx context.Context, doi string) (*Paper, error) {
			return want, nil
		}},
	)

	got, err := s.FetchByDOI(context.Background(), "10.1000/abc")
	if err != nil {
		t.Fatalf("FetchByDOI() error: %v", err)
	}
	if got != want {
		t.Errorf("FetchByDOI() = %#v, want %#v", got, want)
	}

	miss, err := New().FetchByDOI(context.Background(), "10.1000/xyz")
	if err != nil {
		t.Fatalf("FetchByDOI() empty error: %v", err)
	}
	if miss != nil {
		t.Errorf("FetchByDOI() empty = %#v, want nil", miss)
	}
}
