// SPDX-License-Identifier: MIT
package scholar

import (
	"context"
	"testing"

	"github.com/Dxrk777/Dxrk/internal/log"
	"github.com/Dxrk777/Dxrk/internal/tools"
)

func newTestContext(s *Scholar) tools.Context {
	return tools.Context{
		Context: context.WithValue(context.Background(), ScholarContextKey{}, s),
		Logger:  log.NewNop(),
	}
}

func TestRegisterTools(t *testing.T) {
	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatalf("RegisterTools() error: %v", err)
	}
	for _, name := range []string{"scholar_search", "scholar_cite"} {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("RegisterTools() did not register %q", name)
		}
	}

	search, _ := reg.Get("scholar_search")
	if !search.IsEnabled() {
		t.Error("scholar_search should be enabled by default")
	}

	cite, _ := reg.Get("scholar_cite")
	out, err := cite.Execute(newTestContext(New()), map[string]any{"doi": "10.1000/abc"})
	if err != nil {
		t.Fatalf("scholar_cite with no providers should not error, got %v", err)
	}
	res := out.(map[string]any)
	if res["found"] != false {
		t.Errorf("found = %v, want false with no providers", res["found"])
	}

	_, err = cite.Execute(newTestContext(New()), map[string]any{"doi": "not-a-doi"})
	if err == nil {
		t.Error("scholar_cite should error on invalid DOI, got nil")
	}
}

func TestScholarSearchTool(t *testing.T) {
	s := New(&fakeProvider{
		name: "arxiv",
		search: func(ctx context.Context, q string, l int) ([]Paper, error) {
			return []Paper{{Title: "Paper A", DOI: "10.1000/a", Source: "arxiv", Year: 2021}}, nil
		},
	})
	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatalf("RegisterTools() error: %v", err)
	}
	tool, _ := reg.Get("scholar_search")

	out, err := tool.Execute(newTestContext(s), map[string]any{"query": "attention", "limit": 5})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	res := out.(map[string]any)
	if res["total"] != 1 {
		t.Errorf("total = %v, want 1", res["total"])
	}
	if res["enabled"] != true {
		t.Errorf("enabled = %v, want true", res["enabled"])
	}
}

func TestScholarCiteTool(t *testing.T) {
	fetched := &Paper{Title: "The Paper", Authors: []string{"Jane Doe"}, DOI: "10.1000/abc", Year: 2021}
	s := New(&fakeProvider{
		fetch: func(ctx context.Context, doi string) (*Paper, error) {
			return fetched, nil
		},
	})
	reg := tools.New()
	if err := RegisterTools(reg); err != nil {
		t.Fatalf("RegisterTools() error: %v", err)
	}
	tool, _ := reg.Get("scholar_cite")

	out, err := tool.Execute(newTestContext(s), map[string]any{"doi": "10.1000/abc"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	res := out.(map[string]any)
	if res["found"] != true {
		t.Errorf("found = %v, want true", res["found"])
	}
	if _, ok := res["bibtex"].(string); !ok {
		t.Errorf("bibtex missing from output %#v", res)
	}
}
